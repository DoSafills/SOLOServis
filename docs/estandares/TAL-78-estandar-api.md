# TAL-78 — Estándar de API: validación, errores HTTP, logs y tests

| Campo | Valor |
| --- | --- |
| Tarea | TAL-78 (parte 1): estándar + infraestructura reutilizable |
| Alcance | Product API (`cmd/product-api`) y Store API (`cmd/store-api`) |
| Implementación de referencia | `backend/internal/httpx` |
| Estado previo | Ver auditoría TAL-69 (`docs/auditoria/TAL-69-auditoria-product-store-api.md`, rama `feature/TAL-69-auditoria-product-store-api`) |

Este documento es **normativo** para las tareas TAL-70 a TAL-77: todo endpoint nuevo o modificado debe cumplirlo. Los handlers existentes **no** se migraron en esta parte; cada tarea adopta el estándar en los endpoints que toca.

---

## 1. Formato de error único

### Decisión: RFC 9457 (`application/problem+json`)

Se adopta *Problem Details for HTTP APIs* (RFC 9457) con miembros de extensión.

Justificación:
- Es un estándar IETF: clientes, proxies y herramientas (OpenAPI, Postman) lo reconocen; no inventamos un envelope propio.
- El `Content-Type: application/problem+json` distingue un error de un payload exitoso sin inspeccionar el body.
- No requiere dependencias: es un objeto JSON plano serializable con `encoding/json`.
- Admite extensiones, que usamos para el código estable, el `request_id` y los errores por campo.

### Estructura

```json
{
  "type": "about:blank",
  "title": "Unprocessable Entity",
  "status": 422,
  "detail": "La solicitud no cumple las reglas de validación.",
  "instance": "/products",
  "code": "validation_failed",
  "request_id": "3f9c2a7e5b0d4c1a8e6f2b9d0a7c4e1f",
  "errors": [
    { "field": "name", "code": "required", "message": "name es obligatorio" },
    { "field": "categoryId", "code": "min", "message": "categoryId debe ser mayor que 0" }
  ]
}
```

| Miembro | Obligatorio | Descripción |
| --- | --- | --- |
| `type` | sí | Siempre `about:blank` por ahora. RFC 9457 indica que entonces `title` es el texto estándar del status. Si en el futuro se publican páginas de documentación por problema, se reemplaza por su URI. |
| `title` | sí | `http.StatusText(status)`. No cambia entre ocurrencias. |
| `status` | sí | Código HTTP (repetido en el body por conveniencia del cliente). |
| `detail` | sí | Mensaje legible para humanos, en español. **Nunca** incluye detalles internos (SQL, stack, nombres de tablas). |
| `instance` | sí | Path de la petición (sin query string). |
| `code` | sí | **Código interno estable** en `snake_case`. Es lo que el frontend debe usar para decidir qué hacer; `detail` puede cambiar de redacción. |
| `request_id` | sí | El mismo valor del header `X-Request-ID` de la respuesta. |
| `errors` | solo en validación/decodificación por campo | Lista de `{field, code, message}`. `field` usa el nombre JSON del campo (camelCase, notación con punto para anidados: `offers.0.price`). |

> Nota de nomenclatura: los payloads de negocio usan camelCase (`reviewCount`); el miembro `request_id` se mantiene en snake_case por decisión del equipo (TAL-78), alineado con el nombre del campo en los logs.

### Catálogo de códigos internos

| `code` | Status | Cuándo |
| --- | --- | --- |
| `malformed_json` | 400 | Body vacío, JSON sintácticamente inválido o datos extra después del objeto JSON. |
| `unknown_field` | 400 | El JSON trae un campo que el DTO no declara. |
| `invalid_type` | 400 | Un campo trae un tipo JSON incompatible con el DTO (p. ej. `"name": 5`). |
| `invalid_parameter` | 400 | Parámetro de ruta o query con formato inválido (p. ej. UUID mal formado). |
| `unauthorized` | 401 | Falta el token o no corresponde a una sesión vigente (TAL-70). Incluye `WWW-Authenticate: Bearer`. |
| `forbidden` | 403 | Token válido sin el rol requerido (TAL-70: escrituras de productos exigen `admin`). |
| `not_found` | 404 | El recurso no existe (o está desactivado y el endpoint no expone inactivos). |
| `conflict` | 409 | Violación de unicidad o estado incompatible (p. ej. SKU duplicado). |
| `body_too_large` | 413 | El body supera el límite configurado. |
| `unsupported_media_type` | 415 | `Content-Type` ausente o distinto de `application/json`. |
| `validation_failed` | 422 | JSON decodificado correctamente que no cumple reglas de negocio. Incluye `errors`. |
| `internal_error` | 500 | Cualquier error no previsto. Mensaje genérico + `request_id`. |

