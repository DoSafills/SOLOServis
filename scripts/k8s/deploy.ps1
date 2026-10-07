<#
.SYNOPSIS
  Despliega el MVP de SOLOServis en Kubernetes por fases (0-10), con checkpoint por fase.

.DESCRIPTION
  Arquitectura: Browser -> Ingress -> {frontend, product-api, store-api} -> PostgreSQL.
  Cada fase ejecuta, comprueba y muestra su checkpoint. Si un checkpoint falla, el
  script se detiene y muestra que lo bloquea (eventos, logs). Nunca imprime Secrets,
  tokens ni el contenido del kubeconfig.

  Requiere: kubectl, docker (con login al registry) y git.

.EXAMPLE
  # Despliegue completo
  .\scripts\k8s\deploy.ps1 -Kubeconfig "$HOME\Downloads\estudiantes-<usuario>.kubeconfig" `
      -Registry docker.io/<usuario-docker>
  # Host por defecto: soloservis-<usuario>.dev.censei.cl

.EXAMPLE
  # Solo inventario (no modifica nada)
  .\scripts\k8s\deploy.ps1 -ToPhase 0

.EXAMPLE
  # Re-desplegar desde las APIs reutilizando imagenes/tag ya publicados
  .\scripts\k8s\deploy.ps1 -FromPhase 6 -SkipCleanup
#>
[CmdletBinding()]
param(
  # Vacio = $env:KUBECONFIG, ~/.kube/config o el unico Downloads/estudiantes-*.kubeconfig
  [string]$Kubeconfig = '',
  # Vacio = contexto y namespace que trae el kubeconfig (student-<usuario>)
  [string]$Context = '',
  [string]$Namespace = '',
  # Prefijo del registry accesible por los nodos, ej: docker.io/usuario o ghcr.io/usuario
  [string]$Registry = '',
  [string]$Tag = '',
  # Vacio = se detecta y valida en Fase 2
  [string]$StorageClass = '',
  [string]$IngressClass = '',
  [string]$IngressHost = '',
  [ValidateSet('http', 'https')][string]$IngressScheme = 'http',
  # Secret docker-registry ya creado en el namespace (registry privado)
  [string]$ImagePullSecret = '',
  [ValidateRange(0, 10)][int]$FromPhase = 0,
  [ValidateRange(0, 10)][int]$ToPhase = 10,
  [switch]$SkipCleanup
)

$ErrorActionPreference = 'Continue'
$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot '../..')).Path
$OverlayDir = Join-Path $RepoRoot 'k8s/overlays/local'
$StateFile = Join-Path $OverlayDir 'state.json'
$Rendered = Join-Path $OverlayDir 'rendered.yaml'
$Utf8NoBom = New-Object System.Text.UTF8Encoding $false

# --- kubeconfig, contexto y namespace (cada estudiante usa el SUYO)
if (-not $Kubeconfig) {
  if ($env:KUBECONFIG) { $Kubeconfig = $env:KUBECONFIG }
  elseif (Test-Path (Join-Path $HOME '.kube/config')) { $Kubeconfig = Join-Path $HOME '.kube/config' }
  else {
    $found = @(Get-ChildItem (Join-Path $HOME 'Downloads') -Filter 'estudiantes-*.kubeconfig' -ErrorAction SilentlyContinue)
    if ($found.Count -eq 1) { $Kubeconfig = $found[0].FullName }
  }
}
if (-not $Kubeconfig -or -not (Test-Path $Kubeconfig)) {
  Write-Host 'ERROR: indica tu kubeconfig con -Kubeconfig (descargalo desde https://estudiantes.dev.censei.cl).' -ForegroundColor Red
  exit 1
}
$KBase = @('--kubeconfig', $Kubeconfig)
if ($Context) { $KBase += @('--context', $Context) }
if (-not $Namespace) {
  $Namespace = ((& kubectl @KBase config view --minify -o 'jsonpath={..namespace}' 2>$null) -join '').Trim()
}
if (-not $Namespace) {
  Write-Host 'ERROR: el kubeconfig no define namespace; usa -Namespace student-<usuario>.' -ForegroundColor Red
  exit 1
}
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
  Write-Host "Corrige el bloqueo y vuelve a correr con: -FromPhase $N" -ForegroundColor Yellow
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

function Get-ImageRef([string]$Name) { return "$Registry/soloservis-${Name}:$Tag" }

# Genera k8s/overlays/local (ignorado por git) a partir de k8s/base.
function Write-Overlay {
  $migDir = Join-Path $OverlayDir 'migrations'
  New-Item -ItemType Directory -Force $migDir | Out-Null
  Get-ChildItem $migDir -Filter *.sql -ErrorAction SilentlyContinue | Remove-Item -Force
  $sqls = Get-ChildItem (Join-Path $RepoRoot 'backend/database/migrations') -Filter *.sql | Sort-Object Name
  foreach ($f in $sqls) { Copy-Item $f.FullName (Join-Path $migDir $f.Name) -Force }

  $L = New-Object System.Collections.Generic.List[string]
  $L.Add('# GENERADO por scripts/k8s/deploy.ps1 - no editar ni commitear.')
  $L.Add('apiVersion: kustomize.config.k8s.io/v1beta1')
  $L.Add('kind: Kustomization')
  # Mismo namespace que la base: si no, kustomize no enlaza el ConfigMap con hash al Job.
  $L.Add("namespace: $Namespace")
  $L.Add('resources:')
  $L.Add('  - ../../base')
  $L.Add('configMapGenerator:')
  $L.Add('  - name: soloservis-migrations')
  $L.Add('    files:')
  foreach ($f in $sqls) { $L.Add("      - migrations/$($f.Name)") }
  $L.Add('    options:')
  $L.Add('      labels:')
  $L.Add('        app.kubernetes.io/part-of: soloservis')
  $L.Add('        app.kubernetes.io/component: migrations')
  if ($IngressHost) {
    $L.Add('  - name: soloservis-config')
    $L.Add('    behavior: merge')
    $L.Add('    literals:')
    $L.Add("      - FRONTEND_URL=${IngressScheme}://$IngressHost")
  }
  $L.Add('images:')
  foreach ($n in 'product-api', 'store-api', 'frontend') {
    $L.Add("  - name: soloservis/$n")
    $L.Add("    newName: $Registry/soloservis-$n")
    $L.Add("    newTag: `"$Tag`"")
  }
  $L.Add('patches:')
  if ($StorageClass) {
    $L.Add('  - target: {kind: StatefulSet, name: postgres}')
    $L.Add('    patch: |-')
    $L.Add('      - op: add')
    $L.Add('        path: /spec/volumeClaimTemplates/0/spec/storageClassName')
    $L.Add("        value: $StorageClass")
  }
  if ($IngressClass) {
    $L.Add('  - target: {kind: Ingress, name: soloservis}')
    $L.Add('    patch: |-')
    $L.Add('      - op: add')
    $L.Add('        path: /spec/ingressClassName')
    $L.Add("        value: $IngressClass")
  }
  if ($IngressHost) {
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
  if (-not $Registry) { return 'falta -Registry' }
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
            (Test-Path (Join-Path $RepoRoot 'backend/cmd/product-api')) -and
            (Test-Path (Join-Path $RepoRoot 'backend/cmd/store-api')) -and
            (Test-Path (Join-Path $RepoRoot 'fronted/package.json'))
  $cp['Repositorio'] = $(if ($repoOk) { 'OK' } else { 'ERROR' })

  $missing = @('kubectl', 'docker', 'git') | Where-Object { -not (Get-Command $_ -ErrorAction SilentlyContinue) }
  if ($missing -contains 'kubectl') { Stop-Phase 0 $cp 'kubectl no esta instalado' }
  if (-not (Test-Path $Kubeconfig)) { Stop-Phase 0 $cp 'no existe el kubeconfig indicado (-Kubeconfig)' }

  Step 'kubectl auth can-i (puede abrir el navegador para el login OIDC)'
  $can = (K auth can-i list pods) -join ' '
  if ($script:KExit -ne 0 -or $can -notmatch 'yes') {
    $cp['Acceso Kubernetes'] = 'ERROR'
    Stop-Phase 0 $cp "sin acceso al namespace $Namespace : $can"
  }
  $cp['Acceso Kubernetes'] = 'OK'

  Write-Host "  Recursos actuales en ${Namespace}:" -ForegroundColor DarkGray
  Show (K get 'all,pvc,ingress,configmap,job' -o wide)
  Write-Host '  Secrets (solo nombres):' -ForegroundColor DarkGray
  Show (K get secret -o name)
  Write-Host '  ResourceQuota / LimitRange:' -ForegroundColor DarkGray
  Show (K get 'resourcequota,limitrange')
  Write-Host '  LimitRange (minimos/maximos por contenedor):' -ForegroundColor DarkGray
  Show (K describe limitrange | Where-Object { $_ -match 'Container|Pod|PersistentVolumeClaim|Type' })
  Write-Host '  StorageClasses:' -ForegroundColor DarkGray
  Show (K get storageclass)
  Write-Host '  IngressClasses:' -ForegroundColor DarkGray
  Show (K get ingressclass)
  Write-Host '  Pod Security del namespace:' -ForegroundColor DarkGray
  $ns = KJson get namespace $Namespace
  if ($ns -and $ns.metadata.labels) {
    $ns.metadata.labels.PSObject.Properties | Where-Object { $_.Name -like 'pod-security*' } |
      ForEach-Object { Write-Host "    $($_.Name)=$($_.Value)" }
  } else { Write-Host '    (no visible)' }

  $owned = @(K get 'all,pvc,ingress,configmap,secret,job' -l 'app.kubernetes.io/part-of=soloservis' -o name | Where-Object { $_ -match '/' })
  $legacy = @()
  foreach ($r in 'statefulset/postgres', 'service/postgres', 'secret/postgres-secret', 'pod/storage-test-pod', 'pvc/storage-test', 'pvc/postgres-data-postgres-0') {
    $null = K get $r -o name
    if ($script:KExit -eq 0) { $legacy += $r }
  }
  $cp['Recursos encontrados'] = "$($owned.Count) etiquetados soloservis; heredados: $(if ($legacy) { $legacy -join ', ' } else { 'ninguno' })"
  $cp['Archivos Kubernetes existentes'] = "$(@(Get-ChildItem (Join-Path $RepoRoot 'k8s/base') -Recurse -File).Count) en k8s/base"
  $cp['Bloqueos'] = $(if ($missing) { "no instalado: $($missing -join ', ')" } else { 'ninguno' })
  Checkpoint 0 $cp
}

