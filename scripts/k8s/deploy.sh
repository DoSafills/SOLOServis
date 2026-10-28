#!/usr/bin/env bash
# Despliega SOLOServis en Kubernetes por fases (0-9), con checkpoint por fase.
# Equivalente en bash de scripts/k8s/deploy.ps1 (Linux, macOS y Git Bash).
#
# Modos (ver k8s/README.md):
#   local  Kubernetes de Docker Desktop. Una API por Deployment (microservicios),
#          imagenes construidas en tu maquina (sin registry) y acceso con
#          kubectl port-forward en http://localhost:8080.
#   uct    Cluster de la universidad con TU kubeconfig. Las 14 APIs en un solo
#          Deployment "backend" (la cuota no admite 14), imagenes publicadas en
#          un registry y acceso por Ingress en soloservis-<usuario>.dev.censei.cl.
#
# Ejemplos:
#   ./scripts/k8s/deploy.sh --modo local
#   ./scripts/k8s/deploy.sh --modo uct --kubeconfig ~/.kube/estudiantes-<usuario>.kubeconfig \
#       --registry docker.io/<usuario-docker>
#   ./scripts/k8s/deploy.sh --modo uct --to-phase 0          # solo inventario
#   ./scripts/k8s/deploy.sh --modo local --from-phase 6 --skip-cleanup
#
# Nunca imprime Secrets, tokens ni el contenido del kubeconfig.
set -uo pipefail

usage() {
  sed -n '2,20p' "$0" | sed 's/^# \{0,1\}//'
  cat <<'EOF'
Opciones:
  --modo local|uct            (obligatorio)
  --topologia microservicios|consolidado   (por defecto: microservicios en local, consolidado en uct)
  --kubeconfig RUTA  --context NOMBRE  --namespace NOMBRE
  --registry docker.io/usuario   (uct)   --tag TAG
  --storage-class X  --ingress-class X  --ingress-host X  --ingress-scheme http|https  --image-pull-secret X
  --local-port 8080  (local)
  --from-phase N  --to-phase N  (0-9)   --skip-cleanup
EOF
}

MODO="" TOPOLOGIA="" KUBECONFIG_FILE="" CONTEXT="" NAMESPACE="" REGISTRY="" TAG=""
STORAGE_CLASS="" INGRESS_CLASS="" INGRESS_HOST="" INGRESS_SCHEME="http" IMAGE_PULL_SECRET=""
LOCAL_PORT=8080 FROM_PHASE=0 TO_PHASE=9 SKIP_CLEANUP=0
while [ $# -gt 0 ]; do
  case "$1" in
    --modo) MODO="$2"; shift 2 ;;
    --topologia) TOPOLOGIA="$2"; shift 2 ;;
    --kubeconfig) KUBECONFIG_FILE="$2"; shift 2 ;;
    --context) CONTEXT="$2"; shift 2 ;;
    --namespace) NAMESPACE="$2"; shift 2 ;;
    --registry) REGISTRY="$2"; shift 2 ;;
    --tag) TAG="$2"; shift 2 ;;
    --storage-class) STORAGE_CLASS="$2"; shift 2 ;;
    --ingress-class) INGRESS_CLASS="$2"; shift 2 ;;
    --ingress-host) INGRESS_HOST="$2"; shift 2 ;;
    --ingress-scheme) INGRESS_SCHEME="$2"; shift 2 ;;
    --image-pull-secret) IMAGE_PULL_SECRET="$2"; shift 2 ;;
    --local-port) LOCAL_PORT="$2"; shift 2 ;;
    --from-phase) FROM_PHASE="$2"; shift 2 ;;
    --to-phase) TO_PHASE="$2"; shift 2 ;;
    --skip-cleanup) SKIP_CLEANUP=1; shift ;;
    -h|--help) usage; exit 0 ;;
    *) echo "ERROR: opcion desconocida: $1" >&2; usage; exit 1 ;;
  esac
done
case "$MODO" in local|uct) ;; *) echo "ERROR: indica --modo local o --modo uct" >&2; usage; exit 1 ;; esac

REPO_ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
OVERLAY_DIR="$REPO_ROOT/k8s/overlays/$MODO"
STATE_FILE="$OVERLAY_DIR/state.env"
RENDERED="$OVERLAY_DIR/rendered.yaml"
IS_LOCAL=0; [ "$MODO" = local ] && IS_LOCAL=1
if [ -z "$TOPOLOGIA" ]; then
  if [ $IS_LOCAL = 1 ]; then TOPOLOGIA=microservicios; else TOPOLOGIA=consolidado; fi
fi
# APIs segun la topologia, como "nombre:puerto" (bash 3.2 de macOS no tiene arrays asociativos).
if [ "$TOPOLOGIA" = microservicios ]; then
  APIS="product-api:8081 store-api:8082 service-api:8083 user-api:8084 pricing-api:8085 search-api:8086 comparison-api:8087 auth-api:8088 favorites-api:8089 cart-api:8090 review-api:8091 watchlist-api:8092 notification-api:8093 ingestion-api:8094"
else
  APIS="backend:8080"
fi

# Colores solo si la salida es una terminal.
if [ -t 1 ]; then C_CYAN=$'\033[36m' C_GREEN=$'\033[32m' C_RED=$'\033[31m' C_YELLOW=$'\033[33m' C_DIM=$'\033[2m' C_OFF=$'\033[0m'
else C_CYAN="" C_GREEN="" C_RED="" C_YELLOW="" C_DIM="" C_OFF=""; fi