Los códigos por campo (`errors[].code`) iniciales son: `required`, `min`, `max`, `max_length`, `format`, `one_of`, `unknown_field`, `invalid_type`, `not_found` (referencia a otro recurso inexistente, p. ej. `categoryId`). Se pueden agregar nuevos; nunca se renombran los existentes.

---

## 2. Códigos HTTP

| Código | Uso | Body |
| --- | --- | --- |
| **200 OK** | Lectura exitosa (GET) y actualización exitosa (PUT/PATCH) que devuelve el recurso. | Recurso JSON. |
| **201 Created** | Creación exitosa (POST). Incluir header `Location` con la URL del recurso. | Recurso creado, **con la misma forma que el GET** del recurso. |
| **204 No Content** | Operación exitosa sin body: DELETE (incluido soft-delete) y acciones sin resultado. | Vacío. |
| **400 Bad Request** | La petición **no se puede interpretar**: JSON malformado, body vacío, campo desconocido, tipo incorrecto, parámetro de ruta/query mal formado. | Problem (`malformed_json`, `unknown_field`, `invalid_type`, `invalid_parameter`). |
| **401 Unauthorized** | Endpoint protegido sin token válido (`internal/authz`). | Problem `unauthorized`. |
| **403 Forbidden** | Token válido sin el rol requerido. | Problem `forbidden`. |
| **404 Not Found** | El recurso direccionado no existe. **Solo** cuando el repositorio indica "no encontrado" (`pgx.ErrNoRows`), nunca como comodín para cualquier error de BD. | Problem `not_found`. |
| **409 Conflict** | Violación de restricción UNIQUE (SQLSTATE `23505`) o conflicto de estado. | Problem `conflict`. |
| **413 Content Too Large** | Body mayor al límite (1 MB por defecto). | Problem `body_too_large`. |
| **415 Unsupported Media Type** | Endpoint con body y `Content-Type` distinto de `application/json` (se aceptan parámetros como `charset=utf-8`). | Problem `unsupported_media_type`. |
| **422 Unprocessable Content** | El JSON **es válido y se decodificó**, pero no cumple las reglas: obligatorios, rangos, largos, formatos, referencias inexistentes (FK, SQLSTATE `23503`). | Problem `validation_failed` con `errors`. |
| **500 Internal Server Error** | Error no previsto (BD caída, bug, panic). El detalle va al log. | Problem `internal_error` con mensaje genérico. |

### Regla 400 vs 422 (mecánica)

- **Si falla la decodificación al DTO → 400.** Lo produce exclusivamente `httpx.DecodeJSON` (junto con 413 y 415).
- **Si la decodificación funciona y falla la validación → 422.** Lo produce exclusivamente el validador del handler (`httpx.Validator` + `httpx.WriteValidation`).

Ejemplos:

| Body | Resultado |
| --- | --- |
| `{"name": "TV"` | 400 `malformed_json` |
| `{"name": 5}` | 400 `invalid_type` (field `name`) |
| `{"nombre": "TV"}` | 400 `unknown_field` (field `nombre`) |
| `{"name": ""}` | 422 `validation_failed` (`name` → `required`) |
| `{"name": "TV", "categoryId": 999}` (categoría inexistente) | 422 `validation_failed` (`categoryId` → `not_found`) |

---

## 3. Validación

1. **Dónde:** en el **borde del handler**, antes de llamar al repositorio. Orden fijo:
   1. parámetros de ruta/query → 400 `invalid_parameter`;
   2. `httpx.DecodeJSON` → 400/413/415;
   3. normalización (p. ej. `strings.TrimSpace`);
   4. reglas con `httpx.Validator` → 422;
   5. llamada al repositorio; mapeo de errores de BD (§3.5).
2. **Campos desconocidos:** siempre rechazados (`DisallowUnknownFields`) → 400 `unknown_field`.
3. **Tamaño del body:** límite por defecto `httpx.DefaultMaxBodyBytes` = 1 MB (1 048 576 bytes), configurable por endpoint con el argumento `maxBytes` de `DecodeJSON`. Excederlo → 413.
4. **Content-Type:** todo endpoint con body exige `application/json` (con o sin parámetros). Ausente o distinto → 415.
5. **Errores de BD conocidos:** el repositorio o el handler traducen
   - `pgx.ErrNoRows` → 404 `not_found`;
   - SQLSTATE `23505` (unique) → 409 `conflict`;
   - SQLSTATE `23503` (FK) → 422 con el campo afectado;
   - cualquier otro → `httpx.WriteInternal` (500).
