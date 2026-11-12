<#
.SYNOPSIS
  Muestra el estado del despliegue y comprueba las 14 APIs de SOLOServis.

.DESCRIPTION
  El modo UCT ejecuta las 14 APIs dentro del Deployment consolidado "backend".
  Este script comprueba el proceso, PostgreSQL y una ruta representativa de cada
  API desde dentro del pod. Un HTTP 401/403 significa que la API responde y que
  la ruta requiere autenticación; no se considera un fallo de disponibilidad.

.EXAMPLE
  .\scripts\k8s\health.ps1 -Modo uct -Kubeconfig .\estudiantes-marcelo.vidal.kubeconfig
#>
[CmdletBinding()]
param(
  [ValidateSet('local', 'uct')][string]$Modo = 'uct',
  [string]$Kubeconfig = '',
  [string]$Context = '',
  [string]$Namespace = ''
)

$ErrorActionPreference = 'Stop'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$IsLocal = ($Modo -eq 'local')

if (-not $Kubeconfig) {
  if ($env:KUBECONFIG) {
    $Kubeconfig = ($env:KUBECONFIG -split [IO.Path]::PathSeparator)[0]
  } elseif (-not $IsLocal) {
    $found = @(Get-ChildItem $RepoRoot -Filter 'estudiantes-*.kubeconfig' -File -ErrorAction SilentlyContinue)
    if ($found.Count -eq 1) { $Kubeconfig = $found[0].FullName }
  } elseif (Test-Path (Join-Path $HOME '.kube/config')) {
    $Kubeconfig = Join-Path $HOME '.kube/config'
  }
}

if (-not $Kubeconfig -or -not (Test-Path -LiteralPath $Kubeconfig)) {
  throw 'No se encontró el kubeconfig. Usa -Kubeconfig <ruta>.'
}

$KBase = @('--kubeconfig', $Kubeconfig)
if ($Context) { $KBase += @('--context', $Context) }

if (-not $Namespace) {
  if ($IsLocal) {
    $Namespace = 'soloservis'
  } else {
    $Namespace = ((& kubectl @KBase config view --minify -o 'jsonpath={..namespace}' 2>$null) -join '').Trim()
  }
}
if (-not $Namespace) { throw 'El kubeconfig no define namespace. Usa -Namespace <namespace>.' }
$KBase += @('-n', $Namespace)

function Invoke-Kubectl {
  param([string[]]$CommandArgs)
  $previousErrorAction = $ErrorActionPreference
  $ErrorActionPreference = 'Continue'
  $out = & kubectl @KBase @CommandArgs 2>&1
  $ErrorActionPreference = $previousErrorAction
  [PSCustomObject]@{ ExitCode = $LASTEXITCODE; Output = @($out | ForEach-Object { "$_" }) }
}

function Show-Result([string]$Name, [string]$State, [string]$Detail = '') {
  $color = 'Red'
  if ($State -match '^(OK|RESPONDE|Running|Ready|Complete|Bound)') { $color = 'Green' }
  $suffix = ''
  if ($Detail) { $suffix = " - $Detail" }
  Write-Host ("{0,-16} {1}{2}" -f $Name, $State, $suffix) -ForegroundColor $color
}

Write-Host "SOLOServis health | modo=$Modo | namespace=$Namespace" -ForegroundColor Cyan
Write-Host ''

$deploy = Invoke-Kubectl @('get', 'deployments', '-o', 'json')
if ($deploy.ExitCode -ne 0) { throw ($deploy.Output -join [Environment]::NewLine) }
$deployJson = ($deploy.Output -join [Environment]::NewLine) | ConvertFrom-Json
foreach ($item in @($deployJson.items)) {
  $ready = [int]$item.status.readyReplicas
  $desired = [int]$item.spec.replicas
  $state = 'ERROR'
  if ($ready -eq $desired -and $desired -gt 0) { $state = 'Ready' }
  Show-Result $item.metadata.name $state "$ready/$desired replicas"
}

$pods = Invoke-Kubectl @('get', 'pods', '-o', 'json')
if ($pods.ExitCode -ne 0) { throw ($pods.Output -join [Environment]::NewLine) }
$podsJson = ($pods.Output -join [Environment]::NewLine) | ConvertFrom-Json
foreach ($item in @($podsJson.items)) {
  $ready = @($item.status.containerStatuses | Where-Object { $_.ready }).Count
  $total = @($item.status.containerStatuses).Count
  $state = 'ERROR'
  if ($item.status.phase -eq 'Running' -and $ready -eq $total) { $state = 'Running' }
  elseif ($item.status.phase -eq 'Succeeded') { $state = 'Complete' }
  Show-Result $item.metadata.name $state "$ready/$total containers"
}

Write-Host ''
Write-Host 'Health global del backend:' -ForegroundColor Cyan
$global = Invoke-Kubectl @('exec', 'deployment/backend', '--', 'sh', '-c', 'wget -qO- --timeout=10 http://127.0.0.1:8080/health')
if ($global.ExitCode -eq 0) { Show-Result 'backend /health' 'OK' ($global.Output -join '') } else { Show-Result 'backend /health' 'ERROR' ($global.Output -join ' ') }

$apis = [ordered]@{
  'product'      = '/products?limit=1'
  'store'        = '/stores'
  'service'      = '/services'
  'user'         = '/users'
  'pricing'      = '/offers/product/00000000-0000-0000-0000-000000000000/best'
  'search'       = '/search/products?q=a&limit=1'
  'comparison'   = '/comparisons/services/1/items'
  'ingestion'    = '/ingestion/runs'
  'auth'         = '/auth/validate'
  'cart'         = '/cart/items'
  'favorites'    = '/favorites'
  'notification' = '/notifications'
  'review'       = '/reviews'
  'watchlist'    = '/watchlist'
}

Write-Host ''
Write-Host "Health de las $($apis.Count) APIs lógicas:" -ForegroundColor Cyan
$failed = 0
foreach ($api in $apis.Keys) {
  $url = "http://127.0.0.1:8080$($apis[$api])"
  $probe = "wget -S -O /dev/null --timeout=10 $url 2>&1 | grep HTTP | tail -1"
  $result = Invoke-Kubectl @('exec', 'deployment/backend', '--', 'sh', '-c', $probe)
  $response = (($result.Output -join ' ').Trim())
  $code = '000'
  if ($response -match 'HTTP/[0-9.]+\s+([0-9]{3})') { $code = $Matches[1] }
  if ($code -match '^[1-5][0-9][0-9]$') {
    Show-Result $api 'RESPONDE' "HTTP $code ($($apis[$api]))"
  } else {
    Show-Result $api 'ERROR' "sin respuesta ($($apis[$api]))"
    $failed++
  }
}

Write-Host ''
if ($failed -eq 0) {
  Write-Host 'Resultado: las 14 APIs responden desde el backend consolidado.' -ForegroundColor Green
} else {
  Write-Host "Resultado: $failed API(s) no respondieron." -ForegroundColor Red
  exit 1
}