# --- kubeconfig, contexto y namespace
if [ -z "$KUBECONFIG_FILE" ]; then
  if [ -n "${KUBECONFIG:-}" ]; then KUBECONFIG_FILE="${KUBECONFIG%%:*}"
  elif [ -f "$HOME/.kube/config" ]; then KUBECONFIG_FILE="$HOME/.kube/config"
  fi
fi
if [ -z "$KUBECONFIG_FILE" ] || [ ! -f "$KUBECONFIG_FILE" ]; then
  if [ $IS_LOCAL = 1 ]; then echo "${C_RED}ERROR: no hay ~/.kube/config. Activa Kubernetes en Docker Desktop (Settings > Kubernetes).${C_OFF}"
  else echo "${C_RED}ERROR: indica tu kubeconfig con --kubeconfig (descargalo desde https://estudiantes.dev.censei.cl).${C_OFF}"; fi
  exit 1
fi
[ $IS_LOCAL = 1 ] && [ -z "$CONTEXT" ] && CONTEXT=docker-desktop
KBASE=(--kubeconfig "$KUBECONFIG_FILE")
[ -n "$CONTEXT" ] && KBASE+=(--context "$CONTEXT")
if [ -z "$NAMESPACE" ]; then
  if [ $IS_LOCAL = 1 ]; then NAMESPACE=soloservis
  else NAMESPACE="$(kubectl "${KBASE[@]}" config view --minify -o 'jsonpath={..namespace}' 2>/dev/null)"; fi
fi
if [ -z "$NAMESPACE" ]; then echo "${C_RED}ERROR: el kubeconfig no define namespace; usa --namespace student-<usuario>.${C_OFF}"; exit 1; fi
KNONS=("${KBASE[@]}")
KBASE+=(-n "$NAMESPACE")
# student-marcelo-vidal-22843887e32d -> marcelo-vidal (sin el sufijo hash)
STUDENT_USER="$(echo "$NAMESPACE" | sed -E 's/^student-//; s/-[0-9a-f]{8,}$//')"

# ---------------------------------------------------------------- helpers

k() { kubectl "${KBASE[@]}" "$@"; }
show() { sed 's/^/    /'; }
step() { echo "${C_DIM}  - $*${C_OFF}"; }

CP_KEYS=() CP_VALS=()
cp_reset() { CP_KEYS=(); CP_VALS=(); }
cp_set() { CP_KEYS+=("$1"); CP_VALS+=("$2"); }
checkpoint() {
  echo; echo "${C_CYAN}=== FASE $1 ===${C_OFF}"
  local i v color
  for i in "${!CP_KEYS[@]}"; do
    v="${CP_VALS[$i]}"; color=""
    case "$v" in OK*|SI*|FUNCIONAL*|ninguno*) color="$C_GREEN" ;; ERROR*|NO*|BLOQUEADO*) color="$C_RED" ;; esac
    echo "${color}${CP_KEYS[$i]}: ${v}${C_OFF}"
  done
  echo
}
stop_phase() {
  cp_set Bloqueos "$2"; checkpoint "$1"
  echo "${C_RED}Proceso detenido en la Fase $1.${C_OFF}"
  echo "${C_YELLOW}Corrige el bloqueo y vuelve a correr con: --modo $MODO --from-phase $1${C_OFF}"
  exit 1
}
diagnostics() {
  echo "${C_YELLOW}  Diagnostico $1/$2 (ultimas lineas de describe):${C_OFF}"
  k describe "$1" "$2" 2>&1 | tail -20 | show
}
newest_pod() { k get pods -l "app.kubernetes.io/name=$1" --sort-by=.metadata.creationTimestamp -o name 2>/dev/null | tail -1 | sed 's#^pod/##'; }
pod_ready() { [ "$(k get pod "$1" -o 'jsonpath={.status.conditions[?(@.type=="Ready")].status}' 2>/dev/null)" = True ]; }
pod_state() {
  local phase reason
  phase="$(k get pod "$1" -o 'jsonpath={.status.phase}' 2>/dev/null)"
  reason="$(k get pod "$1" -o 'jsonpath={.status.containerStatuses[*].state.waiting.reason}' 2>/dev/null)"
  if [ -z "$phase" ]; then echo "ERROR (sin pod)"; elif [ -n "$reason" ]; then echo "$phase ($reason)"; else echo "$phase"; fi
}
pg_query() {
  printf '%s\n' "$1" | k exec -i postgres-0 -c postgres -- sh -c 'psql -X -q -tA -v ON_ERROR_STOP=1 -U $POSTGRES_USER -d $POSTGRES_DB' 2>&1 | tr -d '\r'
}
api_health() { k exec "deployment/$1" -- wget -qO- -T 5 "http://127.0.0.1:$2/health" 2>/dev/null | grep -q '"database":"connected"'; }