# ---------------------------------------------------------------- FASE 1

function Phase1 {
  $cp = [ordered]@{}
  if ($SkipCleanup) {
    $cp['Limpieza'] = 'OMITIDA (-SkipCleanup)'
    Checkpoint 1 $cp
    return
  }
  Step 'Borrando recursos etiquetados app.kubernetes.io/part-of=soloservis (los PVC no se tocan)'
  Show (K delete 'deployment,statefulset,service,configmap,secret,job,ingress,pod' -l 'app.kubernetes.io/part-of=soloservis' --ignore-not-found --wait=true --timeout=180s)

  Step 'Borrando recursos heredados de k8s/postgres (version anterior, sin etiquetas)'
  foreach ($r in 'statefulset/postgres', 'service/postgres', 'secret/postgres-secret', 'pod/storage-test-pod', 'pvc/storage-test', 'pod/soloservis-storage-probe', 'pvc/soloservis-storage-probe') {
    Show (K delete $r --ignore-not-found --wait=true --timeout=180s)
  }

  # PVC de PostgreSQL: solo se borra si NO esta Bound (nunca llego a tener datos).
  $pvc = KJson get pvc postgres-data-postgres-0
  if ($pvc) {
    if ($pvc.status.phase -ne 'Bound') {
      Step "PVC postgres-data-postgres-0 en $($pvc.status.phase) (sc=$($pvc.spec.storageClassName)): se elimina"
      Show (K delete pvc postgres-data-postgres-0 --wait=true --timeout=180s)
    } else {
      Step 'PVC postgres-data-postgres-0 esta Bound: se conserva (datos)'
    }
  }

  $left = @(K get 'all,ingress,configmap,secret,job' -l 'app.kubernetes.io/part-of=soloservis' -o name | Where-Object { $_ -match '/' })
  foreach ($r in 'statefulset/postgres', 'pod/storage-test-pod', 'pvc/storage-test') {
    $null = K get $r -o name
    if ($script:KExit -eq 0) { $left += $r }
  }
  $keptPvc = @(K get pvc -o name | Where-Object { $_ -match 'postgres-data-postgres-0' })
  $cp['Limpieza'] = $(if ($left.Count -eq 0) { 'OK' } else { 'ERROR' })
  $restantes = @($left) + @($keptPvc | ForEach-Object { "$_ (conservado, Bound)" })
  $cp['Recursos restantes de SoloServise'] = $(if ($restantes.Count -gt 0) { $restantes -join ', ' } else { 'ninguno' })
  $cp['Recursos globales modificados'] = 'ninguno'
  if ($left.Count -gt 0) { Stop-Phase 1 $cp 'quedaron recursos sin borrar (permisos o finalizers)' }
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
  $result = @{ Ok = $false; Persist = $false; Dns = $false; Detail = '' }

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
    $node1 = (KJson get pod soloservis-storage-probe).spec.nodeName
    Step "PVC $($pvc.status.phase); pod Ready en $node1. Escribiendo archivo de prueba"
    $w = K exec soloservis-storage-probe '--' sh -c 'echo soloservis-ok > /data/probe.txt && sync && ls -ln /data'
    if ($script:KExit -ne 0) {
      Write-Host '  No se pudo escribir en el volumen:' -ForegroundColor Yellow
      Show $w
      $result.Detail += '; escritura FALLO'
    } else {
      $null = K exec soloservis-storage-probe '--' nslookup kubernetes.default
      $result.Dns = ($script:KExit -eq 0)
      # Recrear el pod y leer el archivo: prueba de persistencia real del volumen.
      Step 'Recreando el pod (un volumen RWO puede tardar en moverse de nodo)'
      Show (K delete pod soloservis-storage-probe --wait=true --timeout=180s)
      Show (K apply -f $file)
      $null = K wait --for=condition=Ready pod/soloservis-storage-probe --timeout=420s
      if ($script:KExit -ne 0) {
        Write-Host '  El pod recreado no quedo Ready a tiempo:' -ForegroundColor Yellow
        Show-Diagnostics pod soloservis-storage-probe
        $result.Detail += '; pod recreado NO Ready'
      } else {
        $node2 = (KJson get pod soloservis-storage-probe).spec.nodeName
        $read = (K exec soloservis-storage-probe '--' cat /data/probe.txt) -join ' '
        $result.Persist = ($read -match 'soloservis-ok')
        Step "Pod recreado en $node2; lectura: $read"
        if (-not $result.Persist) { $result.Detail += '; el archivo NO persistio' }
      }
    }
  }
  Show (K delete -f $file --ignore-not-found --wait=true --timeout=180s)
  return $result
}

