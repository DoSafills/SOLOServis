---
name: backend-api
description: Use when adding or changing an endpoint, handler, repository query or Go test under backend/. Covers the TAL-78 problem+json standard, the 400/422/404/409/500 rules, pgx error mapping and the table-driven test harness.
---

# Backend Go — `backend/`

Reglas generales en el `AGENTS.md` de la raíz (§7). Norma completa:
`docs/estandares/TAL-78-estandar-api.md`. Si el documento y el código
discrepan, gana el código: corrígelo en el mismo cambio.

## Mapa

| Ruta | Qué hay |
| --- | --- |
| `cmd/all-apis/` | Binario único; `API_DOMAIN` (`comercial`, `usuarios`, `ingestion`) elige las APIs |
| `cmd/<api>/` | Binario por API, sólo para el modo microservicios |
| `internal/<api>/` | Handler, repositorio y tests de cada API |
| `internal/httpx/` | `WriteProblem`, `WriteValidation`, `WriteDecodeError`, `WriteInternal`, `DecodeJSON`, `Validator`, logs, request id |
| `internal/authz/` | `RequireAdmin`, `RequireSession`, token de servicio |
| `internal/allapis/` | Dominios y URLs internas entre APIs |
| `database/queries/` | SQL de sqlc |
| `internal/database/generated/` | Código generado. **No se edita a mano** |

## Reglas que rompen el build o el contrato

1. **Errores:** siempre `problem+json` con `code` estable y `request_id`.
   Nunca `http.Error` nuevo.
2. **400 vs 422:** falla al decodificar → 400. Decodifica pero incumple reglas
   → 422 con `errors[]` por campo.
3. **Errores de base:** `pgx.ErrNoRows` → 404, `23505` → 409, `23503`/`23514`
   → 422, el resto → 500. Ningún comodín.
4. **Nada de detalles internos** (SQL, tablas, stack) en la respuesta. El 5xx se
   registra con `request_id`.
5. **Éxito:** 201 con `Location` en POST, 204 en DELETE. POST y PUT responden
   la misma forma que el GET del recurso.
6. **Handlers contra una interfaz** de repositorio, para poder simular fallos.
7. **Autorización:** sesión, rol `admin` o token de servicio. El `userId` del
   cliente no se confía jamás; se usa el de la sesión.
8. **Mensajes al usuario en español, JSON en camelCase.** SQL parametrizado.

## Cómo se escribe un test

El arnés vive en `internal/features/handler_test.go`:

```go
db := &fakeDB{t: t, rules: []rule{
    qRow("SELECT id FROM service WHERE public_id", int32(20)),
    qExec("INSERT INTO review", 1),
}}
runCases(t, "review-api", []apiCase{
    {name: "feliz", method: http.MethodPost, target: "/reviews/product/1",
     body: `{"rating":5}`, token: "tok-cliente",
     wantStatus: http.StatusCreated, wantFields: []string{"id"}},
    {name: "sin sesión", method: http.MethodPost, target: "/reviews/product/1",
     body: `{"rating":5}`, token: "-", wantStatus: http.StatusUnauthorized},
})
```

- `qRow` / `qRows` / `qExec` declaran qué devuelve cada fragmento de SQL.
  Una consulta sin regla **hace fallar el test**: cada caso declara lo que toca.
- Casos mínimos por endpoint: feliz, 400, 422, 404 y 500 simulado.
- Sólo biblioteca estándar: `testing`, `net/http/httptest`, `t.Run`.

## Comandos

```bash
cd backend
go build ./... && go vet ./... && go test ./...
go test ./internal/features/... -run TestAlgo -v   # un paquete, verboso
```

Los tests de integración se omiten sin `TEST_DATABASE_URL` apuntando a un
PostgreSQL 18 migrado en el orden de `migration_order.txt`. Con la variable,
corre `go test -p 1 ./...`: los paquetes comparten la base y en paralelo se
pisan los datos.

## Antes de abrir el PR

- [ ] `go build ./...`, `go vet ./...` y `go test ./...` en verde.
- [ ] Cada endpoint nuevo tiene sus cinco casos.
- [ ] Ninguna consulta concatena valores; todo va con `$1`, `$2`…
- [ ] Si cambió una consulta de `database/queries/`, se regeneró con sqlc.
- [ ] El README de `backend/` documenta el endpoint si es nuevo.

## Anti-patrones vistos en este repo

- Devolver 404 para cualquier error de base.
- Validar el `userId` del body en vez de usar la sesión.
- Añadir un `if` de rol dentro del handler en vez de usar `authz`.
- Test que mockea la base entera cuando bastaba una regla de `fakeDB`.