# Prueba HTTP desde tu maquina (-k: certificado de la plataforma aun no automatizado).
probe() {
  local body code
  body="$(mktemp)"
  code="$(curl -s -k -L -m 15 -o "$body" -w '%{http_code}' "$1" 2>/dev/null)"
  if [ "$code" = 200 ] && grep -Eq "$2" "$body"; then echo "OK (200 $1)"
  elif [ "$code" = 000 ]; then echo "ERROR (sin respuesta: DNS o red; $1)"
  else echo "ERROR ($code $1)"; fi
  rm -f "$body"
}
# Rutas publicas que la app usa, a traves del nginx del frontend.
public_routes() {
  local base="$1" ok=0 r
  r="$(probe "$base/" '<div id="root"')"; cp_set Frontend "$r"; case "$r" in OK*) ;; *) ok=1 ;; esac
  r="$(probe "$base/products?limit=1" '^\[')"; cp_set "Catalogo (products)" "$r"; case "$r" in OK*) ;; *) ok=1 ;; esac
  r="$(probe "$base/categories" '"items"')"; cp_set Categorias "$r"; case "$r" in OK*) ;; *) ok=1 ;; esac
  r="$(probe "$base/stores" '^\[')"; cp_set Tiendas "$r"; case "$r" in OK*) ;; *) ok=1 ;; esac
  r="$(probe "$base/services" '^\[')"; cp_set Servicios "$r"; case "$r" in OK*) ;; *) ok=1 ;; esac
  r="$(probe "$base/search/products?q=a&limit=1" '^\[')"; cp_set Busqueda "$r"; case "$r" in OK*) ;; *) ok=1 ;; esac
  return $ok
}

# ---------------------------------------------------------------- estado local

save_state() {
  mkdir -p "$OVERLAY_DIR"
  cat > "$STATE_FILE" <<EOF
PREV_REGISTRY='$REGISTRY'
PREV_TAG='$TAG'
PREV_STORAGE_CLASS='$STORAGE_CLASS'
PREV_STORAGE_RESOLVED='$STORAGE_RESOLVED'
PREV_INGRESS_CLASS='$INGRESS_CLASS'
PREV_INGRESS_HOST='$INGRESS_HOST'
PREV_IMAGE_PULL_SECRET='$IMAGE_PULL_SECRET'
EOF
}
new_tag() {
  local sha
  sha="$(git -C "$REPO_ROOT" rev-parse --short HEAD 2>/dev/null || echo local)"
  if [ -n "$(git -C "$REPO_ROOT" status --porcelain 2>/dev/null)" ]; then echo "$sha-$(date +%Y%m%d%H%M)"; else echo "$sha"; fi
}
# Imagenes: "nombre|Dockerfile|contexto"
images() {
  echo "frontend|docker/frontend/Dockerfile|."
  local a n
  for a in $APIS; do
    n="${a%%:*}"
    if [ "$n" = backend ]; then echo "backend|docker/all-apis/Dockerfile|backend"; else echo "$n|docker/$n/Dockerfile|backend"; fi
  done
}
image_ref() {
  if [ $IS_LOCAL = 1 ]; then echo "soloservis/$1:$TAG"; else echo "$REGISTRY/soloservis-$1:$TAG"; fi
}
frontend_url() {
  if [ $IS_LOCAL = 1 ]; then echo "http://localhost:$LOCAL_PORT"; else echo "$INGRESS_SCHEME://$INGRESS_HOST"; fi
}