function Phase2 {
  $cp = [ordered]@{}
  $block = @()

  # --- Storage: probar clases hasta encontrar una que deje el PVC Bound y persista.
  $candidates = @()
  if ($StorageClass) { $candidates = @($StorageClass) }
  else {
    $sc = KJson get storageclass
    if ($sc -and $sc.items) {
      $defs = @()
      foreach ($i in $sc.items) {
        $isDef = ($i.metadata.annotations -and $i.metadata.annotations.'storageclass.kubernetes.io/is-default-class' -eq 'true')
        if ($isDef) { $defs += $i.metadata.name }
        Write-Host ("    {0,-28} {1,-34} {2} {3}" -f $i.metadata.name, $i.provisioner, $i.volumeBindingMode, $(if ($isDef) { '(default)' } else { '' }))
      }
      $rest = @($sc.items | Where-Object { $defs -notcontains $_.metadata.name } | ForEach-Object { $_.metadata.name })
      # '' = sin storageClassName (usa la clase por defecto del cluster)
      if ($defs.Count -gt 0) { $candidates = @('') + $rest } else { $candidates = $rest }
    } else {
      Step 'No se pueden listar StorageClasses (permisos): se prueba la clase por defecto'
      $candidates = @('')
    }
  }
  $chosen = $null
  $dnsOk = $false
  $detail = ''
  foreach ($c in $candidates) {
    $r = Test-StorageClass $c
    if ($r.Ok -and $r.Persist) { $chosen = $c; $dnsOk = $r.Dns; $detail = $r.Detail; break }
  }
  if ($null -ne $chosen) {
    $script:StorageClass = $chosen
    $script:StorageResolved = $true
    $cp['Storage'] = "OK ($(if ($chosen) { $chosen } else { 'clase por defecto' }); $detail; persistencia verificada)"
    $cp['DNS interno'] = $(if ($dnsOk) { 'OK' } else { 'ERROR (nslookup kubernetes.default fallo)' })
  } else {
    $cp['Storage'] = "ERROR ($(if ($r) { $r.Detail } else { 'sin clases candidatas' }))"
    $block += 'almacenamiento persistente'
  }

  # --- Ingress Controller
  if ($IngressClass) {
    $cp['Ingress'] = "OK (clase indicada: $IngressClass)"
  } else {
    $ic = KJson get ingressclass
    if ($ic -and $ic.items -and @($ic.items).Count -gt 0) {
      foreach ($i in $ic.items) { Write-Host "    $($i.metadata.name)  controller=$($i.spec.controller)" }
      $def = @($ic.items | Where-Object { $_.metadata.annotations -and $_.metadata.annotations.'ingressclass.kubernetes.io/is-default-class' -eq 'true' })
      if ($def.Count -ge 1) { $script:IngressClass = $def[0].metadata.name }
      elseif (@($ic.items).Count -eq 1) { $script:IngressClass = @($ic.items)[0].metadata.name }
      if ($script:IngressClass) {
        $ctrl = (@($ic.items) | Where-Object { $_.metadata.name -eq $script:IngressClass }).spec.controller
        $cp['Ingress'] = "OK ($($script:IngressClass), $ctrl)"
      } else {
        $cp['Ingress'] = 'ERROR (varias IngressClass sin default: usar -IngressClass)'
        $block += 'IngressClass ambigua'
      }
    } else {
      $cp['Ingress'] = 'SIN VERIFICAR (no se puede listar IngressClass; se usara la clase por defecto)'
    }
  }
  if (-not $IngressHost) { Step 'Sin -IngressHost: el Ingress respondera a cualquier host (regla sin host)' }

  # --- Imagenes
  if (-not $Registry) {
    $cp['Container images'] = 'ERROR (falta -Registry accesible por los nodos)'
    $block += 'registry de imagenes'
  } elseif (-not (Get-Command docker -ErrorAction SilentlyContinue)) {
    $cp['Container images'] = 'ERROR (docker no instalado)'
    $block += 'docker'
  } else {
    $cp['Container images'] = "OK (registry $Registry, tag $Tag)"
  }

  if ($block.Count -gt 0) {
    $cp['Kubernetes preparado'] = 'ERROR'
    Stop-Phase 2 $cp ($block -join '; ')
  }
  $err = Build-Rendered
  if ($err) { $cp['Kubernetes preparado'] = 'ERROR'; Stop-Phase 2 $cp "kustomize: $err" }
  $cp['Kubernetes preparado'] = 'OK (overlay k8s/overlays/local generado)'
  $cp['Bloqueos'] = 'ninguno'
  Checkpoint 2 $cp
}

