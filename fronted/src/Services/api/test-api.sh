#!/usr/bin/env bash
# Prueba end-to-end de la API de productos.
# Uso: ./test-api.sh [URL_BASE]
# Por defecto usa http://localhost:4000/api

BASE="${1:-http://localhost:4000/api}"
PASS=0
FAIL=0

check() {
  local desc="$1"
  local url="$2"
  local expected_status="$3"

  status=$(curl -s -o /tmp/api_test_body.json -w "%{http_code}" "$url")

  if [ "$status" = "$expected_status" ]; then
    echo "✅ $desc (HTTP $status)"
    PASS=$((PASS+1))
  else
    echo "❌ $desc -> esperaba $expected_status, obtuve $status"
    echo "   Respuesta: $(cat /tmp/api_test_body.json)"
    FAIL=$((FAIL+1))
  fi
}

check_field() {
  local desc="$1"
  local url="$2"
  local jq_filter="$3"
  local expected="$4"

  value=$(curl -s "$url" | jq -r "$jq_filter" 2>/dev/null)

  if [ "$value" = "$expected" ]; then
    echo "✅ $desc"
    PASS=$((PASS+1))
  else
    echo "❌ $desc -> esperaba '$expected', obtuve '$value'"
    FAIL=$((FAIL+1))
  fi
}

echo "== Probando API en $BASE =="
echo

check "Health check"                         "$BASE/health"                    200
check "Lista de productos"                   "$BASE/products"                  200
check "Producto existente"                   "$BASE/products/rtx-4060"         200
check "Producto inexistente -> 404"          "$BASE/products/no-existe"        404
check "Lista de tiendas"                     "$BASE/stores"                    200
check "Tienda existente"                     "$BASE/stores/techzone"           200
check "Lista de servicios"                   "$BASE/services"                  200
check "Servicio existente"                   "$BASE/services/netflix-premium"  200
check "Filtro por categoría"                 "$BASE/products?category=Celulares" 200
check "Filtro por búsqueda"                  "$BASE/products?search=iphone"    200

echo
echo "== Validando contenido de las respuestas (requiere jq) =="
echo

if command -v jq >/dev/null 2>&1; then
  check_field "rtx-4060 tiene nombre correcto"       "$BASE/products/rtx-4060" ".name" "ASUS RTX 4060 Dual 8GB"
  check_field "rtx-4060 tiene 4 ofertas"              "$BASE/products/rtx-4060" ".offers | length" "4"
  check_field "techzone tiene nombre correcto"        "$BASE/stores/techzone"   ".name" "TechZone"
  check_field "netflix-premium tiene 4 beneficios"    "$BASE/services/netflix-premium" ".benefits | length" "4"
else
  echo "⚠️  jq no está instalado, se omiten las validaciones de contenido."
fi

echo
echo "== Resultado: $PASS OK / $FAIL fallidos =="

[ "$FAIL" -eq 0 ]
