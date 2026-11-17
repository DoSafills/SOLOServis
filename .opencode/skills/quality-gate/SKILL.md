---
name: quality-gate
description: Use before committing or opening a PR, when choosing which test suite to run for a change, or when a suite fails and you need to know whether it is pre-existing. Covers the per-area command matrix and the known failures in this repo.
---

# Puerta de calidad

Qué correr antes de decir "está listo". Regla de oro: **no des por validado lo
que no ejecutaste**. Si una suite no se puede correr, se reporta con la salida.

## Matriz por área tocada

| Cambiaste | Obligatorio | Opcional |
| --- | --- | --- |
| `backend/` | `go build ./...` + `go vet ./...` + `go test ./...` | integración con `TEST_DATABASE_URL` y `go test -p 1 ./...` |
| `scraper/` | `./venv/bin/python -m pytest -q` + `./venv/bin/ruff check src tests` | una corrida real contra una tienda |
| `fronted/` | `pnpm build` + `pnpm lint` | `pnpm check` completo |
| `k8s/`, `scripts/k8s/` | `bash scripts/k8s/validate-manifests.sh` + `sh -n` de los scripts | Fase 10 del deploy |
| `docker/` | `docker compose -f docker/compose.yml config --quiet` | levantar el compose |
| `backend/database/` | `go build ./...` + conteo de `migration_order.txt` | migración sobre base limpia |
| sólo `docs/` | revisar enlaces y rutas citadas | — |

## Todo de una vez

```bash
python scripts/testing/reporte.py                     # todas las suites
python scripts/testing/reporte.py --suites backend
python scripts/testing/reporte.py --postgres-docker   # suma integración
```

Deja el resultado en `docs/reportes/testing/` y sale con código 1 si algo falla.

Los tests de integración de Go comparten una sola base: a mano se corren con
`go test -p 1 ./...` (el reporte ya lo hace). En paralelo los paquetes se pisan
los datos y aparecen fallas intermitentes que no son reales.

## Fallas conocidas (no son tuyas)

| Suite | Falla | Estado |
| --- | --- | --- |
| `ruff check src tests` | `I001` import desordenado en `scraper/tests/test_extract.py` | confirmado el 2026-11-16 |
| `pnpm format:check` | archivos con CRLF | reportado en `AGENTS.md` §13 |
| `crawler/` | 2 tests de SQLite sin cerrar, sólo en Windows | reportado en `AGENTS.md` §13 |

Si te topas con una de estas, **dilo en el PR** y no la arregles de paso: se
corrige en su propia rama para que el diff se pueda revisar.

## Qué significa "verde"

- Build y vet pasan: obligatorio siempre, aunque sólo toques docs de código.
- Los tests pasan **o** se omiten por falta de infraestructura, y lo dices.
- Un test nuevo falla si quitas el cambio que protege. Si no, no prueba nada.

## Antes del commit

- [ ] Corrí la suite del área que toqué.
- [ ] El mensaje es `tipo(TAL-XX): descripción` en español.
- [ ] Sin `Co-Authored-By` ni "Generated with".
- [ ] `git status` no muestra archivos ajenos al cambio.

## Antes del PR

- [ ] La rama sale de `develop` actualizado.
- [ ] El PR va contra `develop`, no `main`.
- [ ] La descripción dice qué cambia, cómo se validó y qué riesgo queda.
- [ ] La tarea está en `In Review` con el enlace al PR.

## Anti-patrones

- Decir "tests en verde" sin pegar la salida.
- Arreglar el fallo de otro dentro del PR propio.
- Commitear `k8s/overlays/`, `.env`, `venv/` o `node_modules/`.
- Un PR que toca tres áreas sin relación.