# ---------------------------------------------------------------- FASE 3

function Phase3 {
  $cp = [ordered]@{}
  if (-not $Registry) { Stop-Phase 3 $cp 'falta -Registry' }
  $images = @(
    @{ Name = 'product-api'; File = 'docker/product-api/Dockerfile'; Ctx = 'backend' },
    @{ Name = 'store-api';   File = 'docker/store-api/Dockerfile';   Ctx = 'backend' },
    @{ Name = 'frontend';    File = 'docker/frontend/Dockerfile';    Ctx = '.' }
  )
  $allOk = $true
  foreach ($i in $images) {
    $ref = Get-ImageRef $i.Name
    Step "docker build $ref"
    & docker build --platform linux/amd64 -f (Join-Path $RepoRoot $i.File) -t $ref (Join-Path $RepoRoot $i.Ctx)
    $ok = ($LASTEXITCODE -eq 0)
    if ($ok) {
      Step "docker push $ref"
      & docker push $ref
      $ok = ($LASTEXITCODE -eq 0)
    }
    if ($ok) {
      $null = & docker manifest inspect $ref 2>&1
      $ok = ($LASTEXITCODE -eq 0)
    }
    $cp["$($i.Name) image"] = $(if ($ok) { "OK ($ref)" } else { 'ERROR' })
    if (-not $ok) { $allOk = $false }
  }
  $cp['Imagenes disponibles para Kubernetes'] = $(if ($allOk) { 'OK (publicadas en el registry)' } else { 'ERROR' })
  if (-not $allOk) { Stop-Phase 3 $cp 'build/push fallo (revisar docker login y nombre del registry)' }
  Save-State
  $cp['Bloqueos'] = 'ninguno'
  Checkpoint 3 $cp
}