6. **Datos extra:** un body con más de un valor JSON (`{}{}`) es 400 `malformed_json`.

Uso de referencia:

```go
var req dto.CreateProductRequest
if err := httpx.DecodeJSON(w, r, &req, httpx.DefaultMaxBodyBytes); err != nil {
	httpx.WriteDecodeError(w, r, err)
	return
}
req.Name = strings.TrimSpace(req.Name)

var v httpx.Validator
v.Required("name", req.Name)
v.Check(req.CategoryID > 0, "categoryId", "min", "categoryId debe ser mayor que 0")
if !v.Valid() {
	httpx.WriteValidation(w, r, v.Errors())
	return
}
```

---

## 4. Logging

1. **Formato:** JSON estructurado con `log/slog` (`slog.NewJSONHandler`) a **stdout** (Kubernetes recoge stdout). Nivel configurable con la variable `LOG_LEVEL` (`debug`, `info`, `warn`, `error`; por defecto `info`).
2. **Log de acceso** (una línea por request, middleware `httpx.AccessLog`) con los campos:

   | Campo | Ejemplo |
   | --- | --- |
   | `time` | `2026-10-18T14:03:12.123Z` |
   | `level` | `INFO` (2xx/3xx), `WARN` (4xx), `ERROR` (5xx) |
   | `msg` | `http_request` |
   | `request_id` | `3f9c2a7e…` |
   | `method` | `GET` |
   | `path` | `/products/…` (sin query string) |
   | `status` | `200` |
   | `duration_ms` | `12.4` |
   | `bytes` | `5321` |

3. **Qué NO se loguea nunca:**
   - bodies completos de request o response;
   - credenciales, contraseñas, hashes de contraseña;
   - tokens (header `Authorization`, cookies, refresh/access tokens);
   - cadenas de conexión (`DATABASE_URL`) ni variables de entorno con secretos;
   - query strings completos (pueden contener tokens o datos personales); solo el path.
4. **Errores 5xx:** el handler llama a `httpx.WriteInternal(w, r, logger, err)`: el error real (`err.Error()`) se registra con nivel `ERROR` junto al `request_id`; el cliente recibe solo `internal_error` con un mensaje genérico y su `request_id`, que le permite reportarlo y a nosotros encontrar la línea de log.
5. **Panics:** el middleware `httpx.Recover` responde 500 genérico y registra el valor del panic y el stack trace (`runtime/debug.Stack`).
6. **Correlación:** `request_id` se acepta del cliente en `X-Request-ID` solo si cumple `^[A-Za-z0-9-]{1,64}$`; si no, se genera uno nuevo (128 bits aleatorios, hex). Siempre se devuelve en el header `X-Request-ID` de la respuesta. Las llamadas entre servicios deben propagarlo.

---

## 5. Tests

### Casos mínimos obligatorios por endpoint

| Caso | Qué verifica |
| --- | --- |
| Feliz | Status 2xx correcto, `Content-Type`, forma del body. |
| Entrada inválida | 400 (decodificación/parámetros) **y** 422 (reglas) si el endpoint recibe body; `code` y `errors[].field` esperados. |
| Recurso inexistente | 404 `not_found` cuando el repositorio devuelve "no encontrado". |
| Error interno simulado | El repositorio falso devuelve un error → 500 `internal_error`, el body **no** contiene el mensaje del error y sí el `request_id`. |

Para poder simular el error interno, **los handlers deben depender de una interfaz** de repositorio (definida en el paquete del handler) y no del `*Repository` concreto. Los tests usan un *fake* en memoria; no requieren PostgreSQL.

### Convención

- Archivo `<archivo>_test.go` junto al código, mismo paquete (o `_test` si se prueba solo la API pública).
- Nombre: `Test<Unidad>_<Escenario>` para casos únicos (p. ej. `TestRecover_NoExponeDetalles`) y **table-driven** con subtests `t.Run(tc.name, …)` cuando varían solo las entradas (p. ej. `TestDecodeJSON` con casos "body excedido", "content-type incorrecto"…).
- HTTP con `net/http/httptest` (`httptest.NewRequest` + `httptest.NewRecorder`).
- Sin dependencias de test externas (`testing` estándar).
- Ejecución: `cd backend && go test ./...` (con `-race` en CI si está disponible).

