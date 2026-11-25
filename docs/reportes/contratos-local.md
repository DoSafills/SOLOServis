# Contratos HTTP del gateway

- Gateway: `http://localhost:8443`
- Resultado: 24 ok, 0 fallidas, 6 avisos

| | Status | Pedido | Comprobación | Detalle |
| --- | --- | --- | --- | --- |
| ✅ | 200 | `GET /healthz` | El gateway responde su probe |  |
| ✅ | 200 | `GET /` | La SPA se sirve en la raíz |  |
| ✅ | 200 | `GET /catalogo` | Una ruta del frontend devuelve la SPA |  |
| ✅ | 200 | `GET /products?limit=2` | GET /products devuelve un arreglo |  |
| ✅ | 200 | `GET /products?limit=2` | GET /products trae X-Total-Count |  |
| ✅ | 400 | `GET /products/no-es-uuid` | GET /products con id no UUID es 400 |  |
| ✅ | 404 | `GET /products/00000000-0000-0000-0000-000000000000` | GET /products con UUID ausente es 404 |  |
| ✅ | 400 | `GET /products?limit=0` | GET /products con limit inválido es 400 |  |
| ✅ | 200 | `GET /product-categories` | GET /product-categories responde |  |
| ✅ | 200 | `GET /categories` | GET /categories devuelve items y total |  |
| ✅ | 400 | `GET /categories/NO_VALIDO` | GET /categories con slug inválido es 400 |  |
| ✅ | 200 | `GET /stores` | GET /stores responde |  |
| ✅ | 200 | `GET /services` | GET /services responde |  |
| ✅ | 200 | `GET /search/products?q=tv&limit=2` | GET /search/products responde |  |
| ✅ | 405 | `DELETE /search/products` | DELETE /search/products es 405 con Allow |  |
| ✅ | 401 | `GET /auth/validate` | GET /auth/validate sin token es 401 |  |
| ✅ | 400 | `POST /auth/login` | POST /auth/login con JSON malformado es 400 |  |
| ✅ | 400 | `POST /auth/login` | POST /auth/login con campo desconocido es 400 |  |
| ✅ | 422 | `POST /auth/login` | POST /auth/login sin datos es 422 |  |
| ✅ | 401 | `GET /cart` | GET /cart sin sesión es 401 |  |
| ✅ | 401 | `GET /favorites` | GET /favorites sin sesión es 401 |  |
| ✅ | 401 | `GET /watchlist` | GET /watchlist sin sesión es 401 |  |
| ✅ | 200 | `GET /reviews/product/1` | GET /reviews/product/1 es público |  |
| ✅ | 204 | `OPTIONS /cart/items` | OPTIONS /cart/items responde 204 |  |
| ⚠️ | 403 | `POST /products` | POST /products sin token debería ser problem+json | Content-Type text/html, quiero application/problem+json; el cuerpo no es JSON: '<html>\r\n<head><title>403 Forbidden</title></head>\r\n<body>\r\n<center><h1>403 Forbi' |
| ⚠️ | 200 | `GET /health` | GET /health por el gateway debería ser 404 problem+json | status 200, quiero 404; Content-Type text/html, quiero application/problem+json; el cuerpo no es JSON: '<!doctype html>\n<html lang="en">\n  <head>\n    \n    <meta charset="UTF-8" />\n    ' |
| ⚠️ | 200 | `GET /offers/product/1` | GET /offers no se publica (404 problem+json) | status 200, quiero 404; Content-Type text/html, quiero application/problem+json; el cuerpo no es JSON: '<!doctype html>\n<html lang="en">\n  <head>\n    \n    <meta charset="UTF-8" />\n    ' |
| ⚠️ | 200 | `GET /users` | GET /users no se publica (404 problem+json) | status 200, quiero 404; Content-Type text/html, quiero application/problem+json; el cuerpo no es JSON: '<!doctype html>\n<html lang="en">\n  <head>\n    \n    <meta charset="UTF-8" />\n    ' |
| ⚠️ | 200 | `GET /ingestion/runs` | GET /ingestion/runs no se publica (404 problem+json) | status 200, quiero 404; Content-Type text/html, quiero application/problem+json; el cuerpo no es JSON: '<!doctype html>\n<html lang="en">\n  <head>\n    \n    <meta charset="UTF-8" />\n    ' |
| ⚠️ | 200 | `GET /notifications` | GET /notifications no se publica (404 problem+json) | status 200, quiero 404; Content-Type text/html, quiero application/problem+json; el cuerpo no es JSON: '<!doctype html>\n<html lang="en">\n  <head>\n    \n    <meta charset="UTF-8" />\n    ' |