# ---------------------------------------------------------------- FASE 4

function Phase4 {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase 4 $cp "kustomize: $err" }
  Step 'Aplicando Secret, Services y StatefulSet de PostgreSQL'
  $ok = Apply-Component 'app.kubernetes.io/component=database'
  $null = K get statefulset postgres -o name
  $cp['StatefulSet'] = $(if ($script:KExit -eq 0) { 'OK' } else { 'ERROR' })
  $null = K get service postgres -o name
  $cp['Service'] = $(if ($script:KExit -eq 0) { 'OK (postgres:5432 ClusterIP)' } else { 'ERROR' })
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
    # El describe del Job no dice por que no corre: mirar el pod y los eventos.
    Write-Host '  Pods del Job:' -ForegroundColor Yellow
    Show (K get pods -l job-name=soloservis-migrate -o wide)
    foreach ($p in @((KJson get pods -l job-name=soloservis-migrate).items)) {
      if ($p) { Show-Diagnostics pod $p.metadata.name }
    }
    Write-Host '  Eventos recientes del namespace:' -ForegroundColor Yellow
    Show (K get events --sort-by=.lastTimestamp) -Tail 15
    Stop-Phase 5 $cp 'el Job de migraciones no termino correctamente'
  }
  $files = @(Get-ChildItem (Join-Path $RepoRoot 'backend/database/migrations') -Filter *.sql).Count
  $count = PgQuery 'SELECT count(*) FROM schema_migrations;'
  $applied = PgQuery "SELECT string_agg(version, ', ' ORDER BY version) FROM schema_migrations;"
  $cp['Migraciones aplicadas'] = $(if ("$count" -eq "$files") { "OK ($applied)" } else { "ERROR ($count de $files)" })
  if ("$count" -ne "$files") { $cp['Base de datos lista'] = 'NO'; Stop-Phase 5 $cp 'faltan migraciones' }
  $products = PgQuery 'SELECT count(*) FROM product;'
  $cp['Base de datos lista'] = "SI ($products productos)"
  Checkpoint 5 $cp
}