# Genera k8s/overlays/<modo> (ignorado por git) a partir de base + componente.
write_overlay() {
  local mig="$OVERLAY_DIR/migrations" f line name ref
  mkdir -p "$mig"
  rm -f "$mig"/*.sql
  cp "$REPO_ROOT"/backend/database/migrations/*.sql "$mig/"
  cp "$REPO_ROOT/backend/database/migration_order.txt" "$mig/migration_order.txt"
  {
    echo "# GENERADO por scripts/k8s/deploy.sh - no editar ni commitear."
    echo "apiVersion: kustomize.config.k8s.io/v1beta1"
    echo "kind: Kustomization"
    echo "namespace: $NAMESPACE"
    echo "resources:"
    echo "  - ../../base"
    echo "components:"
    echo "  - ../../components/$TOPOLOGIA"
    echo "configMapGenerator:"
    echo "  - name: soloservis-migrations"
    echo "    files:"
    for f in "$mig"/*.sql; do echo "      - migrations/$(basename "$f")"; done
    echo "      - migrations/migration_order.txt"
    echo "    options:"
    echo "      labels:"
    echo "        app.kubernetes.io/part-of: soloservis"
    echo "        app.kubernetes.io/component: migrations"
    echo "  - name: soloservis-config"
    echo "    behavior: merge"
    echo "    literals:"
    echo "      - FRONTEND_URL=$(frontend_url)"
    echo "images:"
    while IFS='|' read -r name _ _; do
      ref="$(image_ref "$name")"
      echo "  - name: soloservis/$name"
      echo "    newName: ${ref%:*}"
      echo "    newTag: \"$TAG\""
    done < <(images)
    echo "patches:"
    if [ $IS_LOCAL = 1 ]; then
      # Sin Ingress Controller en Docker Desktop: se entra con kubectl port-forward.
      printf '%s\n' "  - patch: |-" "      \$patch: delete" "      apiVersion: networking.k8s.io/v1" "      kind: Ingress" "      metadata:" "        name: soloservis"
    fi
    if [ -n "$STORAGE_CLASS" ]; then
      printf '%s\n' "  - target: {kind: StatefulSet, name: postgres}" "    patch: |-" "      - op: add" "        path: /spec/volumeClaimTemplates/0/spec/storageClassName" "        value: $STORAGE_CLASS"
    fi
    if [ $IS_LOCAL = 0 ] && [ -n "$INGRESS_CLASS" ]; then
      printf '%s\n' "  - target: {kind: Ingress, name: soloservis}" "    patch: |-" "      - op: add" "        path: /spec/ingressClassName" "        value: $INGRESS_CLASS"
    fi
    if [ $IS_LOCAL = 0 ] && [ -n "$INGRESS_HOST" ]; then
      printf '%s\n' "  - target: {kind: Ingress, name: soloservis}" "    patch: |-" "      - op: add" "        path: /spec/rules/0/host" "        value: $INGRESS_HOST"
    fi
    if [ -n "$IMAGE_PULL_SECRET" ]; then
      printf '%s\n' "  - target: {kind: Deployment}" "    patch: |-" "      - op: add" "        path: /spec/template/spec/imagePullSecrets" "        value: [{name: $IMAGE_PULL_SECRET}]"
    fi
  } > "$OVERLAY_DIR/kustomization.yaml.tmp"
  # Sin parches: quitar la clave "patches:" vacia.
  if [ "$(tail -1 "$OVERLAY_DIR/kustomization.yaml.tmp")" = "patches:" ]; then sed -i.bak '$d' "$OVERLAY_DIR/kustomization.yaml.tmp" && rm -f "$OVERLAY_DIR/kustomization.yaml.tmp.bak"; fi
  mv "$OVERLAY_DIR/kustomization.yaml.tmp" "$OVERLAY_DIR/kustomization.yaml"
  save_state
}
build_rendered() {
  if [ $IS_LOCAL = 0 ] && [ -z "$REGISTRY" ]; then echo "falta --registry"; return 1; fi
  write_overlay
  local out
  if ! out="$(kubectl kustomize "$OVERLAY_DIR" 2>&1)"; then echo "$out"; return 1; fi
  printf '%s\n' "$out" > "$RENDERED"
}
apply_selector() { k apply -f "$RENDERED" -l "$1" 2>&1 | show; return "${PIPESTATUS[0]}"; }

# ---------------------------------------------------------------- FASE 0

phase0() {
  cp_reset
  if [ -f "$REPO_ROOT/k8s/base/kustomization.yaml" ] && [ -d "$REPO_ROOT/k8s/components/$TOPOLOGIA" ] && [ -d "$REPO_ROOT/backend/cmd/all-apis" ]; then
    cp_set Repositorio OK; else cp_set Repositorio ERROR; fi
  cp_set Modo "$MODO ($TOPOLOGIA: $(echo $APIS | wc -w | tr -d ' ') Deployment(s) de API)"
  local missing="" t
  for t in kubectl docker git curl; do command -v "$t" >/dev/null 2>&1 || missing="$missing $t"; done
  case "$missing" in *kubectl*) stop_phase 0 "kubectl no esta instalado" ;; esac
  if [ $IS_LOCAL = 1 ]; then
    kubectl --kubeconfig "$KUBECONFIG_FILE" config get-contexts -o name 2>/dev/null | grep -qx "$CONTEXT" ||
      stop_phase 0 "no existe el contexto '$CONTEXT': activa Kubernetes en Docker Desktop (Settings > Kubernetes > Enable Kubernetes)"
    kubectl "${KNONS[@]}" get nodes >/dev/null 2>&1 || stop_phase 0 "el cluster local no responde"
    cp_set "Acceso Kubernetes" "OK (contexto $CONTEXT)"
  else
    step "kubectl auth can-i (puede abrir el navegador para el login OIDC)"
    [ "$(k auth can-i list pods 2>&1)" = yes ] || stop_phase 0 "sin acceso al namespace $NAMESPACE"
    cp_set "Acceso Kubernetes" OK
  fi
  echo "${C_DIM}  Recursos actuales en $NAMESPACE:${C_OFF}"; k get all,pvc,ingress,configmap,job -o wide 2>&1 | show
  echo "${C_DIM}  Secrets (solo nombres):${C_OFF}"; k get secret -o name 2>&1 | show
  if [ $IS_LOCAL = 0 ]; then echo "${C_DIM}  ResourceQuota / LimitRange:${C_OFF}"; k get resourcequota,limitrange 2>&1 | show; fi
  echo "${C_DIM}  StorageClasses:${C_OFF}"; k get storageclass 2>&1 | show
  cp_set "Recursos encontrados" "$(k get all,pvc,ingress,configmap,secret,job -l app.kubernetes.io/part-of=soloservis -o name 2>/dev/null | grep -c /) etiquetados soloservis"
  if [ -n "$missing" ]; then cp_set Bloqueos "no instalado:$missing"; else cp_set Bloqueos ninguno; fi
  checkpoint 0
}

# ---------------------------------------------------------------- FASE 1

phase1() {
  cp_reset
  if [ $IS_LOCAL = 1 ] && ! kubectl "${KNONS[@]}" get namespace "$NAMESPACE" >/dev/null 2>&1; then
    step "Creando el namespace $NAMESPACE"
    kubectl "${KNONS[@]}" create namespace "$NAMESPACE" 2>&1 | show
  fi
  if [ $SKIP_CLEANUP = 1 ]; then cp_set Limpieza "OMITIDA (--skip-cleanup)"; checkpoint 1; return; fi
  # Recursos de un despliegue anterior (por ejemplo, de la otra topologia).
  # El Secret de postgres y el PVC se conservan.
  step "Borrando recursos etiquetados app.kubernetes.io/part-of=soloservis (PVC y Secrets se conservan)"
  k delete deployment,statefulset,service,configmap,job,ingress,pod -l app.kubernetes.io/part-of=soloservis --ignore-not-found --wait=true --timeout=180s 2>&1 | show
  local phase
  phase="$(k get pvc postgres-data-postgres-0 -o 'jsonpath={.status.phase}' 2>/dev/null)"
  if [ -n "$phase" ] && [ "$phase" != Bound ]; then
    step "PVC postgres-data-postgres-0 en $phase: se elimina"
    k delete pvc postgres-data-postgres-0 --wait=true --timeout=180s 2>&1 | show
  elif [ "$phase" = Bound ]; then
    step "PVC postgres-data-postgres-0 esta Bound: se conserva (datos)"
  fi
  local left
  left="$(k get deployment,statefulset,service,job,ingress -l app.kubernetes.io/part-of=soloservis -o name 2>/dev/null | tr '\n' ' ')"
  if [ -n "${left// /}" ]; then cp_set Limpieza ERROR; stop_phase 1 "quedaron recursos sin borrar: $left"; fi
  cp_set Limpieza OK
  if [ "$phase" = Bound ]; then cp_set Conservado "PVC de PostgreSQL (Bound, con datos)"; else cp_set Conservado ninguno; fi
  cp_set Bloqueos ninguno
  checkpoint 1
}

# ---------------------------------------------------------------- FASE 2

# Prueba simple: PVC Bound, pod Ready y escritura en el volumen.
test_storage() {
  local class="$1" file="$OVERLAY_DIR/storage-probe.yaml" sc_line="" ok=1
  [ -n "$class" ] && sc_line="  storageClassName: $class"
  mkdir -p "$OVERLAY_DIR"
  cat > "$file" <<EOF
apiVersion: v1
kind: PersistentVolumeClaim
metadata:
  name: soloservis-storage-probe
  labels: {app.kubernetes.io/part-of: soloservis, app.kubernetes.io/component: storage-probe}
spec:
  accessModes: [ReadWriteOnce]
$sc_line
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
EOF
  step "Probando almacenamiento: ${class:-(clase por defecto)}"
  k apply -f "$file" 2>&1 | show
  if k wait --for=condition=Ready pod/soloservis-storage-probe --timeout=240s >/dev/null 2>&1 &&
     k exec soloservis-storage-probe -- sh -c 'echo soloservis-ok > /data/probe.txt && sync && cat /data/probe.txt' 2>/dev/null | grep -q soloservis-ok; then
    ok=0
  else
    diagnostics pvc soloservis-storage-probe
  fi
  k delete -f "$file" --ignore-not-found --wait=true --timeout=180s 2>&1 | show
  return $ok
}

phase2() {
  cp_reset
  local block=""
  if [ $IS_LOCAL = 1 ]; then
    cp_set Storage "OK (clase por defecto del cluster local)"
    cp_set Ingress "NO SE USA (modo local: kubectl port-forward en localhost:$LOCAL_PORT)"
  else
    local pvc_sc
    pvc_sc="$(k get pvc postgres-data-postgres-0 -o 'jsonpath={.status.phase} {.spec.storageClassName}' 2>/dev/null)"
    if [ "${pvc_sc%% *}" = Bound ]; then
      # Si el PVC de PostgreSQL ya existe, su clase manda (no se puede cambiar).
      STORAGE_CLASS="${pvc_sc#* }"; STORAGE_RESOLVED=1
      cp_set Storage "OK (PVC existente, sc=$STORAGE_CLASS, se conservan los datos)"
    elif [ "$STORAGE_RESOLVED" = 1 ] || test_storage "$STORAGE_CLASS"; then
      STORAGE_RESOLVED=1
      cp_set Storage "OK (${STORAGE_CLASS:-clase por defecto})"
    else
      cp_set Storage "ERROR (el PVC de prueba no quedo Bound/escribible; prueba --storage-class)"; block="$block almacenamiento"
    fi
    if [ -z "$INGRESS_CLASS" ]; then
      INGRESS_CLASS="$(k get ingressclass -o 'jsonpath={range .items[?(@.metadata.annotations.ingressclass\.kubernetes\.io/is-default-class=="true")]}{.metadata.name}{end}' 2>/dev/null)"
    fi
    cp_set Ingress "OK (${INGRESS_CLASS:-nginx, la del manifiesto})"
    if [ -z "$REGISTRY" ]; then cp_set Registry "ERROR (falta --registry, ej. docker.io/miusuario)"; block="$block registry"
    else cp_set Registry "OK ($REGISTRY, tag $TAG)"; fi
  fi
  command -v docker >/dev/null 2>&1 || { cp_set Docker "ERROR (docker no instalado)"; block="$block docker"; }
  [ -n "$block" ] && { cp_set "Kubernetes preparado" ERROR; stop_phase 2 "falta:$block"; }
  local err
  err="$(build_rendered)" || { cp_set "Kubernetes preparado" ERROR; stop_phase 2 "kustomize: $err"; }
  cp_set "Kubernetes preparado" "OK (overlay k8s/overlays/$MODO generado)"
  cp_set Bloqueos ninguno
  checkpoint 2
}

# ---------------------------------------------------------------- FASE 3

phase3() {
  cp_reset
  [ $IS_LOCAL = 0 ] && [ -z "$REGISTRY" ] && stop_phase 3 "falta --registry"
  local name file ctx ref platform=()
  # En uct los nodos son amd64; en local se usa la arquitectura de tu maquina.
  [ $IS_LOCAL = 0 ] && platform=(--platform linux/amd64)
  while IFS='|' read -r name file ctx; do
    ref="$(image_ref "$name")"
    step "docker build $ref"
    if ! docker build ${platform[@]+"${platform[@]}"} -f "$REPO_ROOT/$file" -t "$ref" "$REPO_ROOT/$ctx"; then
      cp_set "$name" ERROR; stop_phase 3 "docker build fallo ($name)"
    fi
    if [ $IS_LOCAL = 0 ]; then
      step "docker push $ref"
      docker push "$ref" || { cp_set "$name" ERROR; stop_phase 3 "docker push fallo (revisa docker login y el nombre del registry)"; }
    fi
    cp_set "$name" "OK ($ref)"
  done < <(images)
  if [ $IS_LOCAL = 1 ]; then cp_set "Imagenes disponibles para Kubernetes" "OK (en Docker local: el cluster de Docker Desktop las ve directo)"
  else cp_set "Imagenes disponibles para Kubernetes" "OK (publicadas en el registry)"; fi
  save_state
  cp_set Bloqueos ninguno
  checkpoint 3
}

# ---------------------------------------------------------------- FASE 4

phase4() {
  cp_reset
  local err
  err="$(build_rendered)" || stop_phase 4 "kustomize: $err"
  # Token de servicio (scraper y llamadas internas). Fuera de git y sin la
  # etiqueta part-of: sobrevive a la limpieza de la Fase 1.
  if k get secret soloservis-internal -o name >/dev/null 2>&1; then
    cp_set "Token interno" "OK (Secret soloservis-internal existente)"
  else
    local token
    token="$(LC_ALL=C tr -dc 'a-f0-9' </dev/urandom | head -c 64)"
    if k create secret generic soloservis-internal --from-literal=INTERNAL_API_TOKEN="$token" >/dev/null 2>&1; then
      cp_set "Token interno" "OK (Secret soloservis-internal creado)"
    else cp_set "Token interno" ERROR; fi
  fi
  step "Aplicando Secret, Services y StatefulSet de PostgreSQL"
  apply_selector app.kubernetes.io/component=database || stop_phase 4 "kubectl apply fallo"
  step "Esperando rollout de postgres (max 5 min)"
  k rollout status statefulset/postgres --timeout=300s 2>&1 | show
  cp_set PVC "$(k get pvc postgres-data-postgres-0 -o 'jsonpath={.status.phase} (sc={.spec.storageClassName}, {.status.capacity.storage})' 2>/dev/null)"
  cp_set "PostgreSQL Pod" "$(pod_state postgres-0)"
  if ! pod_ready postgres-0; then
    cp_set "PostgreSQL Ready" NO
    diagnostics pvc postgres-data-postgres-0; diagnostics pod postgres-0
    k logs postgres-0 -c postgres --tail=30 2>&1 | show
    stop_phase 4 "PostgreSQL no quedo Ready (no se continua a migraciones)"
  fi
  cp_set "PostgreSQL Ready" SI
  step "PostgreSQL $(pg_query 'SHOW server_version;')"
  checkpoint 4
}

# ---------------------------------------------------------------- FASE 5

phase5() {
  cp_reset
  local err state=timeout i succeeded failed
  err="$(build_rendered)" || stop_phase 5 "kustomize: $err"
  step "Recreando Job soloservis-migrate (los Jobs son inmutables)"
  k delete job soloservis-migrate --ignore-not-found --wait=true --timeout=120s 2>&1 | show
  apply_selector app.kubernetes.io/component=config >/dev/null
  apply_selector app.kubernetes.io/component=migrations || stop_phase 5 "kubectl apply fallo"
  step "Esperando el Job (max 5 min)"
  for i in $(seq 1 60); do
    succeeded="$(k get job soloservis-migrate -o 'jsonpath={.status.succeeded}' 2>/dev/null)"
    failed="$(k get job soloservis-migrate -o 'jsonpath={.status.conditions[?(@.type=="Failed")].status}' 2>/dev/null)"
    [ "${succeeded:-0}" -ge 1 ] && { state=complete; break; }
    [ "$failed" = True ] && { state=failed; break; }
    sleep 5
  done
  echo "${C_DIM}  Logs del Job:${C_OFF}"; k logs job/soloservis-migrate --tail=40 2>&1 | show
  if [ "$state" != complete ]; then
    cp_set "Migration Job" "ERROR ($state)"
    k get pods -l job-name=soloservis-migrate -o wide 2>&1 | show
    k get events --sort-by=.lastTimestamp 2>&1 | tail -15 | show
    stop_phase 5 "el Job de migraciones no termino correctamente"
  fi
  cp_set "Migration Job" OK
  local expected count
  expected="$(tr -d '\r' < "$REPO_ROOT/backend/database/migration_order.txt" | grep -c '\.sql$')"
  count="$(pg_query 'SELECT count(*) FROM schema_migrations;')"
  if [ "$count" != "$expected" ]; then cp_set "Migraciones aplicadas" "ERROR ($count de $expected)"; stop_phase 5 "faltan migraciones"; fi
  cp_set "Migraciones aplicadas" "OK ($count de $expected)"
  cp_set "Base de datos lista" "SI ($(pg_query 'SELECT count(*) FROM product;') productos)"
  checkpoint 5
}

# ---------------------------------------------------------------- FASE 6

phase6() {
  cp_reset
  local err a name port pod failed=""
  err="$(build_rendered)" || stop_phase 6 "kustomize: $err"
  apply_selector app.kubernetes.io/component=config >/dev/null
  apply_selector app.kubernetes.io/component=api || stop_phase 6 "kubectl apply fallo"
  for a in $APIS; do
    name="${a%%:*}"; port="${a##*:}"
    k rollout status "deployment/$name" --timeout=240s >/dev/null 2>&1
    pod="$(newest_pod "$name")"
    if [ -n "$pod" ] && pod_ready "$pod" && api_health "$name" "$port"; then
      cp_set "$name" "OK (Ready, PostgreSQL conectado, :$port)"
    else
      cp_set "$name" "ERROR ($(pod_state "$pod"))"; failed="$failed $name"
      [ -n "$pod" ] && diagnostics pod "$pod"
      k logs "deployment/$name" --tail=20 2>&1 | show
    fi
  done
  if [ -n "$failed" ]; then
    if printf '%s\n' "${CP_VALS[@]}" | grep -Eq 'ImagePull|ErrImage'; then
      if [ $IS_LOCAL = 1 ]; then stop_phase 6 "el cluster no ve las imagenes locales: en Docker Desktop usa Kubernetes con el aprovisionador kubeadm (ver k8s/README.md)"; fi
      stop_phase 6 "el cluster no puede descargar la imagen (registry privado: crear Secret docker-registry y usar --image-pull-secret)"
    fi
    stop_phase 6 "no quedaron Ready:$failed"
  fi
  cp_set Bloqueos ninguno
  checkpoint 6
}

# ---------------------------------------------------------------- FASE 7

phase7() {
  cp_reset
  local err pod h
  err="$(build_rendered)" || stop_phase 7 "kustomize: $err"
  apply_selector app.kubernetes.io/name=frontend || stop_phase 7 "kubectl apply fallo"
  k rollout status deployment/frontend --timeout=240s >/dev/null 2>&1
  pod="$(newest_pod frontend)"
  cp_set Pod "$(pod_state "$pod")"
  if [ -z "$pod" ] || ! pod_ready "$pod"; then
    [ -n "$pod" ] && diagnostics pod "$pod"
    k logs deployment/frontend --tail=30 2>&1 | show
    stop_phase 7 "frontend no quedo Ready"
  fi
  # nginx -> cada API por el DNS de su Service (lo mismo que hara el navegador).
  h="$(k exec deployment/frontend -- wget -qO- -T 5 'http://127.0.0.1:8080/products?limit=1' 2>&1)"
  case "$h" in \[*) cp_set "nginx -> APIs" "OK (/products responde a traves del frontend)" ;; *) cp_set "nginx -> APIs" "ERROR ($h)" ;; esac
  checkpoint 7
}

# ---------------------------------------------------------------- FASE 8

phase8() {
  cp_reset
  if [ $IS_LOCAL = 1 ]; then
    step "Abriendo kubectl port-forward temporal en localhost:$LOCAL_PORT"
    k port-forward svc/frontend "$LOCAL_PORT:80" >/dev/null 2>&1 &
    local pf=$! ok=0
    sleep 3
    public_routes "http://localhost:$LOCAL_PORT" || ok=1
    kill "$pf" 2>/dev/null; wait "$pf" 2>/dev/null
    [ $ok = 0 ] || stop_phase 8 "alguna ruta publica no respondio (revisa los logs del frontend y de la API)"
    checkpoint 8
    return
  fi
  local err addr="" i
  err="$(build_rendered)" || stop_phase 8 "kustomize: $err"
  apply_selector app.kubernetes.io/component=ingress || stop_phase 8 "kubectl apply fallo"
  step "Esperando direccion del Ingress (max 2 min)"
  for i in $(seq 1 24); do
    addr="$(k get ingress soloservis -o 'jsonpath={.status.loadBalancer.ingress[0].ip}{.status.loadBalancer.ingress[0].hostname}' 2>/dev/null)"
    [ -n "$addr" ] && break
    sleep 5
  done
  k get ingress soloservis 2>&1 | show
  if [ -n "$addr" ]; then cp_set Ingress "OK (host=$INGRESS_HOST, address=$addr)"; else cp_set Ingress "ERROR (sin address)"; fi
  if ! public_routes "$(frontend_url)"; then
    echo "${C_YELLOW}  Nota: el DNS de *.dev.censei.cl puede tardar unos minutos la primera vez; si sigue, prueba desde la red de la universidad.${C_OFF}"
  fi
  checkpoint 8
}

# ---------------------------------------------------------------- FASE 9

phase9() {
  cp_reset
  local what first old new before after a apis_ok=1
  for what in pods pvc deployments statefulsets services ingress; do
    echo "${C_DIM}  kubectl get $what${C_OFF}"; k get "$what" -l app.kubernetes.io/part-of=soloservis 2>&1 | show
  done
  # Recreacion de un Pod administrado por un Deployment.
  first="${APIS%% *}"; first="${first%%:*}"
  step "Borrando el pod de $first: Kubernetes debe recrearlo"
  old="$(newest_pod "$first")"
  [ -n "$old" ] && k delete pod "$old" --wait=true --timeout=120s 2>&1 | show
  k rollout status "deployment/$first" --timeout=180s >/dev/null 2>&1
  new="$(newest_pod "$first")"
  if [ -n "$new" ] && [ "$new" != "$old" ] && pod_ready "$new"; then cp_set Kubernetes "FUNCIONAL (recreo el pod de $first)"
  else cp_set Kubernetes "ERROR (no se verifico la recreacion)"; fi
  # Persistencia: reiniciar postgres-0 sin tocar el PVC y comparar datos.
  step "Reiniciando postgres-0 (el PVC NO se toca) para verificar persistencia"
  before="$(pg_query 'SELECT count(*) FROM schema_migrations;')"
  k delete pod postgres-0 --wait=true --timeout=120s 2>&1 | show
  k rollout status statefulset/postgres --timeout=300s >/dev/null 2>&1
  k wait --for=condition=Ready pod/postgres-0 --timeout=180s >/dev/null 2>&1
  after="$(pg_query 'SELECT count(*) FROM schema_migrations;')"
  if [ -n "$before" ] && [ "$before" = "$after" ]; then cp_set Persistencia "FUNCIONAL (mismos datos tras reiniciar: $after migraciones)"
  else cp_set Persistencia "ERROR (antes=$before despues=$after)"; fi
  # Tras reiniciar la BD, las APIs deben volver a conectarse solas.
  sleep 10
  for a in $APIS; do api_health "${a%%:*}" "${a##*:}" || { apis_ok=0; cp_set "${a%%:*}" "ERROR (no reconecto a PostgreSQL)"; }; done
  if [ $apis_ok = 1 ]; then cp_set APIs "FUNCIONAL ($(echo $APIS | wc -w | tr -d ' ') Deployment(s), reconectan a PostgreSQL)"; else cp_set APIs ERROR; fi
  checkpoint 9
  echo "${C_GREEN}Para abrir la app:${C_OFF}"
  if [ $IS_LOCAL = 1 ]; then
    echo "${C_GREEN}  kubectl --context $CONTEXT -n $NAMESPACE port-forward svc/frontend $LOCAL_PORT:80${C_OFF}"
    echo "${C_GREEN}  y abre http://localhost:$LOCAL_PORT (deja esa terminal abierta)${C_OFF}"
  else
    echo "${C_GREEN}  $(frontend_url)${C_OFF}"
  fi
}

# ---------------------------------------------------------------- main

STORAGE_RESOLVED=0
if [ -f "$STATE_FILE" ]; then
  # shellcheck disable=SC1090
  . "$STATE_FILE"
  [ -z "$REGISTRY" ] && REGISTRY="${PREV_REGISTRY:-}"
  [ -z "$TAG" ] && [ "$FROM_PHASE" -gt 3 ] && TAG="${PREV_TAG:-}"
  if [ -z "$STORAGE_CLASS" ] && [ "$FROM_PHASE" -gt 2 ] && [ "${PREV_STORAGE_RESOLVED:-0}" = 1 ]; then STORAGE_CLASS="${PREV_STORAGE_CLASS:-}"; STORAGE_RESOLVED=1; fi
  [ -z "$INGRESS_CLASS" ] && [ "$FROM_PHASE" -gt 2 ] && INGRESS_CLASS="${PREV_INGRESS_CLASS:-}"
  [ -z "$INGRESS_HOST" ] && INGRESS_HOST="${PREV_INGRESS_HOST:-}"
  [ -z "$IMAGE_PULL_SECRET" ] && IMAGE_PULL_SECRET="${PREV_IMAGE_PULL_SECRET:-}"
fi
# Dominio publico segun el manual: <proyecto>-<usuario>.dev.censei.cl
[ $IS_LOCAL = 0 ] && [ -z "$INGRESS_HOST" ] && INGRESS_HOST="soloservis-$STUDENT_USER.dev.censei.cl"
if [ -n "$REGISTRY" ] && ! echo "$REGISTRY" | grep -Eq '^[a-z0-9.:-]+(/[a-z0-9._-]+)+$'; then
  echo "${C_RED}ERROR: --registry '$REGISTRY' no es valido. Usa tu usuario real en minusculas, ej: docker.io/miusuario${C_OFF}"; exit 1
fi
if [ -z "$TAG" ]; then
  [ "$FROM_PHASE" -gt 3 ] && echo "${C_YELLOW}AVISO: no hay tag guardado de la Fase 3; se genera uno nuevo (las imagenes deben existir con ese tag). Usa --tag si ya construiste otro.${C_OFF}"
  TAG="$(new_tag)"
fi

echo "${C_CYAN}SOLOServis -> Kubernetes | modo=$MODO topologia=$TOPOLOGIA contexto=${CONTEXT:-(el del kubeconfig)} namespace=$NAMESPACE fases=$FROM_PHASE..$TO_PHASE${C_OFF}"
if [ $IS_LOCAL = 1 ]; then echo "${C_CYAN}Kubeconfig: $(basename "$KUBECONFIG_FILE") | tag=$TAG${C_OFF}"
else echo "${C_CYAN}Kubeconfig: $(basename "$KUBECONFIG_FILE") | tag=$TAG registry=${REGISTRY:-(sin definir)}${C_OFF}"; fi

for n in $(seq "$FROM_PHASE" "$TO_PHASE"); do
  echo; echo "---------------- FASE $n ----------------"
  "phase$n"
done
echo "${C_GREEN}Listo.${C_OFF}"
