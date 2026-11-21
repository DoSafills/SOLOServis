---
name: linear-workflow
description: Use when reading or updating a Linear task (TAL-XX), moving a task between states, commenting with evidence, or linking a PR to a task. Covers the scripts/linear CLI, the team and state names, and the rules for which issues may be touched.
---

# Linear — `scripts/linear/linear.py`

CLI sin dependencias externas. Lee `LINEAR_API_KEY` o el `.env` del repo.

```bash
python3 scripts/linear/linear.py --help
```

## Comandos que se usan

| Comando | Para qué |
| --- | --- |
| `whoami` | Confirmar con qué usuario se escribe |
| `projects` | Avance por proyecto |
| `mine --limit 200` | Todas las tareas propias |
| `issues --project Sprint_3 --limit 200` | Listar por proyecto |
| `show TAL-XX` | Detalle y comentarios |
| `search "texto"` | Buscar en títulos y descripciones |
| `start TAL-XX` | A `In Progress` |
| `review TAL-XX` | A `In Review` |
| `done TAL-XX` | A `Done` |
| `comment TAL-XX "texto"` | Comentario |
| `raw '<graphql>'` | Consulta cruda cuando falta un comando |

`--json` para parsear, `--dry-run` para ver la mutación sin ejecutarla.

## Reglas

1. **Sólo se tocan las tareas propias.** El workspace es compartido con cuatro
   personas más; mover o comentar la tarea de otro es un error grave.
2. **Una rama por dominio**, desde `develop` (`feat/api`, `feat/backend`,
   `feat/frontend`, `feat/test`…; ver AGENTS.md §6). El identificador va en
   el commit: `test(TAL-208): reintentos`.
3. **`In Review` al abrir el PR**, y el comentario lleva el enlace y qué se
   entregó.
4. **`Done` sólo cuando el merge está confirmado**, no cuando el PR se abrió:

```bash
git merge-base --is-ancestor <commit-del-merge> origin/develop && echo mergeado
```

5. **El comentario de cierre cita evidencia:** número de PR, commit corto y
   fecha. Sin eso no sirve para auditar.
6. **Un comentario explica el porqué**, no repite el título de la tarea.

## Formato de comentario que funciona

```text
Mergeado en develop con el PR #78 (commit 1c6081f, 2026-11-15).

<qué se entregó, en dos o tres frases, con rutas de archivo>

Queda cerrada como Done. Flujo restante: <qué viene después>.
```

## Notas técnicas

- El header es `Authorization: <key>` **sin** `Bearer`.
- `--oldest` ordena por `createdAt` en memoria, porque la API devuelve
  descendente; escanea hasta 1000 elementos.
- Los estados son `Backlog`, `Todo`, `In Progress`, `In Review`, `Done`,
  `Canceled`, `Duplicate`.
- Si la API falla, revisa que la key no esté vacía o vencida antes de tocar el
  código del CLI.

## Ante un error

- `401`: key inválida o mal formada.
- `400` con `Argument Validation Error`: el identificador (`TAL-XX`) no existe
  o el estado no pertenece al equipo.
- Un cambio que no aparece: confirma con `show TAL-XX` antes de repetirlo.

## Anti-patrones

- Cerrar una tarea porque el PR se abrió.
- Comentar "listo" sin PR ni commit.
- Reasignar tareas ajenas.
- Cambiar prioridades o proyectos sin que se pida.