# ---------------------------------------------------------------- FASES 6-8

function Deploy-App([int]$N, [string]$Name, [int]$Port, [string]$HealthPath, [bool]$UsesDb) {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase $N $cp "kustomize: $err" }
  if ($UsesDb) { $null = Apply-Component 'app.kubernetes.io/component=config' }
  $ok = Apply-Component "app.kubernetes.io/name=$Name"
  $cp['Deployment'] = $(if ($ok) { 'OK' } else { 'ERROR' })
  if (-not $ok) { Stop-Phase $N $cp 'kubectl apply fallo' }

  $null = K rollout status "deployment/$Name" --timeout=240s
  $pod = @(Get-PodsByName $Name | Sort-Object { $_.metadata.creationTimestamp } -Descending)[0]
  $cp['Pod'] = Get-PodState $pod
  $ready = Test-PodReady $pod
  $cp['Readiness'] = $(if ($ready) { 'OK' } else { 'ERROR' })
  $null = K get service $Name -o name
  $cp['Service'] = $(if ($script:KExit -eq 0) { "OK ($Name ClusterIP)" } else { 'ERROR' })

  if (-not $ready) {
    if ($pod) { Show-Diagnostics pod $pod.metadata.name }
    Show (K logs "deployment/$Name" --tail=30)
    if ("$($cp['Pod'])" -match 'ImagePull|ErrImage') {
      Stop-Phase $N $cp 'el cluster no puede descargar la imagen (registry privado: crear Secret docker-registry y usar -ImagePullSecret)'
    }
    Stop-Phase $N $cp "$Name no quedo Ready"
  }
  $health = (K exec "deployment/$Name" '--' wget -qO- -T 5 "http://127.0.0.1:$Port$HealthPath") -join ' '
  if ($UsesDb) {
    $cp['Conexion PostgreSQL'] = $(if ($health -match '"database":"connected"') { 'OK (postgres:5432 via DNS del Service)' } else { "ERROR ($health)" })
  } else {
    $cp['Build frontend'] = $(if ($health -match 'ok') { 'OK (nginx sirve el build de Vite)' } else { "ERROR ($health)" })
  }
  Checkpoint $N $cp
}

function Phase6 { Deploy-App 6 'product-api' 8081 '/health' $true }
function Phase7 { Deploy-App 7 'store-api' 8082 '/health' $true }
function Phase8 { Deploy-App 8 'frontend' 8080 '/healthz' $false }

# ---------------------------------------------------------------- FASE 9

