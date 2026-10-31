#!/bin/sh
# Pod de test (TAL-103): verifica el despliegue desde dentro del clúster.
# Solo hace lecturas: no escribe nada en la base.
#   - /health de cada pod de API (con PostgreSQL conectado)
#   - una lectura por cada una de las 14 APIs
#   - rutas protegidas sin token -> 401; ruta interna con token -> 200
#   - aislamiento: cada dominio responde 404 a las rutas de otro
#   - el nginx del frontend envía cada ruta al pod correcto
# Sale con código 1 si alguna verificación falla.
set -u

FRONTEND=http://frontend:80
TOKEN=${INTERNAL_API_TOKEN:-}
pass=0
fail=0

# check <nombre> <código esperado> <url> [texto esperado en la respuesta] [token]
check() {
  name=$1 want=$2 url=$3 expect=${4:-} auth=${5:-}
  if [ -n "$auth" ]; then
    out=$(curl -s -m 10 -H "Authorization: Bearer $auth" -w '\n%{http_code}' "$url")
  else
    out=$(curl -s -m 10 -w '\n%{http_code}' "$url")
  fi
  code=$(printf '%s' "$out" | tail -n 1)
  body=$(printf '%s' "$out" | sed '$d')
  if [ "$code" = "$want" ] && { [ -z "$expect" ] || printf '%s' "$body" | grep -q -- "$expect"; }; then
    pass=$((pass + 1))
    printf 'OK    %-34s %s  %s\n' "$name" "$code" "$url"
  else
    fail=$((fail + 1))
    printf 'FALLA %-34s %s (quiero %s)  %s\n' "$name" "$code" "$want" "$url"
    printf '      %.200s\n' "$body"
  fi
}

# Topología: dominios (3 Services) o consolidado (un Service backend).
if [ "$(curl -s -m 5 -o /dev/null -w '%{http_code}' http://comercial:8080/health)" = "000" ]; then
  TOPOLOGIA=consolidado
  COMERCIAL=http://backend:8080 USUARIOS=http://backend:8080 INGESTION=http://backend:8080
else
  TOPOLOGIA=dominios
  COMERCIAL=http://comercial:8080 USUARIOS=http://usuarios:8080 INGESTION=http://ingestion:8080
fi
echo "Topología detectada: $TOPOLOGIA"

echo "== Health (ping a PostgreSQL)"
check "comercial /health" 200 "$COMERCIAL/health" '"database":"connected"'
if [ "$TOPOLOGIA" = dominios ]; then
  check "usuarios /health" 200 "$USUARIOS/health" '"database":"connected"'
  check "ingestion /health" 200 "$INGESTION/health" '"database":"connected"'
fi

# Un producto real para las lecturas que necesitan id.
PID=$(curl -s -m 10 "$COMERCIAL/products?limit=1" | sed -n 's/^\[{"id":"\([^"]*\)".*/\1/p')
[ -n "$PID" ] || echo "AVISO: no se pudo leer un producto; las lecturas con id van a fallar"

# Usuario que no existe: watchlist y notification responden una lista vacía.
NOUSER=2147483647

echo "== Una lectura por API"
check "product   GET /products" 200 "$COMERCIAL/products?limit=1" '"id"'
check "store     GET /stores" 200 "$COMERCIAL/stores" '"id"'
check "service   GET /services" 200 "$COMERCIAL/services" '"id"'
check "pricing   GET /offers/product/{id}" 200 "$COMERCIAL/offers/product/$PID" '"price"'
check "search    GET /search/products" 200 "$COMERCIAL/search/products?q=a&limit=1" '"id"'
check "comparison GET /comparisons/compatible" 200 "$COMERCIAL/comparisons/compatible/$PID"
check "review    GET /reviews/product/{id}" 200 "$COMERCIAL/reviews/product/$PID"
check "watchlist GET /watchlist" 200 "$COMERCIAL/watchlist?userId=$NOUSER" '^\[\]'
check "notification GET /notifications" 200 "$COMERCIAL/notifications?userId=$NOUSER" '^\[\]'
check "user      GET /users" 200 "$USUARIOS/users" 'user-api-ready'

echo "== Rutas protegidas sin token -> 401"
check "favorites GET /favorites" 401 "$COMERCIAL/favorites"
check "cart      GET /cart" 401 "$COMERCIAL/cart"
check "comparison GET /comparisons/products" 401 "$COMERCIAL/comparisons/products"
check "auth      GET /auth/validate" 401 "$USUARIOS/auth/validate"
check "ingestion GET /ingestion/runs" 401 "$INGESTION/ingestion/runs"

echo "== Ruta interna con INTERNAL_API_TOKEN -> 200"
if [ -n "$TOKEN" ]; then
  check "ingestion GET /ingestion/runs" 200 "$INGESTION/ingestion/runs" '' "$TOKEN"
else
  echo "OMITIDA: el Secret soloservis-internal no existe"
fi

if [ "$TOPOLOGIA" = dominios ]; then
  echo "== Aislamiento entre dominios -> 404"
  check "comercial no sirve /auth" 404 "$COMERCIAL/auth/validate"
  check "comercial no sirve /ingestion" 404 "$COMERCIAL/ingestion/runs"
  check "usuarios no sirve /products" 404 "$USUARIOS/products"
  check "ingestion no sirve /products" 404 "$INGESTION/products"
fi

echo "== nginx del frontend -> pod de cada dominio"
check "frontend /" 200 "$FRONTEND/" 'id="root"'
check "frontend /products -> comercial" 200 "$FRONTEND/products?limit=1" '"id"'
check "frontend /auth -> usuarios" 401 "$FRONTEND/auth/validate"

echo "Resultado: $pass OK, $fail fallas"
[ "$fail" -eq 0 ]
