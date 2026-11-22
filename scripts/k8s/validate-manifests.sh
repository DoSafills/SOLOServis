#!/usr/bin/env bash
# Valida los manifiestos de Kubernetes contra el esquema de la API (TAL-183).
#
# `kubectl kustomize` solo arma el YAML: no detecta un campo mal indentado o
# con otro nombre. Asi paso un `spec:` fuera de lugar en el Deployment del
# scraper, que recien fallo en el `kubectl apply` del despliegue. Este script
# genera las dos topologias (dominios y consolidado), con las NetworkPolicy de
# components/red, y los Jobs de pruebas, y los pasa por kubeconform en modo
# estricto, que rechaza campos desconocidos.
#
#   scripts/k8s/validate-manifests.sh
#
# Usa `kubeconform` si esta en el PATH; si no, la imagen de Docker. No necesita
# acceso al cluster. Compatible con bash 3.2.
set -euo pipefail

KUBECONFORM_VERSION="v0.6.7"
KUBERNETES_VERSION="${KUBERNETES_VERSION:-1.36.0}"

ROOT="$(cd "$(dirname "$0")/../.." && pwd)"
TMP="$ROOT/k8s/.validate-tmp"
trap 'rm -rf "$TMP"' EXIT

conform() {
  if command -v kubeconform >/dev/null 2>&1; then
    kubeconform -strict -summary -kubernetes-version "$KUBERNETES_VERSION" -
  else
    docker run --rm -i "ghcr.io/yannh/kubeconform:$KUBECONFORM_VERSION" \
      -strict -summary -kubernetes-version "$KUBERNETES_VERSION" -
  fi
}

failed=0
check() {
  # $1 = nombre para el reporte, $2 = directorio con kustomization.yaml
  echo "== $1"
  if ! kubectl kustomize "$2" > "$TMP/rendered.yaml"; then
    echo "ERROR: kubectl kustomize fallo en $1"
    failed=1
    return 0
  fi
  if ! conform < "$TMP/rendered.yaml"; then
    failed=1
  fi
}

for topologia in dominios consolidado; do
  rm -rf "$TMP"
  mkdir -p "$TMP/overlay"
  cat > "$TMP/overlay/kustomization.yaml" <<EOF
apiVersion: kustomize.config.k8s.io/v1beta1
kind: Kustomization
resources:
  - ../../base
components:
  - ../../components/$topologia
  - ../../components/red
EOF
  check "base + $topologia + red" "$TMP/overlay"
done

mkdir -p "$TMP"
check "tests (pod de pruebas)" "$ROOT/k8s/tests"
check "tests (visor de llamadas)" "$ROOT/k8s/tests/llamadas"

if [ "$failed" -ne 0 ]; then
  echo "FALLA: hay manifiestos que la API de Kubernetes rechazaria."
  exit 1
fi
echo "OK: todos los manifiestos son validos."