function Invoke-Probe([string]$Url, [string]$Expect) {
  # curl.exe (incluido en Windows 10+): sigue redirecciones http->https y acepta
  # el certificado aun no automatizado de la plataforma (-k), cosa que
  # Invoke-WebRequest de PowerShell 5.1 no permite.
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

function Phase9 {
  $cp = [ordered]@{}
  $err = Build-Rendered
  if ($err) { Stop-Phase 9 $cp "kustomize: $err" }
  $ok = Apply-Component 'app.kubernetes.io/component=ingress'
  if (-not $ok) { $cp['Ingress'] = 'ERROR'; Stop-Phase 9 $cp 'kubectl apply fallo' }

  Step 'Esperando direccion del Ingress (max 2 min)'
  $addr = ''
  $deadline = (Get-Date).AddSeconds(120)
  while ((Get-Date) -lt $deadline) {
    $ing = KJson get ingress soloservis
    $lb = $null
    if ($ing -and $ing.status.loadBalancer.ingress) { $lb = @($ing.status.loadBalancer.ingress)[0] }
    if ($lb) { $addr = $(if ($lb.hostname) { $lb.hostname } else { $lb.ip }); break }
    Start-Sleep -Seconds 5
  }
  Show (K get ingress soloservis)
  $hostForTest = $(if ($IngressHost) { $IngressHost } else { $addr })
  $cp['Ingress'] = $(if ($addr -or $IngressHost) { "OK (host=$(if ($IngressHost) { $IngressHost } else { '*' }), address=$addr)" } else { 'ERROR (sin address)' })
  if (-not $hostForTest) { Stop-Phase 9 $cp 'el Ingress no obtuvo direccion y no se indico -IngressHost' }

  $base = "${IngressScheme}://$hostForTest"
  $cp['Frontend accesible'] = Invoke-Probe "$base/" '<div id="root"'
  $cp['product-api accesible'] = Invoke-Probe "$base/products" '\['
  $cp['store-api accesible'] = Invoke-Probe "$base/stores" '\['
  $script:IngressBase = $base
  Checkpoint 9 $cp
  if (@($cp.Values | Where-Object { "$_" -like 'ERROR*' }).Count -gt 0) {
    Write-Host '  Nota: si el address es interno (campus/VPN), prueba desde esa red o define -IngressHost.' -ForegroundColor Yellow
  }
}

# ---------------------------------------------------------------- FASE 10

function Test-ApiHealth([string]$Name, [int]$Port) {
  $h = (K exec "deployment/$Name" '--' wget -qO- -T 5 "http://127.0.0.1:$Port/health") -join ' '
  return ($h -match '"database":"connected"')
}

function Phase10 {
  $cp = [ordered]@{}
  foreach ($what in 'pods', 'pvc', 'deployments', 'statefulsets', 'services', 'ingress') {
    Write-Host "  kubectl get $what" -ForegroundColor DarkGray
    Show (K get $what -l 'app.kubernetes.io/part-of=soloservis')
  }

  $productOk = Test-ApiHealth 'product-api' 8081
  $storeOk = Test-ApiHealth 'store-api' 8082
  # frontend -> APIs por DNS de los Services (mismo destino que usa el Ingress)
  $f1 = (K exec deployment/frontend '--' wget -qO- -T 5 'http://product-api:8081/health') -join ' '
  $f2 = (K exec deployment/frontend '--' wget -qO- -T 5 'http://store-api:8082/health') -join ' '
  $frontOk = ($f1 -match 'connected') -and ($f2 -match 'connected')

  # Recreacion de un Pod administrado por un Deployment.
  Step 'Borrando el pod de product-api: Kubernetes debe recrearlo'
  $old = @(Get-PodsByName 'product-api')[0]
  if ($old) { Show (K delete pod $old.metadata.name --wait=true --timeout=120s) }
  $null = K rollout status deployment/product-api --timeout=180s
  $new = @(Get-PodsByName 'product-api' | Where-Object { Test-PodReady $_ })[0]
  $recreated = [bool]($new -and $old -and $new.metadata.name -ne $old.metadata.name)

  # Persistencia: reiniciar postgres-0 sin tocar el PVC y comparar datos.
  Step 'Reiniciando postgres-0 (el PVC NO se toca) para verificar persistencia'
  $pvc = KJson get pvc postgres-data-postgres-0
  $before = PgQuery 'SELECT count(*) FROM schema_migrations;'
  Show (K delete pod postgres-0 --wait=true --timeout=120s)
  $null = K rollout status statefulset/postgres --timeout=300s
  $null = K wait --for=condition=Ready pod/postgres-0 --timeout=180s
  $after = PgQuery 'SELECT count(*) FROM schema_migrations;'
  $pvcAfter = KJson get pvc postgres-data-postgres-0
  $persist = [bool]($before -and "$before" -eq "$after" -and $pvc -and $pvcAfter -and
                    $pvcAfter.metadata.uid -eq $pvc.metadata.uid -and $pvcAfter.status.phase -eq 'Bound')
  # Tras reiniciar la BD, las APIs deben volver a conectarse solas.
  Start-Sleep -Seconds 10
  $productOk = $productOk -and (Test-ApiHealth 'product-api' 8081)
  $storeOk = $storeOk -and (Test-ApiHealth 'store-api' 8082)

  $ingressOk = $false
  if ($script:IngressBase) {
    $ingressOk = ((Invoke-Probe "$($script:IngressBase)/products" '\[') -like 'OK*') -and
                 ((Invoke-Probe "$($script:IngressBase)/stores" '\[') -like 'OK*') -and
                 ((Invoke-Probe "$($script:IngressBase)/" '<div id="root"') -like 'OK*')
  }

  $pgOk = Test-PodReady (KJson get pod postgres-0)
  $cp['Kubernetes'] = $(if ($recreated) { 'FUNCIONAL (recreo el pod de product-api)' } else { 'ERROR (no se verifico la recreacion)' })
  $cp['PostgreSQL'] = $(if ($pgOk) { 'FUNCIONAL' } else { 'ERROR' })
  $cp['Persistencia'] = $(if ($persist) { "FUNCIONAL (PVC Bound, mismos datos tras reiniciar: $after migraciones)" } else { "ERROR (antes=$before despues=$after)" })
  $cp['product-api'] = $(if ($productOk) { 'FUNCIONAL' } else { 'ERROR' })
  $cp['store-api'] = $(if ($storeOk) { 'FUNCIONAL' } else { 'ERROR' })
  $cp['frontend'] = $(if ($frontOk) { 'FUNCIONAL (frontend -> APIs por Service OK)' } else { 'ERROR' })
  $cp['Ingress'] = $(if ($ingressOk) { 'FUNCIONAL' } elseif ($script:IngressBase) { 'ERROR' } else { 'SIN VERIFICAR (ejecutar la Fase 9)' })
  Checkpoint 10 $cp
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
if (-not $IngressHost) { $IngressHost = "soloservis-$StudentUser.dev.censei.cl" }
if ($Registry -and $Registry -notmatch '^[a-z0-9.\-:]+(/[a-z0-9._\-]+)+$') {
  Write-Host "ERROR: -Registry '$Registry' no es valido. Usa tu usuario real en minusculas, ej: docker.io/miusuario" -ForegroundColor Red
  exit 1
}
if (-not $Tag) {
  if ($FromPhase -gt 3) { Write-Host 'AVISO: no hay tag guardado de la Fase 3; se genera uno nuevo (las imagenes deben existir con ese tag). Usa -Tag si ya publicaste otro.' -ForegroundColor Yellow }
  $Tag = New-ImageTag
}
$script:IngressBase = $(if ($IngressHost) { "${IngressScheme}://$IngressHost" } else { '' })
if ($FromPhase -gt 2 -and -not $script:StorageResolved -and $ToPhase -ge 4) {
  Write-Host 'AVISO: no hay StorageClass validada (Fase 2). Se usara la clase por defecto del cluster.' -ForegroundColor Yellow
}

Write-Host "SOLOServis -> Kubernetes | contexto=$(if ($Context) { $Context } else { '(el del kubeconfig)' }) namespace=$Namespace fases=$FromPhase..$ToPhase" -ForegroundColor Cyan
Write-Host "Kubeconfig: $(Split-Path $Kubeconfig -Leaf) | registry=$(if ($Registry) { $Registry } else { '(sin definir)' }) tag=$Tag" -ForegroundColor Cyan

$phases = @{
  0 = { Phase0 }; 1 = { Phase1 }; 2 = { Phase2 }; 3 = { Phase3 }; 4 = { Phase4 }; 5 = { Phase5 }
  6 = { Phase6 }; 7 = { Phase7 }; 8 = { Phase8 }; 9 = { Phase9 }; 10 = { Phase10 }
}
for ($n = $FromPhase; $n -le $ToPhase; $n++) {
  Write-Host ''
  Write-Host "---------------- FASE $n ----------------" -ForegroundColor White
  & $phases[$n]
}
Write-Host 'Listo.' -ForegroundColor Green
