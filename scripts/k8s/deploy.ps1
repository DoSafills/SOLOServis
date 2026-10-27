<#
.SYNOPSIS
  Despliega SOLOServis en Kubernetes por fases (0-9), con checkpoint por fase.

.DESCRIPTION
  Dos modos (ver k8s/README.md):
    local  Kubernetes de Docker Desktop. Una API por Deployment (microservicios),
           imagenes construidas en tu maquina (sin registry) y acceso con
           kubectl port-forward en http://localhost:8080.
    uct    Cluster de la universidad con TU kubeconfig. Las 14 APIs en un solo
           Deployment "backend" (la cuota no admite 14), imagenes publicadas en
           un registry y acceso por Ingress en soloservis-<usuario>.dev.censei.cl.

  Cada fase ejecuta, comprueba y muestra su checkpoint. Si un checkpoint falla, el
  script se detiene y muestra que lo bloquea (eventos, logs). Nunca imprime Secrets,
  tokens ni el contenido del kubeconfig.

  Requiere: kubectl, docker y git (en uct, ademas docker login al registry).

.EXAMPLE
  # Local (Docker Desktop con Kubernetes activado)
  .\scripts\k8s\deploy.ps1 -Modo local

.EXAMPLE
  # Cluster UCT
  .\scripts\k8s\deploy.ps1 -Modo uct -Kubeconfig "$HOME\.kube\estudiantes-<usuario>.kubeconfig" `
      -Registry docker.io/<usuario-docker>

.EXAMPLE
  # Solo inventario (no modifica nada)
  .\scripts\k8s\deploy.ps1 -Modo uct -ToPhase 0

.EXAMPLE
  # Re-desplegar desde las APIs reutilizando las imagenes ya construidas
  .\scripts\k8s\deploy.ps1 -Modo local -FromPhase 6 -SkipCleanup
#>
[CmdletBinding()]
param(
  [Parameter(Mandatory = $true)][ValidateSet('local', 'uct')][string]$Modo,
  # Por defecto: microservicios en local, consolidado en uct.
  [ValidateSet('', 'microservicios', 'consolidado')][string]$Topologia = '',
  [string]$Kubeconfig = '',
  [string]$Context = '',
  [string]$Namespace = '',
  # Solo uct: registry accesible por los nodos (ej. docker.io/miusuario).
  [string]$Registry = '',
  [string]$Tag = '',
  # Solo uct: autodetectados en la Fase 2 si no se indican.
  [string]$StorageClass = '',
  [string]$IngressClass = '',
  [string]$IngressHost = '',
  [ValidateSet('http', 'https')][string]$IngressScheme = 'http',
  [string]$ImagePullSecret = '',
  # Solo local: puerto de tu maquina para kubectl port-forward.
  [int]$LocalPort = 8080,
  [ValidateRange(0, 9)][int]$FromPhase = 0,
  [ValidateRange(0, 9)][int]$ToPhase = 9,
  [switch]$SkipCleanup
)

$ErrorActionPreference = 'Continue'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$OverlayDir = Join-Path $RepoRoot "k8s/overlays/$Modo"
$StateFile = Join-Path $OverlayDir 'state.json'
$Rendered = Join-Path $OverlayDir 'rendered.yaml'
$Utf8NoBom = New-Object System.Text.UTF8Encoding $false
$IsLocal = ($Modo -eq 'local')
if (-not $Topologia) { $Topologia = $(if ($IsLocal) { 'microservicios' } else { 'consolidado' }) }

# APIs que se despliegan segun la topologia (nombre del Deployment y puerto).
$MicroApis = [ordered]@{
  'product-api' = 8081; 'store-api' = 8082; 'service-api' = 8083; 'user-api' = 8084
  'pricing-api' = 8085; 'search-api' = 8086; 'comparison-api' = 8087; 'auth-api' = 8088
  'favorites-api' = 8089; 'cart-api' = 8090; 'review-api' = 8091; 'watchlist-api' = 8092
  'notification-api' = 8093; 'ingestion-api' = 8094
}
if ($Topologia -eq 'microservicios') { $Apis = $MicroApis } else { $Apis = [ordered]@{ 'backend' = 8080 } }

# --- kubeconfig, contexto y namespace
if (-not $Kubeconfig) {
  if ($env:KUBECONFIG) { $Kubeconfig = ($env:KUBECONFIG -split [IO.Path]::PathSeparator)[0] }
  elseif (Test-Path (Join-Path $HOME '.kube/config')) { $Kubeconfig = Join-Path $HOME '.kube/config' }
  elseif (-not $IsLocal) {
    $found = @(Get-ChildItem (Join-Path $HOME 'Downloads'), (Join-Path $HOME '.kube') -Filter 'estudiantes-*.kubeconfig' -ErrorAction SilentlyContinue)
    if ($found.Count -eq 1) { $Kubeconfig = $found[0].FullName }
  }
}
if (-not $Kubeconfig -or -not (Test-Path $Kubeconfig)) {
  if ($IsLocal) { Write-Host 'ERROR: no hay ~/.kube/config. Activa Kubernetes en Docker Desktop (Settings > Kubernetes).' -ForegroundColor Red }
  else { Write-Host 'ERROR: indica tu kubeconfig con -Kubeconfig (descargalo desde https://estudiantes.dev.censei.cl).' -ForegroundColor Red }
  exit 1
}
if ($IsLocal -and -not $Context) { $Context = 'docker-desktop' }
$KBase = @('--kubeconfig', $Kubeconfig)
if ($Context) { $KBase += @('--context', $Context) }
if (-not $Namespace) {
  if ($IsLocal) { $Namespace = 'soloservis' }
  else { $Namespace = ((& kubectl @KBase config view --minify -o 'jsonpath={..namespace}' 2>$null) -join '').Trim() }
}
if (-not $Namespace) {
  Write-Host 'ERROR: el kubeconfig no define namespace; usa -Namespace student-<usuario>.' -ForegroundColor Red
  exit 1
}
$KNoNs = $KBase
$KBase += @('-n', $Namespace)
# student-marcelo-vidal-22843887e32d -> marcelo-vidal (sin el sufijo hash)
$StudentUser = ($Namespace -replace '^student-', '') -replace '-[0-9a-f]{8,}$', ''
$script:KExit = 0

# ---------------------------------------------------------------- helpers

# kubectl con kubeconfig/contexto/namespace fijos. Devuelve lineas de texto.
# Funcion simple (sin [Parameter]) para que flags como -o lleguen en $args.
# OJO: PowerShell se come un -- sin comillas al llamar funciones; usar '--'.
function K {
  $out = & kubectl @KBase @args 2>&1
  $script:KExit = $LASTEXITCODE
  $out | ForEach-Object { "$_" }
}

function KJson {
  $raw = K @args -o json
  if ($script:KExit -ne 0) { return $null }
  try { return (($raw -join "`n") | ConvertFrom-Json) } catch { return $null }
}

