# Cambia las 3 APIs (comercial, usuarios, ingestion) entre PostgreSQL local y
# Neon, y recrea solo esos contenedores. La URL no se imprime.
#
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\switch-database.ps1 neon
#   powershell -NoProfile -ExecutionPolicy Bypass -File .\scripts\switch-database.ps1 local
#
# Neon toma DATABASE_URL de backend/.env; local usa el servicio "postgres" de
# docker/compose.yml. No cambies de base con "docker compose up" a mano: sin
# NEON_DATABASE_URL, Compose cae al Postgres local y deja los dominios partidos.

param(
    [Parameter(Position = 0, Mandatory = $true)]
    [ValidateSet("neon", "local")]
    [string]$Destino
)

$ErrorActionPreference = "Stop"

$root = Split-Path -Parent $PSScriptRoot
$backendEnv = Join-Path $root "backend/.env"
$localUrl = "postgres://postgres:postgres@postgres:5432/soloservis"
$containers = @("soloservis_usuarios", "soloservis_comercial", "soloservis_ingestion")

# Get-EnvValue lee una clave KEY=valor de un archivo .env sin expandir variables
# ni imprimir el contenido. Devuelve el valor o $null si la clave no existe.
function Get-EnvValue {
    param([string]$Path, [string]$Key)
    if (-not (Test-Path -LiteralPath $Path)) { return $null }
    foreach ($line in Get-Content -LiteralPath $Path) {
        if ($line -match ("^\s*" + [regex]::Escape($Key) + "\s*=\s*(.*)$")) {
            $value = $Matches[1].Trim()
            if ($value.Length -ge 2 -and $value.StartsWith('"') -and $value.EndsWith('"')) {
                $value = $value.Substring(1, $value.Length - 2)
            }
            return $value
        }
    }
    return $null
}

# Show-DatabaseHost muestra host y base que usa un contenedor, con las
# credenciales ocultas. Recibe el nombre del contenedor y no devuelve nada.
function Show-DatabaseHost {
    param([string]$Container)
    $redacted = docker exec $Container sh -c 'echo "$DATABASE_URL" | sed -E "s#://[^@/]+@#://****@#"'
    if ($LASTEXITCODE -eq 0) {
        Write-Host ("{0}: {1}" -f $Container, $redacted.Trim())
    } else {
        Write-Warning "No se pudo leer DATABASE_URL de $Container (esta corriendo?)."
    }
}

if ($Destino -eq "neon") {
    $url = Get-EnvValue -Path $backendEnv -Key "DATABASE_URL"
    if (-not $url) {
        throw "backend/.env no define DATABASE_URL. Agrega la URL de Neon antes de cambiar."
    }
} else {
    $url = $localUrl
}

if ($url -notmatch "^(postgres|postgresql)://") {
    throw "DATABASE_URL no parece una URL de PostgreSQL."
}

# Compose interpola NEON_DATABASE_URL en DATABASE_URL; solo existe en esta sesion.
$env:NEON_DATABASE_URL = $url
Push-Location $root
try {
    docker compose -f docker/compose.yml up -d --no-build comercial usuarios ingestion
    if ($LASTEXITCODE -ne 0) {
        throw "docker compose fallo con codigo $LASTEXITCODE."
    }
} finally {
    Pop-Location
}

Write-Host "Base activa: $Destino"
foreach ($container in $containers) {
    Show-DatabaseHost -Container $container
}
