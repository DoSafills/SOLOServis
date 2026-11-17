# TAL-235 — CI/CD con GitHub Actions

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-16 |
| Alcance | `.github/workflows/ci.yml`, `AGENTS.md` §13 |
| Estado previo | No existía `.github/workflows/`. Todo se validaba a mano |

## 1. Decisión

CI de **integración continua** con cinco jobs independientes, sin secretos y sin
infraestructura externa. El despliegue sigue siendo manual (`scripts/k8s/deploy.sh`):
automatizarlo requiere registry, kubeconfig del clúster UCT y una decisión sobre
quién puede desplegar, que no corresponde a esta tarea.

Cada job ejecuta exactamente el comando que documenta el arnés
(`AGENTS.md` y `.opencode/skills/`). Si el arnés y el CI divergen, uno de los dos
está mal, y eso es visible en el primer rojo.

## 2. Jobs

| Job | Comando | Qué protege |
| --- | --- | --- |
| `backend` | `go build ./...`, `go vet ./...`, `go test ./...` | Que `develop` compile: ya pasó que no compilaba tras un merge (PR #85) |
| `scraper` | `ruff check src tests`, `pytest -q` | Estilo y 115 pruebas del scraper |
| `frontend` | `pnpm install --frozen-lockfile`, `pnpm lint`, `pnpm build` | Que el frontend compile y no tenga errores de ESLint |
| `skills` | `./scripts/check-skills.sh` | Que `.opencode/skills` y `.claude/skills` no deriven |
| `compose` | `docker compose ... config --quiet` (ambas topologías) | Que los compose sigan siendo válidos |

Disparadores: `push` y `pull_request` contra `develop` y `main`. Concurrencia
cancelable por rama para no gastar minutos en pushes intermedios.

## 3. Lo que queda fuera, y por qué

| Excluido | Motivo |
| --- | --- |
| Tests de integración de Go | Necesitan PostgreSQL 18 migrado; se levantan a mano con `TEST_DATABASE_URL`. Añadir un service container es posible, pero multiplica el tiempo de cada PR |
| `pnpm format:check` | Hoy falla por archivos con CRLF (AGENTS.md §13). Entra cuando se normalicen los finales de línea |
| `docker build` y `docker push` | Requieren credenciales de Docker Hub como secrets del repo |
| Despliegue al clúster | Requiere kubeconfig y una política de quién despliega |
| Pod de test (Fase 10) | Vive dentro del clúster; se corre tras el deploy |

## 4. Hallazgos al implementarlo

1. **`ruff` ya estaba rojo en `develop`**: `I001` en
   `scraper/tests/test_extract.py`, introducido por el merge de TAL-173. Se
   corrigió en el PR #97, en rama aparte, para que este PR no mezcle áreas.
2. **`scripts/check-skills.sh` llega en TAL-236** (PR #96). Mientras no esté
   mergeado, el job `skills` emite un `::warning::` y pasa. Cuando el script
   exista, se ejecuta solo: no hay que tocar el workflow.
3. **`lint-staged` de la raíz** apuntaba a `backend/**/*.ts` y `frontend/`, rutas
   inexistentes. Corregido en TAL-236.

## 5. Verificación local (lo mismo que corre el CI)

```bash
cd backend && go build ./... && go vet ./... && go test ./...
cd scraper && ./venv/bin/ruff check src tests && ./venv/bin/python -m pytest -q
cd fronted && pnpm lint && pnpm build
./scripts/check-skills.sh
docker compose -f docker/compose.yml config --quiet
docker compose -f docker/compose.microservicios.yml config --quiet
```

Resultado el 2026-11-16:

| Comando | Resultado |
| --- | --- |
| `go build`, `go vet`, `go test` | verde |
| `pytest` | verde (115 pruebas) |
| `ruff` | rojo por `I001` ajeno; corregido en #97 |
| `pnpm lint`, `pnpm build` | no ejecutados aquí (sin `node_modules` local) |
| `check-skills.sh` | verde en la rama de TAL-236 |
| `compose config` (ambas) | verde |

Los dos comandos de frontend no se pudieron ejecutar en esta máquina: se
reportan como no verificados, no como aprobados. El primer run del workflow es
la verificación real.

## 6. Recomendación de protección de rama

En GitHub, sobre `develop`: exigir los cinco jobs antes del merge. Sin eso, el
CI informa pero no impide. Es una acción de administración del repo, fuera del
alcance de un PR.

## 7. Riesgos residuales

| Riesgo | Mitigación |
| --- | --- |
| `pip install -e ".[dev]"` instala Scrapling y alarga el job | Caché de pip por `pyproject.toml`; el job tiene 20 minutos de margen |
| El job `frontend` depende de `fronted/pnpm-lock.yaml` | `--frozen-lockfile` falla si el lock y el `package.json` divergen: es lo deseado |
| CI verde no significa desplegado | El pod de test de la Fase 10 sigue siendo la verificación del clúster |
| Minutos de CI | Concurrencia cancelable y sin PostgreSQL; cinco jobs cortos |

## 8. Siguiente paso natural

Cuando haya registry y política de despliegue: un workflow `release.yml` que
construya y publique las tres imágenes con tag = SHA, y que el deploy consuma
ese tag en vez de construirlas en la máquina de cada persona.
