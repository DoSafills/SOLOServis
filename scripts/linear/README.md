# linear.py — CLI de Linear para la terminal

Cliente de Linear sin dependencias externas: sólo la librería estándar de
Python 3.9+ (`urllib`, `argparse`, `json`). No hay que instalar nada.

```
scripts/linear/linear.py <comando> [opciones]
```

## Credenciales

La API key se busca en este orden:

1. variable de entorno `LINEAR_API_KEY`
2. `.env` en el directorio actual o en cualquier ancestro
3. `.env` en la raíz del repositorio

```bash
# opción A: exportar en la sesión
export LINEAR_API_KEY=lin_api_...

# opción B: dejarla en el .env del repo (ya está en .gitignore)
echo 'LINEAR_API_KEY=lin_api_...' >> .env
```

Las **personal API keys** de Linear viajan en `Authorization: <key>` **sin** el
prefijo `Bearer`. Con `Bearer` la API responde `401`.

## Uso rápido

```bash
L=scripts/linear/linear.py

$L whoami                          # quién es el dueño de la key y su workspace
$L teams                           # equipos y cantidad de issues
$L users                           # personas del workspace
$L states                          # columnas del flujo (Backlog, Todo, …)
$L projects                        # proyectos y % de avance
$L stats                           # resumen por estado y por persona
```

### Leer issues

```bash
$L issues                          # últimas 25, más recientes primero
$L issues --limit 100
$L issues --state "In Progress"
$L issues --state done             # los alias en minúscula también sirven
$L issues --assignee Ricardo
$L issues --project Sprint_3
$L issues --search precio
$L issues --all-teams              # sin filtrar por equipo

$L mine                            # mis issues asignadas
$L mine --state Todo
$L person Ricardo                  # issues de una persona (nombre o email)
$L person "CÁCERES" --state "In Review"

# orden: por defecto lo más reciente primero (por última actualización)
$L mine --oldest                   # mis issues más antiguas primero (por creación)
$L issues --oldest --limit 50      # las más antiguas del equipo
$L mine --sort created             # las creadas más recientemente primero
$L mine --oldest --sort updated    # las que llevan más tiempo sin tocarse

$L show TAL-233                    # detalle + comentarios
$L show 233                        # el prefijo del equipo es opcional
$L open TAL-233                    # abre en el navegador
$L open TAL-233 --print-only       # o sólo imprime la URL
```

### Mover y marcar issues

```bash
$L start TAL-233                   # → In Progress
$L review TAL-233                  # → In Review
$L done TAL-233                    # → Done
$L todo TAL-233                    # → Todo
$L backlog TAL-233                 # → Backlog
$L move TAL-233 "In Review"        # cualquier estado, por nombre o alias
```

Alias disponibles: `doing` = `start`, `complete` = `done`, `mv` = `move`.

### Asignar, comentar y crear

```bash
$L assign TAL-233 "Ricardo Rios"   # por nombre o email
$L assign TAL-233                  # sin persona: desasigna

$L comment TAL-233 "listo para revisión"
echo "texto largo" | $L comment TAL-233 -     # leer de stdin

$L create --title "Validar precios" \
          --description "Detalle…" \
          --assignee Ricardo \
          --project Sprint_3 \
          --state Todo \
          --priority 2
```

Si el workspace tiene un solo equipo (como hoy), `--team` es opcional. Con
varios equipos hay que indicarlo.

## Opciones globales

Valen antes o después del subcomando:

| Flag | Efecto |
|---|---|
| `--json` | salida JSON cruda, ideal para pipes con `jq` |
| `--dry-run` | imprime la mutación que enviaría, **sin ejecutarla** |
| `--team KEY` | fuerza el equipo (por defecto el único del workspace) |
| `--no-color` | desactiva colores ANSI (también respeta `NO_COLOR`) |

```bash
$L --json issues --state Todo | jq -r '.[].identifier'
$L issues --state Todo --json | jq length
```

## Consultas crudas

```bash
$L raw 'query { viewer { name } }'
$L raw 'query { issues(first: 3) { nodes { identifier } } }'
$L raw @consulta.graphql --vars '{"first": 5}'
```

## Notas de seguridad

- El script **no implementa mutaciones destructivas** (`issueDelete`,
  `teamDelete`, `organizationDelete`, etc.). A propósito.
- La key del `.env` es de tipo administrador: puede borrar y crear webhooks.
  Trátala como credencial de producción y no la expongas en el frontend
  (`fronted/`), porque todo `VITE_*` termina en el bundle público.
- Antes de una mutación masiva, usa `--dry-run` para revisar el payload.
- Los cambios quedan atribuidos al dueño de la key, no necesariamente a quien
  ejecuta el comando.
