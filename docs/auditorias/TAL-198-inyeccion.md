# TAL-198 — Protección contra inyección

Fecha: 2026-11-12
Alcance: `backend/` (Go). El scraper no construye SQL: sólo escribe SQLite con
sentencias fijas y consultas parametrizadas en `scraper/src/scraper/db/repository.py`.

## 1. Método

1. Revisión estática de todas las consultas del backend.
2. Revisión de los puntos de entrada (decodificación JSON, validadores, límites).
3. Pruebas nuevas que envían payloads de inyección y verifican que:
   - se rechazan antes de tocar la base, o
   - viajan como argumentos y el SQL no cambia.

## 2. Hallazgos

| # | Control | Evidencia |
|---|---|---|
| 1 | **Todas las consultas usan placeholders `$n`** | `grep -rn "Sprintf" backend/internal` sólo devuelve mensajes de validación; no hay SQL interpolado |
| 2 | **Sin concatenación de SQL** | no existe `query +=`, `+ " WHERE"` ni `strings.Builder` en consultas |
| 3 | **Búsqueda con arreglos tipados** | `internal/search/repository.go` usa `unnest($1::text[])`, `$2::int`, `$8::int[]`; los filtros son fragmentos estáticos |
| 4 | **Validación antes de la base** | `auth.go` valida email/password; `reviews_watch_notifications.go` valida `userId`, `type`, `title`, `message` |
| 5 | **JSON estricto** | `httpx.DecodeJSON` rechaza campos desconocidos y bodies sobre el límite |
| 6 | **Sin identificadores dinámicos** | no hay `ORDER BY` ni nombres de tabla/columna construidos con entrada del cliente |
| 7 | **Errores sin filtrar detalles** | los 5xx registran el error y responden `problem+json` genérico |

### Superficies revisadas

- Auth: `register`, `login`, `refresh`, `logout`, `validate`.
- Catálogo y ofertas: creación, edición, baja, búsqueda y filtros.
- Reseñas, favoritos, carrito, watchlist y notificaciones.
- Ingestion API: apertura de runs, lotes de ítems y cierre.

## 3. Pruebas añadidas

`backend/internal/features/injection_test.go`:

| Test | Qué fija |
|---|---|
| `TestInjectionPayloadsAreRejectedBeforeDatabaseAccess` | `' OR 1=1--`, `'; DROP TABLE`, `" OR ""="` y `UNION SELECT` en `/auth/register` y `/auth/login` responden 422 sin ejecutar ninguna consulta |
| `TestInjectionPayloadStaysAParameter` | el payload en `type`/`message` de una notificación se guarda como argumento; el SQL conserva `$1` y no contiene el texto |

El fake de base de datos ahora registra `lastSQL` y `lastArgs`, de modo que la
prueba puede afirmar ambas cosas en vez de confiar en la inspección visual.

## 4. Riesgos residuales

| Riesgo | Estado |
|---|---|
| Inyección en SQLite del scraper | Sin superficie: `repository.py` usa `?` con parámetros y SQL fijo |
| Inyección en logs | Los valores se registran como campos estructurados, no se concatenan a la sentencia |
| SSRF por URL en specs | El scraper sólo visita URLs declaradas en el spec, validadas con `match.detect` |
| Dependencias | `go.mod` fijado; sin ORM que genere SQL dinámico |

## 5. Cómo reproducir

```bash
cd backend
go test ./internal/features/... -run Injection -v
go test ./...
```

```bash
# revisión estática
grep -rn "Sprintf" backend/internal --include="*.go" | grep -iE "select|insert|update|delete"
grep -rn "query +=" backend/internal --include="*.go"
```

## 6. Conclusión

No se encontraron consultas construidas con entrada del cliente. La defensa es
estructural —placeholders, validadores y decodificación estricta— y ahora está
fijada con pruebas que fallan si alguien vuelve a concatenar.