function Show([string[]]$Lines, [int]$Tail = 0) {
  if (-not $Lines) { return }
  if ($Tail -gt 0) { $Lines = $Lines | Select-Object -Last $Tail }
  $Lines | ForEach-Object { Write-Host "    $_" }
}

function Step([string]$Text) { Write-Host "  - $Text" -ForegroundColor DarkGray }

function Checkpoint([int]$N, $Items) {
  Write-Host ''
  Write-Host "=== FASE $N ===" -ForegroundColor Cyan
  foreach ($k in $Items.Keys) {
    $v = "$($Items[$k])"
    $color = 'Gray'
    if ($v -match '^(OK|SI|Bound|Running|FUNCIONAL|ninguno)') { $color = 'Green' }
    elseif ($v -match '^(ERROR|NO|Pending|BLOQUEADO)') { $color = 'Red' }
    Write-Host ("{0}: {1}" -f $k, $v) -ForegroundColor $color
  }
  Write-Host ''
}

function Stop-Phase([int]$N, $Items, [string]$Reason) {
  $Items['Bloqueos'] = $Reason
  Checkpoint $N $Items
  Write-Host "Proceso detenido en la Fase $N." -ForegroundColor Red
  Write-Host "Corrige el bloqueo y vuelve a correr con: -Modo $Modo -FromPhase $N" -ForegroundColor Yellow
  exit 1
}

function Show-Diagnostics([string]$Kind, [string]$Name) {
  Write-Host "  Diagnostico $Kind/$Name (ultimas lineas de describe):" -ForegroundColor Yellow
  Show (K describe $Kind $Name) -Tail 20
}

function Get-PodsByName([string]$AppName) {
  $j = KJson get pods -l "app.kubernetes.io/name=$AppName"
  if ($j) { return @($j.items) } else { return @() }
}

function Get-PodState($Pod) {
  if (-not $Pod) { return 'ERROR (sin pod)' }
  $phase = $Pod.status.phase
  foreach ($cs in @($Pod.status.containerStatuses)) {
    if ($cs -and $cs.state.waiting) { return "$phase ($($cs.state.waiting.reason))" }
  }
  return $phase
}

function Test-PodReady($Pod) {
  if (-not $Pod) { return $false }
  foreach ($c in @($Pod.status.conditions)) {
    if ($c.type -eq 'Ready' -and $c.status -eq 'True') { return $true }
  }
  return $false
}

# SQL por stdin para no pelear con comillas en PowerShell 5.1.
function PgQuery([string]$Sql) {
  $out = $Sql | & kubectl @KBase exec -i postgres-0 -c postgres '--' sh -c 'psql -X -q -tA -v ON_ERROR_STOP=1 -U $POSTGRES_USER -d $POSTGRES_DB' 2>&1
  $script:KExit = $LASTEXITCODE
  (($out | ForEach-Object { "$_" }) -join "`n").Trim()
}

function Write-TextFile([string]$Path, [string[]]$Lines) {
  [System.IO.File]::WriteAllLines($Path, $Lines, $Utf8NoBom)
}

# Prueba HTTP desde tu maquina. curl.exe (incluido en Windows 10+) sigue
# redirecciones http->https y acepta el certificado de la plataforma (-k).
function Invoke-Probe([string]$Url, [string]$Expect) {
  if (Get-Command curl.exe -ErrorAction SilentlyContinue) {
    $tmp = [System.IO.Path]::GetTempFileName()
    $code = (& curl.exe -s -k -L -m 15 -o $tmp -w '%{http_code}' $Url 2>&1) -join ''
    $body = ''
    try { $body = [System.IO.File]::ReadAllText($tmp) } catch { }
    Remove-Item $tmp -ErrorAction SilentlyContinue
    if ($code -eq '200' -and ($body -match $Expect)) { return "OK (200 $Url)" }
    if ($code -eq '000') { return "ERROR (sin respuesta: DNS o red; $Url)" }
    return "ERROR ($code $Url)"
  }
  try {
    $r = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 15
    $body = $r.Content
    if ($body -is [byte[]]) { $body = [System.Text.Encoding]::UTF8.GetString($body) }
    if ($r.StatusCode -eq 200 -and ($body -match $Expect)) { return "OK (200 $Url)" }
    return "ERROR ($($r.StatusCode) $Url)"
  } catch {
    return "ERROR ($($_.Exception.Message) $Url)"
  }
}

# Rutas publicas que la app usa, a traves del nginx del frontend.
$PublicProbes = [ordered]@{
  'Frontend'            = @('/', '<div id="root"')
  'Catalogo (products)' = @('/products?limit=1', '\[')
  'Categorias'          = @('/categories', '"items"')
  'Tiendas'             = @('/stores', '\[')
  'Servicios'           = @('/services', '\[')
  'Busqueda'            = @('/search/products?q=a&limit=1', '\[')
}

function Test-PublicRoutes([string]$Base, $Cp) {
  $allOk = $true
  foreach ($k in $PublicProbes.Keys) {
    $p = $PublicProbes[$k]
    $Cp[$k] = Invoke-Probe "$Base$($p[0])" $p[1]
    if ("$($Cp[$k])" -notlike 'OK*') { $allOk = $false }
  }
  return $allOk
}

# kubectl port-forward temporal (modo local) para probar desde tu maquina.
function Start-PortForward {
  # Start-Process (PowerShell 5.1) no cita los argumentos: se citan aqui.
  $args2 = (@($KBase) + @('port-forward', 'svc/frontend', "$($LocalPort):80")) | ForEach-Object { '"' + $_ + '"' }
  $p = Start-Process kubectl -ArgumentList $args2 -PassThru -WindowStyle Hidden
  Start-Sleep -Seconds 3
  return $p
}

