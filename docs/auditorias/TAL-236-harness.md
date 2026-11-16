# TAL-236 — Arnés de agentes

| Campo | Valor |
| --- | --- |
| Fecha | 2026-11-16 |
| Alcance | Documentación y tooling: `.opencode/skills/`, `.claude/skills/`, `AGENTS.md`, `scripts/check-skills.sh`, `.gitignore`, `package.json` |
| Código de producción | Ninguno. No se tocó backend, scraper ni frontend |

## 1. Qué es un arnés

El arnés es lo que permite que un agente —o una persona nueva— entre al repo y
actúe sin arqueología: dónde está cada cosa, qué comando se corre, qué está
prohibido y cómo se sabe que algo quedó bien.

Se compone de tres capas:

1. **Mapa** (`AGENTS.md` raíz): qué es el proyecto, stack real, flujo de ramas.
2. **Skills por área**: el *cómo* operativo, con comandos que se ejecutan tal cual.
3. **Verificación**: un comando que confirma que el arnés sigue coherente.

La capa 3 es la que suele faltar y la que convierte la documentación en algo que
no se pudre en silencio.

## 2. Inventario inicial

| Archivo | Líneas | Estado |
| --- | --- | --- |
| `crawler/AGENTS.md` | 331 | **Obsoleto.** Describe el crawler v1 (`src/crawler/`, `database.py`), que ya no se despliega |
| `fronted/AGENTS.md` | 41 | **Plantilla de Figma Make**: "figma-make-app", dev server ya corriendo en `$PORT` |
| `fronted/CLAUDE.md` | 1 | `@AGENTS.md` |
| `AGENTS.md` raíz | 228 | **Añadido por Marcelo Vidal el 2026-11-16 12:49**, durante este mismo sprint |
| `backend/`, `scraper/`, `k8s/`, `docker/`, `scripts/`, `docs/` | — | Sin reglas propias |
| Skills | 0 | `grep -ri skill` no devolvía nada |

Cobertura efectiva: **2 de 8 áreas**, una describiendo código muerto y la otra
siendo boilerplate ajeno al proyecto.

El `AGENTS.md` raíz cambió el alcance de esta tarea: ya no había que escribirlo,
sino **completarlo**. Duplicar su contenido en los skills habría creado dos
fuentes de verdad, que es el defecto que el arnés debe evitar.

## 3. Huecos cerrados

| Hueco | Cómo se cerró |
| --- | --- |
| Sin reglas operativas por área | Siete skills con disparador, comandos y definición de término |
| Ninguna forma de detectar un skill desactualizado | `scripts/check-skills.sh` + espejo verificado |
| `crawler/AGENTS.md` guiaba trabajo nuevo hacia código muerto | Nota de legado al inicio del archivo |
| `fronted/AGENTS.md` no decía qué es SOLOServis | El skill `frontend-ui` lo cubre; el archivo se conserva por el flujo de Figma |
| Pre-commit apuntaba a rutas inexistentes | `lint-staged` corregido (§4) |
| `.claude/` ignorado por git | Excepción `!.claude/skills/` en `.gitignore` |

## 4. Lo construido

### Skills

| Skill | Disparador | Contenido |
| --- | --- | --- |
| `backend-api` | Endpoints, handlers, repositorios, tests de Go | Mapa de `internal/`, reglas problem+json, 400/422/404/409/500, receta de test con `fakeDB`/`runCases` |
| `scraper-spec` | Scraper, tienda nueva, extracción, ingesta | Spec-first, contrato de ingestion, contadores `records_*` vs `items_*`, comandos con el venv |
| `frontend-ui` | `fronted/` | Stack real, `VITE_*`, problem+json por `code`, `pnpm check`, aviso de que no hay tests |
| `database-migrations` | SQL, `migration_order.txt`, sqlc | Orden explícito, checksums, transacción manual, `sqlc generate`, conteo archivos == índice |
| `k8s-deploy` | Despliegue, manifiestos, pod de test | 11 fases, cuota, LimitRange, Pod Security, `enableServiceLinks`, smoke test |
| `linear-workflow` | Tareas TAL, estados, comentarios | CLI, header sin `Bearer`, sólo tareas propias, `Done` tras verificar el merge |
| `quality-gate` | Antes de commit o PR | Matriz por área, fallas conocidas, qué significa "verde" |

Cada skill pesa entre 95 y 140 líneas: lo suficiente para no obligar a leer el
README de 2 493 líneas del backend, y lo bastante corto para caber en contexto.

### Espejo entre agentes

El contenido canónico vive en `.opencode/skills/`. `.claude/skills/` es copia
idéntica, porque cada agente lee de su propio directorio.

Se descartaron los symlinks: el repo se clona en Windows para el despliegue
(`deploy.ps1`, PowerShell 5.1), donde requieren configuración extra. La
duplicación se controla con un verificador:

```bash
./scripts/check-skills.sh --sync   # .opencode -> .claude
./scripts/check-skills.sh          # falla si difieren
```

### Otros cambios

- `AGENTS.md` raíz: sección "Arnés de agentes" con la tabla de skills y la
  regla de sincronización.
- `crawler/AGENTS.md`: nota de legado; el documento se conserva como historia.
- `.gitignore`: `!.claude/skills/` para que el espejo se versione.
- `package.json`: `lint-staged` apuntaba a `backend/**/*.ts` (no existe: el
  backend es Go) y a `frontend/src/**` (el directorio es `fronted/`). El
  pre-commit no formateaba nada; ahora sí.

## 5. Verificación

```text
$ ./scripts/check-skills.sh
OK: 7 skills sincronizados entre .opencode y .claude

$ diff -r .opencode/skills .claude/skills
(sin salida)
```

- 49 rutas citadas en los skills comprobadas contra el árbol; las que no existen
  son plantillas (`stores/<slug>/spec.json`), artefactos de ejecución
  (`out/rejected/run-{id}.jsonl`) o nombres de rama.
- `python3 -c "import json;json.load(open('package.json'))"` — JSON válido.
- `git check-ignore .claude/skills/backend-api/SKILL.md` — ya no se ignora.

## 6. Defectos encontrados y no corregidos aquí

| Defecto | Por qué no se corrigió en esta rama |
| --- | --- |
| `021_dedupe_scraped_products.sql` existe pero **no está en `migration_order.txt`**: nunca se aplica | Es de TAL-169 y toca criterio de datos; va en rama propia |
| `ruff check` falla con `I001` en `scraper/tests/test_extract.py` | Falla preexistente en `develop`, ajena a documentación |
| `fronted/AGENTS.md` sigue siendo boilerplate de Figma | Reescribirlo puede romper el flujo de Figma Make; el skill cubre el vacío |

## 7. Riesgos residuales

| Riesgo | Mitigación |
| --- | --- |
| Los skills envejecen cuando cambia el código | `AGENTS.md` ya declara que gana el código; el skill se corrige en el mismo PR |
| Nadie ejecuta `check-skills.sh` | TAL-235 lo añade como job de CI |
| Dos directorios duplicados | El verificador lo detecta; el espejo se regenera con `--sync` |
| Un skill demasiado largo deja de leerse | Tope práctico: si pasa de ~150 líneas, se parte o se delega al README |

## 8. Cómo reproducir

```bash
./scripts/check-skills.sh
find .opencode/skills -name SKILL.md | sort
grep -n "Arnés de agentes" -A 20 AGENTS.md
```
