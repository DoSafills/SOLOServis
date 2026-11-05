<#
.SYNOPSIS
  Starts the local test stack against an isolated, migrated database.
.DESCRIPTION
  This helper and its Compose override do not alter application code or Sensei/UCT
  settings. The original PostgreSQL database and Docker volume are preserved.
.EXAMPLE
  .\scripts\local-test\start.ps1
.EXAMPLE
  .\scripts\local-test\start.ps1 -RestoreCompose
#>
[CmdletBinding()]
param(
  [string]$Database = 'soloservis_localtest',
  [switch]$RestoreCompose
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$ComposeFile = Join-Path $RepoRoot 'docker/compose.yml'
$OverrideFile = Join-Path $PSScriptRoot 'compose.override.yml'

if ($Database -notmatch '^[a-z][a-z0-9_]{0,62}$') {
  throw 'El nombre de la base debe usar letras minúsculas, números y guion bajo.'
}

function Invoke-Docker {
  param([string[]]$Arguments)

  & docker @Arguments
  if ($LASTEXITCODE -ne 0) {
    throw "Falló docker $($Arguments -join ' ') (código $LASTEXITCODE)."
  }
}

function Get-ComposeArgs {
  if ($RestoreCompose) {
    return @('compose', '-f', $ComposeFile)
  }
  return @('compose', '-f', $ComposeFile, '-f', $OverrideFile)
}

$ComposeArgs = Get-ComposeArgs

if ($RestoreCompose) {
  Write-Host 'Restaurando la configuración Docker Compose original (sin borrar volúmenes ni la base de pruebas)...'
  Invoke-Docker ($ComposeArgs + @('up', '-d', '--build'))
  exit 0
}

Write-Host "Preparando la base de datos de pruebas '$Database'..."
Invoke-Docker ($ComposeArgs + @('up', '-d', 'postgres'))

$PostgresContainer = (& docker @ComposeArgs ps -q postgres).Trim()
if ($LASTEXITCODE -ne 0 -or -not $PostgresContainer) {
  throw 'No se encontró el contenedor PostgreSQL de Compose.'
}

$ready = $false
for ($attempt = 0; $attempt -lt 60; $attempt++) {
  $health = (& docker inspect --format '{{.State.Health.Status}}' $PostgresContainer).Trim()
  if ($LASTEXITCODE -eq 0 -and $health -eq 'healthy') {
    $ready = $true
    break
  }
  Start-Sleep -Seconds 2
}
if (-not $ready) {
  throw 'PostgreSQL no quedó saludable dentro del límite de espera.'
}

$exists = (& docker @ComposeArgs exec -T postgres psql -X -qAt -U postgres -d postgres -c "SELECT 1 FROM pg_database WHERE datname = '$Database';").Trim()
if ($LASTEXITCODE -ne 0) {
  throw 'No se pudo consultar el catálogo de bases de PostgreSQL.'
}
if ($exists -ne '1') {
  Invoke-Docker ($ComposeArgs + @('exec', '-T', 'postgres', 'createdb', '-U', 'postgres', $Database))
}

Invoke-Docker ($ComposeArgs + @(
  'exec', '-T', '-e', "PGDATABASE=$Database", 'postgres',
  'bash', '/docker-entrypoint-initdb.d/001_migrations.sh'
))

Write-Host 'Levantando y reconstruyendo las APIs contra la base aislada...'
Invoke-Docker ($ComposeArgs + @('up', '-d', '--build'))
Write-Host "Base de pruebas activa: $Database"
Write-Host 'Para volver al Compose original, ejecuta: .\scripts\local-test\start.ps1 -RestoreCompose'