# ---------------------------------------------------------------- estado local

function Read-State {
  if (Test-Path $StateFile) {
    try { return (Get-Content $StateFile -Raw | ConvertFrom-Json) } catch { }
  }
  return $null
}

function Save-State {
  New-Item -ItemType Directory -Force $OverlayDir | Out-Null
  $s = [ordered]@{
    Topologia       = $Topologia
    Registry        = $Registry
    Tag             = $Tag
    StorageClass    = $StorageClass
    StorageResolved = $script:StorageResolved
    IngressClass    = $IngressClass
    IngressHost     = $IngressHost
    IngressScheme   = $IngressScheme
    ImagePullSecret = $ImagePullSecret
  }
  Write-TextFile $StateFile @(($s | ConvertTo-Json))
}

function New-ImageTag {
  $sha = (& git -C $RepoRoot rev-parse --short HEAD 2>$null)
  if (-not $sha) { $sha = 'local' }
  $dirty = (& git -C $RepoRoot status --porcelain 2>$null)
  if ($dirty) { return "$sha-$(Get-Date -Format 'yyyyMMddHHmm')" }
  return "$sha"
}

# Imagenes de este despliegue: frontend + (backend | una por API).
function Get-Images {
  $list = @(@{ Name = 'frontend'; File = 'docker/frontend/Dockerfile'; Ctx = '.' })
  foreach ($n in $Apis.Keys) {
    $file = $(if ($n -eq 'backend') { 'docker/all-apis/Dockerfile' } else { "docker/$n/Dockerfile" })
    $list += @{ Name = $n; File = $file; Ctx = 'backend' }
  }
  return $list
}

function Get-ImageRef([string]$Name) {
  if ($IsLocal) { return "soloservis/${Name}:$Tag" }
  return "$Registry/soloservis-${Name}:$Tag"
}

function Get-FrontendUrl {
  if ($IsLocal) { return "http://localhost:$LocalPort" }
  return "${IngressScheme}://$IngressHost"
}

