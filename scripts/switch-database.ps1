param(
    [Parameter(Mandatory = $true)]
    [ValidateSet("postgres", "neon")]
    [string]$Database
)

$ErrorActionPreference = "Stop"
$root = Resolve-Path (Join-Path $PSScriptRoot "..")
$composeFile = Join-Path $root "docker\compose.yml"
$envFile = Join-Path $root ".env"

if ($Database -eq "postgres") {
    $databaseUrl = "postgresql://postgres:postgres@postgres:5432/soloservis"
}
else {
    $backendEnv = Join-Path $root "backend\.env"
    if (-not (Test-Path $backendEnv)) {
        throw "No existe backend/.env. Configura DATABASE_URL con la cadena de Neon antes de continuar."
    }

    $databaseUrl = Select-String -Path $backendEnv -Pattern "^DATABASE_URL\s*=" |
        Select-Object -First 1 -ExpandProperty Line
    if (-not $databaseUrl) {
        throw "backend/.env no contiene DATABASE_URL."
    }

    $databaseUrl = ($databaseUrl -replace "^DATABASE_URL\s*=", "").Trim().Trim('"').Trim("'")
    if ($databaseUrl -match "^postgresql://.*localhost|^postgresql://.*127\.0\.0\.1") {
        throw "DATABASE_URL de backend/.env no parece una conexion de Neon."
    }
}

$envLines = @(
    "DATABASE_URL=$databaseUrl"
    "FRONTEND_URL=http://localhost:8443"
    "INTERNAL_API_TOKEN=dev-internal-token-solo-para-compose-local"
)
Set-Content -Path $envFile -Value $envLines -Encoding UTF8

Write-Host "Cambiando las APIs a $Database..."
docker compose --env-file $envFile -f $composeFile up -d --build --force-recreate
if ($LASTEXITCODE -ne 0) {
    throw "Docker Compose no pudo recrear las APIs."
}

Write-Host "Base activa: $Database"
Write-Host "Para cerrar sesiones antiguas del navegador, elimina 'soloservice.auth-session' y vuelve a iniciar sesion."