Estructura de referencia:

```go
func TestCreateProduct(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		repo       fakeRepo
		wantStatus int
		wantCode   string
	}{
		{"caso feliz", `{"name":"TV","categoryId":1,"imageUrl":"https://x/y.png"}`, fakeRepo{}, 201, ""},
		{"json malformado", `{"name":`, fakeRepo{}, 400, "malformed_json"},
		{"regla incumplida", `{"name":"","categoryId":1,"imageUrl":"https://x/y.png"}`, fakeRepo{}, 422, "validation_failed"},
		{"error interno", `{"name":"TV","categoryId":1,"imageUrl":"https://x/y.png"}`, fakeRepo{err: errBoom}, 500, "internal_error"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) { /* … */ })
	}
}
```

---

## 6. Definition of Done (TAL-70 a TAL-77)

Cada endpoint nuevo o modificado:

- [ ] Errores con `httpx.WriteProblem` / `WriteValidation` / `WriteDecodeError` / `WriteInternal`; ningún `http.Error` ni `apiutil.WriteError` nuevo.
- [ ] Body decodificado con `httpx.DecodeJSON` (campos desconocidos rechazados, límite de tamaño, Content-Type verificado).
- [ ] Validación en el borde del handler con `httpx.Validator`; 422 con `errors[]` por campo.
- [ ] Errores de BD mapeados: `ErrNoRows`→404, `23505`→409, `23503`→422, resto→500. Prohibido usar 404 o 500 como comodín.
- [ ] Códigos de éxito según §2 (201 + `Location` en POST, 204 en DELETE); respuesta de POST/PUT con la misma forma que el GET.
- [ ] Ningún detalle interno en respuestas; errores 5xx registrados con `request_id`.
- [ ] Nada de §4.3 en logs.
- [ ] Handler depende de una interfaz de repositorio.
- [ ] Tests con los 4 casos mínimos (§5); `go test ./...` y `go vet ./...` en verde.
- [ ] Endpoints y códigos documentados en el README del servicio (y README desactualizado corregido si aplica).
- [ ] Endpoints de escritura con autenticación/autorización (cuando el mecanismo esté definido; ver preguntas abiertas TAL-69).

---

## 7. Infraestructura implementada (`backend/internal/httpx`)

| Archivo | Contenido |
| --- | --- |
| `problem.go` | `Problem`, `FieldError`, constantes de códigos, `WriteProblem`, `WriteValidation`, `WriteInternal`. |
| `validate.go` | `Validator` (acumula `FieldError`): `Required`, `MaxLength`, `Check`, `Add`, `Valid`, `Errors`. |
| `decode.go` | `DecodeJSON`, `DecodeError`, `WriteDecodeError`, `DefaultMaxBodyBytes`. |
| `requestid.go` | Middleware `RequestID`, `RequestIDFrom(ctx)`, header `X-Request-ID`. |
| `logging.go` | `NewLogger`, `ParseLevel`, middleware `AccessLog`. |
| `recover.go` | Middleware `Recover`. |

Cableado (`backend/internal/http/router.go`): en `NewProductRouter` y `NewStoreRouter`, orden `RequestID → AccessLog → Recover → CORS`. Los endpoints existentes responden igual que antes; lo nuevo es el header `X-Request-ID`, el log de acceso y que un panic devuelve 500 problem+json en lugar de cortar la conexión.

### Decisión: paquete compartido vs duplicado

Se **comparte** `internal/httpx` entre Product y Store API. Ambas ya son binarios del mismo módulo Go y comparten `go.mod`, `internal/http/router.go`, `internal/config` e `internal/database` (TAL-69 §3.1); el paquete no agrega un acoplamiento nuevo, mientras que duplicarlo haría divergir justamente lo que se quiere estandarizar. `httpx` solo depende de la biblioteca estándar, por lo que puede extraerse a un módulo propio sin cambios si los servicios se separan de repositorio. `internal/apiutil` (usado por Pricing, Search, etc.) **no** se modificó para no cambiar las respuestas de esas APIs.

### Pendiente para TAL-70+

- Responder problem+json en rutas inexistentes / método no permitido (`chi` `NotFound`/`MethodNotAllowed`); hoy chi responde texto plano.
- Agregar `X-Request-ID` a `Access-Control-Expose-Headers` en CORS para que el frontend pueda leerlo en orígenes cruzados (en k8s el acceso es same-origin y ya es legible).
- Migrar los handlers existentes de Product/Store al estándar dentro de cada tarea.