# Genera k8s/overlays/<modo> (ignorado por git) a partir de base + componente.
function Write-Overlay {
  $migDir = Join-Path $OverlayDir 'migrations'
  New-Item -ItemType Directory -Force $migDir | Out-Null
  Get-ChildItem $migDir -Filter *.sql -ErrorAction SilentlyContinue | Remove-Item -Force
  $sqls = Get-ChildItem (Join-Path $RepoRoot 'backend/database/migrations') -Filter *.sql | Sort-Object Name
  foreach ($f in $sqls) { Copy-Item $f.FullName (Join-Path $migDir $f.Name) -Force }
  Copy-Item (Join-Path $RepoRoot 'backend/database/migration_order.txt') (Join-Path $migDir 'migration_order.txt') -Force

  $L = New-Object System.Collections.Generic.List[string]
  $L.Add('# GENERADO por scripts/k8s/deploy.ps1 - no editar ni commitear.')
  $L.Add('apiVersion: kustomize.config.k8s.io/v1beta1')
  $L.Add('kind: Kustomization')
  $L.Add("namespace: $Namespace")
  $L.Add('resources:')
  $L.Add('  - ../../base')
  $L.Add('components:')
  $L.Add("  - ../../components/$Topologia")
  $L.Add('configMapGenerator:')
  $L.Add('  - name: soloservis-migrations')
  $L.Add('    files:')
  foreach ($f in $sqls) { $L.Add("      - migrations/$($f.Name)") }
  $L.Add('      - migrations/migration_order.txt')
  $L.Add('    options:')
  $L.Add('      labels:')
  $L.Add('        app.kubernetes.io/part-of: soloservis')
  $L.Add('        app.kubernetes.io/component: migrations')
  $L.Add('  - name: soloservis-config')
  $L.Add('    behavior: merge')
  $L.Add('    literals:')
  $L.Add("      - FRONTEND_URL=$(Get-FrontendUrl)")
  $L.Add('images:')
  foreach ($i in Get-Images) {
    $ref = Get-ImageRef $i.Name
    $L.Add("  - name: soloservis/$($i.Name)")
    $L.Add("    newName: $($ref.Substring(0, $ref.LastIndexOf(':')))")
    $L.Add("    newTag: `"$Tag`"")
  }
  $L.Add('patches:')
  if ($IsLocal) {
    # Sin Ingress Controller en Docker Desktop: se entra con kubectl port-forward.
    $L.Add('  - patch: |-')
    $L.Add('      $patch: delete')
    $L.Add('      apiVersion: networking.k8s.io/v1')
    $L.Add('      kind: Ingress')
    $L.Add('      metadata:')
    $L.Add('        name: soloservis')
  }
  if ($StorageClass) {
    $L.Add('  - target: {kind: StatefulSet, name: postgres}')
    $L.Add('    patch: |-')
    $L.Add('      - op: add')
    $L.Add('        path: /spec/volumeClaimTemplates/0/spec/storageClassName')
    $L.Add("        value: $StorageClass")
  }
  if (-not $IsLocal -and $IngressClass) {
    $L.Add('  - target: {kind: Ingress, name: soloservis}')
    $L.Add('    patch: |-')
    $L.Add('      - op: add')
    $L.Add('        path: /spec/ingressClassName')
    $L.Add("        value: $IngressClass")
  }
  if (-not $IsLocal -and $IngressHost) {
    $L.Add('  - target: {kind: Ingress, name: soloservis}')
    $L.Add('    patch: |-')
    $L.Add('      - op: add')
    $L.Add('        path: /spec/rules/0/host')
    $L.Add("        value: $IngressHost")
  }
  if ($ImagePullSecret) {
    $L.Add('  - target: {kind: Deployment}')
    $L.Add('    patch: |-')
    $L.Add('      - op: add')
    $L.Add('        path: /spec/template/spec/imagePullSecrets')
    $L.Add("        value: [{name: $ImagePullSecret}]")
  }
  if ($L[$L.Count - 1] -eq 'patches:') { $L.RemoveAt($L.Count - 1) }
  Write-TextFile (Join-Path $OverlayDir 'kustomization.yaml') $L.ToArray()
  Save-State
}

function Build-Rendered {
  if (-not $IsLocal -and -not $Registry) { return 'falta -Registry' }
  Write-Overlay
  $out = & kubectl kustomize $OverlayDir 2>&1
  if ($LASTEXITCODE -ne 0) { return ($out | ForEach-Object { "$_" }) -join ' ' }
  Write-TextFile $Rendered ($out | ForEach-Object { "$_" })
  return ''
}

function Apply-Component([string]$Selector) {
  $out = K apply -f $Rendered -l $Selector
  Show $out
  return ($script:KExit -eq 0)
}

# ---------------------------------------------------------------- FASE 0

function Phase0 {
  $cp = [ordered]@{}
  $repoOk = (Test-Path (Join-Path $RepoRoot 'k8s/base/kustomization.yaml')) -and
            (Test-Path (Join-Path $RepoRoot "k8s/components/$Topologia")) -and
            (Test-Path (Join-Path $RepoRoot 'backend/cmd/all-apis')) -and
            (Test-Path (Join-Path $RepoRoot 'fronted/package.json'))
  $cp['Repositorio'] = $(if ($repoOk) { 'OK' } else { 'ERROR' })
  $cp['Modo'] = "$Modo (${Topologia}: $($Apis.Count) Deployment(s) de API)"

  $missing = @('kubectl', 'docker', 'git') | Where-Object { -not (Get-Command $_ -ErrorAction SilentlyContinue) }
  if ($missing -contains 'kubectl') { Stop-Phase 0 $cp 'kubectl no esta instalado' }

  if ($IsLocal) {
    $ctxs = @(& kubectl --kubeconfig $Kubeconfig config get-contexts -o name 2>$null)
    if ($ctxs -notcontains $Context) {
      Stop-Phase 0 $cp "no existe el contexto '$Context': activa Kubernetes en Docker Desktop (Settings > Kubernetes > Enable Kubernetes)"
    }
    $nodes = (& kubectl @KNoNs get nodes 2>&1) -join ' '
    if ($LASTEXITCODE -ne 0) { Stop-Phase 0 $cp "el cluster local no responde: $nodes" }
    $cp['Acceso Kubernetes'] = "OK (contexto $Context)"
  } else {
    Step 'kubectl auth can-i (puede abrir el navegador para el login OIDC)'
    $can = (K auth can-i list pods) -join ' '
    if ($script:KExit -ne 0 -or $can -notmatch 'yes') {
      $cp['Acceso Kubernetes'] = 'ERROR'
      Stop-Phase 0 $cp "sin acceso al namespace $Namespace : $can"
    }
    $cp['Acceso Kubernetes'] = 'OK'
  }

  Write-Host "  Recursos actuales en ${Namespace}:" -ForegroundColor DarkGray
  Show (K get 'all,pvc,ingress,configmap,job' -o wide)
  Write-Host '  Secrets (solo nombres):' -ForegroundColor DarkGray
  Show (K get secret -o name)
  if (-not $IsLocal) {
    Write-Host '  ResourceQuota / LimitRange:' -ForegroundColor DarkGray
    Show (K get 'resourcequota,limitrange')
  }
  Write-Host '  StorageClasses:' -ForegroundColor DarkGray
  Show (K get storageclass)

  $owned = @(K get 'all,pvc,ingress,configmap,secret,job' -l 'app.kubernetes.io/part-of=soloservis' -o name | Where-Object { $_ -match '/' })
  $cp['Recursos encontrados'] = "$($owned.Count) etiquetados soloservis"
  $cp['Bloqueos'] = $(if ($missing) { "no instalado: $($missing -join ', ')" } else { 'ninguno' })
  Checkpoint 0 $cp
}

# ---------------------------------------------------------------- FASE 1

function Phase1 {
  $cp = [ordered]@{}
  if ($IsLocal) {
    $null = K get namespace $Namespace -o name
    if ($script:KExit -ne 0) {
      Step "Creando el namespace $Namespace"
      Show (& kubectl @KNoNs create namespace $Namespace 2>&1)
    }
  }
  if ($SkipCleanup) {
    $cp['Limpieza'] = 'OMITIDA (-SkipCleanup)'
    Checkpoint 1 $cp
    return
  }
  # Deployments, Services, ConfigMaps, Jobs e Ingress de un despliegue anterior
  # (por ejemplo, de la otra topologia). El Secret de postgres y el PVC se conservan.
  Step 'Borrando recursos etiquetados app.kubernetes.io/part-of=soloservis (PVC y Secrets se conservan)'
  Show (K delete 'deployment,statefulset,service,configmap,job,ingress,pod' -l 'app.kubernetes.io/part-of=soloservis' --ignore-not-found --wait=true --timeout=180s)

  # PVC de PostgreSQL: solo se borra si NO esta Bound (nunca llego a tener datos).
  $pvc = KJson get pvc postgres-data-postgres-0
  if ($pvc) {
    if ($pvc.status.phase -ne 'Bound') {
      Step "PVC postgres-data-postgres-0 en $($pvc.status.phase): se elimina"
      Show (K delete pvc postgres-data-postgres-0 --wait=true --timeout=180s)
    } else {
      Step 'PVC postgres-data-postgres-0 esta Bound: se conserva (datos)'
    }
  }

  $left = @(K get 'deployment,statefulset,service,job,ingress' -l 'app.kubernetes.io/part-of=soloservis' -o name | Where-Object { $_ -match '/' })
  $keptPvc = @(K get pvc -o name | Where-Object { $_ -match 'postgres-data-postgres-0' })
  $cp['Limpieza'] = $(if ($left.Count -eq 0) { 'OK' } else { 'ERROR' })
  $cp['Conservado'] = $(if ($keptPvc) { 'PVC de PostgreSQL (Bound, con datos)' } else { 'ninguno' })
  if ($left.Count -gt 0) { Stop-Phase 1 $cp "quedaron recursos sin borrar: $($left -join ', ')" }
  $cp['Bloqueos'] = 'ninguno'
  Checkpoint 1 $cp
}

# ---------------------------------------------------------------- FASE 2

function Test-StorageClass([string]$Class) {
  $label = $(if ($Class) { $Class } else { '(clase por defecto)' })
  Step "Probando almacenamiento: $label"
  $scLine = $(if ($Class) { "  storageClassName: $Class" } else { '' })
  $manifest = @"
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: soloservis-storage-probe
  labels: {app.kubernetes.io/part-of: soloservis, app.kubernetes.io/component: storage-probe}
spec:
  accessModes: [ReadWriteOnce]
$scLine
  resources: {requests: {storage: 1Gi}}
---
apiVersion: v1
kind: Pod
metadata:
  name: soloservis-storage-probe
  labels: {app.kubernetes.io/part-of: soloservis, app.kubernetes.io/component: storage-probe}
spec:
  restartPolicy: Never
  automountServiceAccountToken: false
  securityContext: {runAsNonRoot: true, runAsUser: 999, runAsGroup: 999, fsGroup: 999, seccompProfile: {type: RuntimeDefault}}
  containers:
    - name: probe
      image: busybox:1.37
      command: [sh, -c, 'sleep 900']
      securityContext: {allowPrivilegeEscalation: false, capabilities: {drop: [ALL]}}
      resources: {requests: {cpu: 25m, memory: 64Mi}, limits: {cpu: 50m, memory: 64Mi}}
      volumeMounts: [{name: data, mountPath: /data}]
  volumes:
    - name: data
      persistentVolumeClaim: {claimName: soloservis-storage-probe}
"@
  New-Item -ItemType Directory -Force $OverlayDir | Out-Null
  $file = Join-Path $OverlayDir 'storage-probe.yaml'
  Write-TextFile $file @($manifest)
  $result = @{ Ok = $false; Persist = $false; Detail = '' }

  Show (K apply -f $file)
  $null = K wait --for=condition=Ready pod/soloservis-storage-probe --timeout=240s
  if ($script:KExit -ne 0) {
    $pvc = KJson get pvc soloservis-storage-probe
    $result.Detail = "PVC $($pvc.status.phase); el pod no quedo Ready"
    Show-Diagnostics pvc soloservis-storage-probe
    Show-Diagnostics pod soloservis-storage-probe
  } else {
    $pvc = KJson get pvc soloservis-storage-probe
    $result.Ok = $true
    $result.Detail = "PVC $($pvc.status.phase), sc=$($pvc.spec.storageClassName)"
    $w = K exec soloservis-storage-probe '--' sh -c 'echo soloservis-ok > /data/probe.txt && sync'
    if ($script:KExit -ne 0) {
      Show $w
      $result.Detail += '; escritura FALLO'
    } else {
      # Recrear el pod y leer el archivo: prueba de persistencia real del volumen.
      Step 'Recreando el pod (un volumen RWO puede tardar en moverse de nodo)'
      Show (K delete pod soloservis-storage-probe --wait=true --timeout=180s)
      Show (K apply -f $file)
      $null = K wait --for=condition=Ready pod/soloservis-storage-probe --timeout=420s
      if ($script:KExit -eq 0) {
        $read = (K exec soloservis-storage-probe '--' cat /data/probe.txt) -join ' '
        $result.Persist = ($read -match 'soloservis-ok')
      }
      if (-not $result.Persist) { $result.Detail += '; el archivo NO persistio' }
    }
  }
  Show (K delete -f $file --ignore-not-found --wait=true --timeout=180s)
  return $result
}

function Phase2 {
  $cp = [ordered]@{}
  $block = @()

  if ($IsLocal) {
    # Docker Desktop trae la StorageClass "hostpath" por defecto.
    $cp['Storage'] = 'OK (clase por defecto del cluster local)'
    $cp['Ingress'] = "NO SE USA (modo local: kubectl port-forward en localhost:$LocalPort)"
  } else {
    # --- Storage: probar clases hasta encontrar una que deje el PVC Bound y persista.
    $candidates = @()
    if ($StorageClass) { $candidates = @($StorageClass) }
    elseif ($script:StorageResolved) { $candidates = @($StorageClass) }
    else {
      $sc = KJson get storageclass
      if ($sc -and $sc.items) {
        $defs = @($sc.items | Where-Object { $_.metadata.annotations -and $_.metadata.annotations.'storageclass.kubernetes.io/is-default-class' -eq 'true' } | ForEach-Object { $_.metadata.name })
        $rest = @($sc.items | Where-Object { $defs -notcontains $_.metadata.name } | ForEach-Object { $_.metadata.name })
        # '' = sin storageClassName (usa la clase por defecto del cluster)
        if ($defs.Count -gt 0) { $candidates = @('') + $rest } else { $candidates = $rest }
      } else { $candidates = @('') }
    }
    # Si el PVC de PostgreSQL ya existe, su clase manda (no se puede cambiar).
    $pvc = KJson get pvc postgres-data-postgres-0
    if ($pvc -and $pvc.status.phase -eq 'Bound') {
      $script:StorageClass = $pvc.spec.storageClassName
      $script:StorageResolved = $true
      $cp['Storage'] = "OK (PVC existente, sc=$($pvc.spec.storageClassName), se conservan los datos)"
    } else {
      $chosen = $null
      foreach ($c in $candidates) {
        $r = Test-StorageClass $c
        if ($r.Ok -and $r.Persist) { $chosen = $c; break }
      }
      if ($null -ne $chosen) {
        $script:StorageClass = $chosen
        $script:StorageResolved = $true
        $cp['Storage'] = "OK ($(if ($chosen) { $chosen } else { 'clase por defecto' }); persistencia verificada)"
      } else {
        $cp['Storage'] = 'ERROR (ninguna StorageClass dejo el PVC Bound y persistente)'
        $block += 'almacenamiento persistente'
      }
    }

    # --- Ingress Controller
    if ($IngressClass) {
      $cp['Ingress'] = "OK (clase indicada: $IngressClass)"
    } else {
      $ic = KJson get ingressclass
      if ($ic -and $ic.items -and @($ic.items).Count -gt 0) {
        $def = @($ic.items | Where-Object { $_.metadata.annotations -and $_.metadata.annotations.'ingressclass.kubernetes.io/is-default-class' -eq 'true' })
        if ($def.Count -ge 1) { $script:IngressClass = $def[0].metadata.name }
        elseif (@($ic.items).Count -eq 1) { $script:IngressClass = @($ic.items)[0].metadata.name }
        if ($script:IngressClass) { $cp['Ingress'] = "OK ($($script:IngressClass))" }
        else { $cp['Ingress'] = 'ERROR (varias IngressClass sin default: usar -IngressClass)'; $block += 'IngressClass ambigua' }
      } else {
        $cp['Ingress'] = 'SIN VERIFICAR (no se puede listar IngressClass; se usa la del manifiesto: nginx)'
      }
    }
    if (-not $Registry) {
      $cp['Registry'] = 'ERROR (falta -Registry, ej. docker.io/miusuario)'
      $block += 'registry de imagenes'
    } else {
      $cp['Registry'] = "OK ($Registry, tag $Tag)"
    }
  }

  if (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    $cp['Docker'] = 'ERROR (docker no instalado)'
    $block += 'docker'
  }
  if ($block.Count -gt 0) {
    $cp['Kubernetes preparado'] = 'ERROR'
    Stop-Phase 2 $cp ($block -join '; ')
  }
  $err = Build-Rendered
  if ($err) { $cp['Kubernetes preparado'] = 'ERROR'; Stop-Phase 2 $cp "kustomize: $err" }
  $cp['Kubernetes preparado'] = "OK (overlay k8s/overlays/$Modo generado)"
  $cp['Bloqueos'] = 'ninguno'
  Checkpoint 2 $cp
}

# ---------------------------------------------------------------- FASE 3

function Phase3 {
  $cp = [ordered]@{}
  if (-not $IsLocal -and -not $Registry) { Stop-Phase 3 $cp 'falta -Registry' }
  $allOk = $true
  foreach ($i in Get-Images) {
    $ref = Get-ImageRef $i.Name
    Step "docker build $ref"
    # En uct los nodos son amd64; en local se usa la arquitectura de tu maquina.
    $platform = @()
    if (-not $IsLocal) { $platform = @('--platform', 'linux/amd64') }
    & docker build @platform -f (Join-Path $RepoRoot $i.File) -t $ref (Join-Path $RepoRoot $i.Ctx)
    $ok = ($LASTEXITCODE -eq 0)
    if ($ok -and -not $IsLocal) {
      Step "docker push $ref"
      & docker push $ref
      $ok = ($LASTEXITCODE -eq 0)
    }
    $cp["$($i.Name)"] = $(if ($ok) { "OK ($ref)" } else { 'ERROR' })
    if (-not $ok) { $allOk = $false; break }
  }
  $where = $(if ($IsLocal) { 'en Docker local (el cluster de Docker Desktop las ve directo)' } else { 'publicadas en el registry' })
  $cp['Imagenes disponibles para Kubernetes'] = $(if ($allOk) { "OK ($where)" } else { 'ERROR' })
  if (-not $allOk) {
    $hint = $(if ($IsLocal) { 'revisa el error de docker build' } else { 'revisa docker login y el nombre del registry' })
    Stop-Phase 3 $cp "build/push fallo ($hint)"
  }
  Save-State
  $cp['Bloqueos'] = 'ninguno'
  Checkpoint 3 $cp
}

# ---------------------------------------------------------------- FASE 4

function Phase4 {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase 4 $cp "kustomize: $err" }

  # Token de servicio (scraper y llamadas internas). Fuera de git y sin la
  # etiqueta part-of: sobrevive a la limpieza de la Fase 1.
  $null = K get secret soloservis-internal -o name
  if ($script:KExit -ne 0) {
    $bytes = New-Object byte[] 32
    [System.Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($bytes)
    $token = ([System.BitConverter]::ToString($bytes) -replace '-', '').ToLower()
    $null = K create secret generic soloservis-internal "--from-literal=INTERNAL_API_TOKEN=$token"
    $cp['Token interno'] = $(if ($script:KExit -eq 0) { 'OK (Secret soloservis-internal creado)' } else { 'ERROR' })
  } else {
    $cp['Token interno'] = 'OK (Secret soloservis-internal existente)'
  }

  Step 'Aplicando Secret, Services y StatefulSet de PostgreSQL'
  $ok = Apply-Component 'app.kubernetes.io/component=database'
  if (-not $ok) { Stop-Phase 4 $cp 'kubectl apply fallo' }

  Step 'Esperando rollout de postgres (max 5 min)'
  $null = K rollout status statefulset/postgres --timeout=300s
  $pvc = KJson get pvc postgres-data-postgres-0
  $pod = KJson get pod postgres-0
  $cp['PVC'] = $(if ($pvc) { "$($pvc.status.phase) (sc=$($pvc.spec.storageClassName), $($pvc.status.capacity.storage))" } else { 'ERROR (no existe)' })
  $cp['PostgreSQL Pod'] = Get-PodState $pod
  $ready = Test-PodReady $pod
  $cp['PostgreSQL Ready'] = $(if ($ready) { 'SI' } else { 'NO' })
  if (-not $ready) {
    Show-Diagnostics pvc postgres-data-postgres-0
    Show-Diagnostics pod postgres-0
    Show (K logs postgres-0 -c postgres --tail=30)
    Stop-Phase 4 $cp 'PostgreSQL no quedo Ready (no se continua a migraciones)'
  }
  Step "PostgreSQL $(PgQuery 'SHOW server_version;')"
  Checkpoint 4 $cp
}

# ---------------------------------------------------------------- FASE 5

function Phase5 {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase 5 $cp "kustomize: $err" }
  Step 'Recreando Job soloservis-migrate (los Jobs son inmutables)'
  Show (K delete job soloservis-migrate --ignore-not-found --wait=true --timeout=120s)
  $null = Apply-Component 'app.kubernetes.io/component=config'
  $ok = Apply-Component 'app.kubernetes.io/component=migrations'
  if (-not $ok) { $cp['Migration Job'] = 'ERROR'; Stop-Phase 5 $cp 'kubectl apply fallo' }

  Step 'Esperando el Job (max 5 min)'
  $state = 'timeout'
  $deadline = (Get-Date).AddSeconds(300)
  while ((Get-Date) -lt $deadline) {
    $j = KJson get job soloservis-migrate
    if ($j -and $j.status.succeeded -ge 1) { $state = 'complete'; break }
    if ($j -and $j.status.conditions) {
      if (@($j.status.conditions | Where-Object { $_.type -eq 'Failed' -and $_.status -eq 'True' }).Count -gt 0) { $state = 'failed'; break }
    }
    Start-Sleep -Seconds 5
  }
  Write-Host '  Logs del Job:' -ForegroundColor DarkGray
  Show (K logs job/soloservis-migrate --tail=40)
  $cp['Migration Job'] = $(if ($state -eq 'complete') { 'OK' } else { "ERROR ($state)" })
  if ($state -ne 'complete') {
    Show (K get pods -l job-name=soloservis-migrate -o wide)
    foreach ($p in @((KJson get pods -l job-name=soloservis-migrate).items)) {
      if ($p) { Show-Diagnostics pod $p.metadata.name }
    }
    Show (K get events --sort-by=.lastTimestamp) -Tail 15
    Stop-Phase 5 $cp 'el Job de migraciones no termino correctamente'
  }
  $expected = @(Get-Content (Join-Path $RepoRoot 'backend/database/migration_order.txt') | Where-Object { $_ -match '\.sql\s*$' }).Count
  $count = PgQuery 'SELECT count(*) FROM schema_migrations;'
  $cp['Migraciones aplicadas'] = $(if ("$count" -eq "$expected") { "OK ($count de $expected)" } else { "ERROR ($count de $expected)" })
  if ("$count" -ne "$expected") { $cp['Base de datos lista'] = 'NO'; Stop-Phase 5 $cp 'faltan migraciones' }
  $products = PgQuery 'SELECT count(*) FROM product;'
  $cp['Base de datos lista'] = "SI ($products productos)"
  Checkpoint 5 $cp
}

# ---------------------------------------------------------------- FASE 6

function Test-ApiHealth([string]$Name, [int]$Port) {
  $h = (K exec "deployment/$Name" '--' wget -qO- -T 5 "http://127.0.0.1:$Port/health") -join ' '
  return ($h -match '"database":"connected"')
}

function Phase6 {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase 6 $cp "kustomize: $err" }
  $null = Apply-Component 'app.kubernetes.io/component=config'
  $ok = Apply-Component 'app.kubernetes.io/component=api'
  if (-not $ok) { Stop-Phase 6 $cp 'kubectl apply fallo' }

  $failed = @()
  foreach ($name in $Apis.Keys) {
    $null = K rollout status "deployment/$name" --timeout=240s
    $pod = @(Get-PodsByName $name | Sort-Object { $_.metadata.creationTimestamp } -Descending)[0]
    $ready = Test-PodReady $pod
    if ($ready -and (Test-ApiHealth $name $Apis[$name])) {
      $cp[$name] = "OK (Ready, PostgreSQL conectado, :$($Apis[$name]))"
    } else {
      $cp[$name] = "ERROR ($(Get-PodState $pod))"
      $failed += $name
      if ($pod) { Show-Diagnostics pod $pod.metadata.name }
      Show (K logs "deployment/$name" --tail=20)
    }
  }
  if ($failed.Count -gt 0) {
    if (@($cp.Values | Where-Object { "$_" -match 'ImagePull|ErrImage' }).Count -gt 0) {
      if ($IsLocal) { Stop-Phase 6 $cp 'el cluster no ve las imagenes locales: en Docker Desktop usa Kubernetes con el aprovisionador kubeadm (ver k8s/README.md)' }
      Stop-Phase 6 $cp 'el cluster no puede descargar la imagen (registry privado: crear Secret docker-registry y usar -ImagePullSecret)'
    }
    Stop-Phase 6 $cp "no quedaron Ready: $($failed -join ', ')"
  }
  $cp['Bloqueos'] = 'ninguno'
  Checkpoint 6 $cp
}

# ---------------------------------------------------------------- FASE 7

function Phase7 {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase 7 $cp "kustomize: $err" }
  $ok = Apply-Component 'app.kubernetes.io/name=frontend'
  if (-not $ok) { Stop-Phase 7 $cp 'kubectl apply fallo' }
  $null = K rollout status deployment/frontend --timeout=240s
  $pod = @(Get-PodsByName 'frontend' | Sort-Object { $_.metadata.creationTimestamp } -Descending)[0]
  $cp['Pod'] = Get-PodState $pod
  if (-not (Test-PodReady $pod)) {
    if ($pod) { Show-Diagnostics pod $pod.metadata.name }
    Show (K logs deployment/frontend --tail=30)
    Stop-Phase 7 $cp 'frontend no quedo Ready'
  }
  # nginx -> cada API por el DNS de su Service (lo mismo que hara el navegador).
  $h = (K exec deployment/frontend '--' wget -qO- -T 5 'http://127.0.0.1:8080/products?limit=1') -join ' '
  $cp['nginx -> APIs'] = $(if ($h -match '^\s*\[') { 'OK (/products responde a traves del frontend)' } else { "ERROR ($h)" })
  Checkpoint 7 $cp
}

# ---------------------------------------------------------------- FASE 8

function Phase8 {
  $cp = [ordered]@{}
  if ($IsLocal) {
    Step "Abriendo kubectl port-forward temporal en localhost:$LocalPort"
    $pf = Start-PortForward
    $ok = Test-PublicRoutes "http://localhost:$LocalPort" $cp
    if ($pf -and -not $pf.HasExited) { Stop-Process -Id $pf.Id -Force -ErrorAction SilentlyContinue }
    Checkpoint 8 $cp
    if (-not $ok) { Stop-Phase 8 $cp 'alguna ruta publica no respondio (revisa los logs del frontend y de la API)' }
    return
  }
  $err = Build-Rendered
  if ($err) { Stop-Phase 8 $cp "kustomize: $err" }
  $ok = Apply-Component 'app.kubernetes.io/component=ingress'
  if (-not $ok) { $cp['Ingress'] = 'ERROR'; Stop-Phase 8 $cp 'kubectl apply fallo' }
  Step 'Esperando direccion del Ingress (max 2 min)'
  $addr = ''
  $deadline = (Get-Date).AddSeconds(120)
  while ((Get-Date) -lt $deadline) {
    $ing = KJson get ingress soloservis
    if ($ing -and $ing.status.loadBalancer.ingress) {
      $lb = @($ing.status.loadBalancer.ingress)[0]
      $addr = $(if ($lb.hostname) { $lb.hostname } else { $lb.ip }); break
    }
    Start-Sleep -Seconds 5
  }
  Show (K get ingress soloservis)
  $cp['Ingress'] = $(if ($addr) { "OK (host=$IngressHost, address=$addr)" } else { 'ERROR (sin address)' })
  $ok = Test-PublicRoutes (Get-FrontendUrl) $cp
  Checkpoint 8 $cp
  if (-not $ok) {
    Write-Host '  Nota: el DNS de *.dev.censei.cl puede tardar unos minutos la primera vez; si sigue, prueba desde la red de la universidad.' -ForegroundColor Yellow
  }
}

# ---------------------------------------------------------------- FASE 9

function Phase9 {
  $cp = [ordered]@{}
  foreach ($what in 'pods', 'pvc', 'deployments', 'statefulsets', 'services', 'ingress') {
    Write-Host "  kubectl get $what" -ForegroundColor DarkGray
    Show (K get $what -l 'app.kubernetes.io/part-of=soloservis')
  }

  # Recreacion de un Pod administrado por un Deployment.
  $probe = @($Apis.Keys)[0]
  Step "Borrando el pod de ${probe}: Kubernetes debe recrearlo"
  $old = @(Get-PodsByName $probe)[0]
  if ($old) { Show (K delete pod $old.metadata.name --wait=true --timeout=120s) }
  $null = K rollout status "deployment/$probe" --timeout=180s
  $new = @(Get-PodsByName $probe | Where-Object { Test-PodReady $_ })[0]
  $recreated = [bool]($new -and $old -and $new.metadata.name -ne $old.metadata.name)

  # Persistencia: reiniciar postgres-0 sin tocar el PVC y comparar datos.
  Step 'Reiniciando postgres-0 (el PVC NO se toca) para verificar persistencia'
  $before = PgQuery 'SELECT count(*) FROM schema_migrations;'
  Show (K delete pod postgres-0 --wait=true --timeout=120s)
  $null = K rollout status statefulset/postgres --timeout=300s
  $null = K wait --for=condition=Ready pod/postgres-0 --timeout=180s
  $after = PgQuery 'SELECT count(*) FROM schema_migrations;'
  $persist = [bool]($before -and "$before" -eq "$after")
  # Tras reiniciar la BD, las APIs deben volver a conectarse solas.
  Start-Sleep -Seconds 10
  $apisOk = $true
  foreach ($name in $Apis.Keys) { if (-not (Test-ApiHealth $name $Apis[$name])) { $apisOk = $false; $cp[$name] = 'ERROR (no reconecto a PostgreSQL)' } }

  $cp['Kubernetes'] = $(if ($recreated) { "FUNCIONAL (recreo el pod de $probe)" } else { 'ERROR (no se verifico la recreacion)' })
  $cp['Persistencia'] = $(if ($persist) { "FUNCIONAL (mismos datos tras reiniciar: $after migraciones)" } else { "ERROR (antes=$before despues=$after)" })
  $cp['APIs'] = $(if ($apisOk) { "FUNCIONAL ($($Apis.Count) Deployment(s), reconectan a PostgreSQL)" } else { 'ERROR' })
  Checkpoint 9 $cp

  Write-Host 'Para abrir la app:' -ForegroundColor Green
  if ($IsLocal) {
    Write-Host "  kubectl --context $Context -n $Namespace port-forward svc/frontend $($LocalPort):80" -ForegroundColor Green
    Write-Host "  y abre http://localhost:$LocalPort (deja esa terminal abierta)" -ForegroundColor Green
  } else {
    Write-Host "  $(Get-FrontendUrl)" -ForegroundColor Green
  }
}

# ---------------------------------------------------------------- main

$prev = Read-State
$script:StorageResolved = $false
if ($prev) {
  if (-not $Registry) { $Registry = $prev.Registry }
  if (-not $Tag -and $FromPhase -gt 3 -and $prev.Tag) { $Tag = $prev.Tag }
  if (-not $StorageClass -and $FromPhase -gt 2 -and $prev.StorageResolved) { $StorageClass = $prev.StorageClass; $script:StorageResolved = $true }
  if (-not $IngressClass -and $FromPhase -gt 2) { $IngressClass = $prev.IngressClass }
  if (-not $IngressHost) { $IngressHost = $prev.IngressHost }
  if (-not $ImagePullSecret) { $ImagePullSecret = $prev.ImagePullSecret }
}
# Dominio publico segun el manual: <proyecto>-<usuario>.dev.censei.cl
if (-not $IsLocal -and -not $IngressHost) { $IngressHost = "soloservis-$StudentUser.dev.censei.cl" }
if ($Registry -and $Registry -notmatch '^[a-z0-9.\-:]+(/[a-z0-9._\-]+)+$') {
  Write-Host "ERROR: -Registry '$Registry' no es valido. Usa tu usuario real en minusculas, ej: docker.io/miusuario" -ForegroundColor Red
  exit 1
}
if (-not $Tag) {
  if ($FromPhase -gt 3) { Write-Host 'AVISO: no hay tag guardado de la Fase 3; se genera uno nuevo (las imagenes deben existir con ese tag). Usa -Tag si ya construiste otro.' -ForegroundColor Yellow }
  $Tag = New-ImageTag
}

Write-Host "SOLOServis -> Kubernetes | modo=$Modo topologia=$Topologia contexto=$(if ($Context) { $Context } else { '(el del kubeconfig)' }) namespace=$Namespace fases=$FromPhase..$ToPhase" -ForegroundColor Cyan
Write-Host "Kubeconfig: $(Split-Path $Kubeconfig -Leaf) | tag=$Tag$(if (-not $IsLocal) { " registry=$(if ($Registry) { $Registry } else { '(sin definir)' })" })" -ForegroundColor Cyan

$phases = @{
  0 = { Phase0 }; 1 = { Phase1 }; 2 = { Phase2 }; 3 = { Phase3 }; 4 = { Phase4 }
  5 = { Phase5 }; 6 = { Phase6 }; 7 = { Phase7 }; 8 = { Phase8 }; 9 = { Phase9 }
}
for ($n = $FromPhase; $n -le $ToPhase; $n++) {
  Write-Host ''
  Write-Host "---------------- FASE $n ----------------" -ForegroundColor White
  & $phases[$n]
}
Write-Host 'Listo.' -ForegroundColor Green
