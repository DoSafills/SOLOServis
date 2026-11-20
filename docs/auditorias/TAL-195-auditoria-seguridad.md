# TAL-195 — Auditoría de seguridad

Fecha: 2026-11-20
Rama: `feat/api` (al día con `origin/develop`, `0b1e323`)
Alcance: backend Go, gateway nginx, frontend, scraper v2, Docker Compose,
manifiestos de Kubernetes, scripts de despliegue e historial de git.
Tipo: revisión estática y de configuración. **No se cambió código.**

Auditorías previas que esta no repite: TAL-162 (autenticación y permisos) y
TAL-198 (inyección SQL).

Los hallazgos alimentan cuatro tareas:

| Tarea | Tema | Hallazgos |
|---|---|---|
| TAL-197 | Protección de endpoints | H4, H7, H12, H14 |
| TAL-199 | Gestión de secretos | H3, H8, H9, H10, H11 |
| TAL-200 | Control de sesiones | H2, H5, H6, H15 |
| TAL-201 | Solicitudes excesivas | H1, H13 |
| chore (backend) | Dependencias de Go | H16 |

---

## 1. Método

1. **Matriz de rutas:** todas las rutas registradas en `backend/internal/http/router.go`
   y `backend/internal/features/router.go`, con su middleware, cruzadas con
   lo que publica el gateway (`docker/frontend/default.conf.template`).
2. **Propiedad de recursos (IDOR):** cada `UPDATE`/`DELETE`/`SELECT` por id en
   rutas con sesión se revisó en busca del filtro `user_id = <sesión>`.
3. **Sesiones:** `features/auth.go`, `authz/authz.go` y el almacenamiento en
   `fronted/src/services/api/personalization.ts`.
4. **Solicitudes excesivas:** límites de paginación, body, lote, timeouts del
   servidor, pool de PostgreSQL, nginx y recursos de los pods.
5. **Secretos:** `git grep` sobre el árbol y `git log --all -G` sobre el
   historial, buscando patrones de token, contraseña y llave. Los valores se
   enmascararon al revisar; ninguno se copia en este documento.
6. **Gateway y frontend:** cabeceras, CORS y sumideros XSS
   (`dangerouslySetInnerHTML`, `href` con datos externos).
7. **Dependencias:** `pnpm audit`, `govulncheck ./...` y `pip-audit`.
8. **Kubernetes:** `securityContext`, montaje de Secrets, Services y NetworkPolicy.

---

## 2. Matriz de rutas

`S` = `RequireSession` (el usuario sale de la sesión), `A` = `RequireAdmin`
(sesión con rol admin o `INTERNAL_API_TOKEN`), `—` = pública.
Gateway: `GET` = solo lectura (`limit_except GET`), `sí` = todos los métodos, `no` = no se publica.

| API | Rutas | Protección | Gateway |
|---|---|---|---|
| product | `GET /products`, `/products/{id}`, `/products/{id}/reviews`, `/product-categories`, `/categories*` | — | GET |
| product | `POST/PUT/DELETE /products*`, `POST /products/duplicate-candidates` | A | bloqueado (GET) |
| store | `GET /stores*` | — | GET |
| service | `GET /services*` | — | GET |
| pricing | `GET /offers*` | — | no |
| pricing | `POST /offers`, `PUT /offers/{id}` | A | no |
| search | `GET /search/products`, `popular-products`, `presets*`, `facets`, `categories`, `brands` | — | sí |
| search | `/search/history`, `/search/saved*` | S | sí |
| comparison | `GET /comparisons/products/compare`, `/comparisons/compatible/{id}` | — | sí |
| comparison | `/comparisons/products*`, `/comparisons/services*` | S | sí |
| auth | `/auth/register`, `login`, `refresh`, `logout`, `validate` | — (son la entrada) | GET, POST |
| favorites | `/favorites*` | S | sí |
| cart | `/cart*` | S | sí |
| review | `GET /reviews/{product,store,service}/{id}` | — | sí |
| review | `POST /reviews/*`, `PUT/DELETE /reviews/{id}` | S | sí |
| watchlist | `/watchlist*` | S | sí |
| notification | `POST /notifications` | A | no |
| notification | `GET /notifications*`, `PUT .../read*` | S | no |
| user | `GET /users` (estado), `GET /users/{id}` (solo el propio) | —, S | no |
| ingestion | `/ingestion/runs*` | A | no |

**Resultado:** ninguna escritura queda sin protección. Las rutas con sesión
filtran por `user_id` de la sesión en todas las consultas por id (carrito,
reseñas, notificaciones, búsquedas guardadas y comparaciones de productos y
servicios); un id ajeno responde 404. No se encontró IDOR.

---

## 3. Hallazgos

Severidad: **Alta** = explotable hoy desde internet con impacto en
disponibilidad o cuentas. **Media** = requiere una condición adicional o
debilita una defensa. **Baja** = endurecimiento o higiene.

### H1 — Sin límite de solicitudes en ninguna capa · Alta · TAL-201

- No hay `limit_req` en nginx ni middleware de límites en Go
  (`grep -ri 'limit_req\|ratelimit\|limiter'` no devuelve nada).
- `POST /auth/login` y `POST /auth/register` calculan PBKDF2-SHA256 con
  120 000 iteraciones (`backend/internal/features/auth.go:19`).
  Medido en esta máquina con la implementación estándar de Go: **~38 ms de
  CPU por intento**.
- El pod `usuarios` tiene un límite de **200m de CPU**
  (`k8s/components/dominios/usuarios.yaml:80`). Con ese límite cada login
  cuesta ~190 ms de tiempo real: **unos 5 intentos por segundo bastan para
  saturar el dominio `usuarios`** y dejar a todos sin login.
- Además no hay freno a la fuerza bruta de contraseñas (riesgo que ya dejó
  abierto TAL-162).

**Recomendación:** límite por IP en nginx para `/auth/` (más estricto) y uno
general para las APIs, más un límite por email en login dentro de Go. Ojo:
detrás del Ingress la IP real llega en `X-Forwarded-For`; nginx necesita
`real_ip_header`/`set_real_ip_from` para no limitar a todos como si fueran
una sola IP.

### H2 — Enumeración de usuarios · Media · TAL-200

- **Por tiempo:** si el email no existe, `Login` responde sin calcular PBKDF2
  (`auth.go:91-96`); si existe, tarda ~38 ms más. La diferencia es medible
  desde fuera.
- **Por registro:** `POST /auth/register` responde 409 «El email ya está
  registrado.» (`auth.go:65`).

**Recomendación:** en login, calcular un PBKDF2 contra un hash ficticio
cuando el email no existe. El 409 del registro se acepta por usabilidad,
siempre que H1 limite cuántos emails se pueden probar.

### H3 — Un solo token de servicio con poder total · Media · TAL-199

- `INTERNAL_API_TOKEN` equivale a rol admin en las 14 APIs (`authz.RequireAdmin`),
  y el mismo valor es también el token de la API de control del scraper
  (`k8s/base/scraper/deployment.yaml:48-54`).
- Se monta en 6 workloads: los 3 dominios (o el consolidado), el scraper, el
  CronJob del scraper y los 2 Jobs de test.
- El scraper procesa HTML de terceros. Si alguien compromete ese pod, obtiene
  admin sobre el catálogo, los precios y las notificaciones.

**Recomendación:** separar tokens por uso (`INGESTION_TOKEN` solo para
`/ingestion/*`, `SCRAPER_API_TOKEN` solo para el scraper) y dejar de montarlo
donde no se usa: el dominio `usuarios` no lo necesita para atender `/auth`.

### H4 — Sin NetworkPolicy en un clúster compartido · Media · TAL-197

- `k8s/` no define ninguna `NetworkPolicy`. Los Services `comercial`,
  `usuarios`, `ingestion`, `postgres` y `scraper` quedan accesibles desde
  cualquier pod del namespace y, según la configuración del clúster UCT,
  posiblemente desde otros namespaces.
- Quien llega directo a `comercial:8080` se salta las restricciones del
  gateway (`limit_except GET`, rutas no publicadas como `/notifications`).
  La autenticación de Go sigue funcionando, pero el gateway deja de servir
  de barrera.

**Recomendación:** antes de proponer el cambio, verificar que la plataforma
UCT permita NetworkPolicy. Después, una política por defecto que niegue el
ingreso y permita solo `frontend → dominios`, `scraper → ingestion`,
`ingestion → comercial` y `apis/migraciones → postgres`.

### H5 — Tokens en `localStorage` y sesiones que no vencen nunca · Media · TAL-200

- El frontend guarda el access token y el refresh token en `localStorage`
  (`fronted/src/services/api/personalization.ts:217,250`). Cualquier XSS los
  puede leer.
- Cada refresh crea una sesión nueva de **30 días** (`auth.go:127`). Mientras
  el cliente refresque, la sesión **no tiene vencimiento absoluto**.

**Recomendación:** fijar un vencimiento absoluto **sin migración**: al
refrescar, la sesión nueva hereda el `refresh_expires_at` de la que se borra en
lugar de `NOW()+30 days`; así la sesión vence a los 30 días del login aunque se
rote. (`created_at` no sirve porque se reinicia con cada rotación.) Además,
evaluar mover el refresh token a una cookie `HttpOnly; Secure; SameSite=Strict`
limitada a `/auth/refresh`, coordinado con frontend (Cristian).

### H6 — Sesiones sin limpieza ni tope · Media · TAL-200

- Las filas vencidas de `user_session` nunca se borran, y cada login crea una
  fila nueva sin tope por usuario (agrava H1).
- No existe «cerrar todas mis sesiones» ni cambio de contraseña. Si roban un
  refresh token, el usuario no puede revocarlo.

**Recomendación:** borrar las sesiones vencidas del usuario al hacer login,
limitar las activas por usuario (por ejemplo, 10, borrando las más antiguas)
y agregar `POST /auth/logout-all`. Nada de esto necesita migración. Un
CronJob de purga es la alternativa, pero consume cuota de pods.

### H7 — Gateway sin cabeceras de seguridad · Media · TAL-197

`docker/frontend/default.conf.template` solo envía `Cache-Control`. Faltan:

- `Content-Security-Policy`: es la mitigación principal de XSS, y hoy importa
  por H5;
- `X-Content-Type-Options: nosniff`;
- `frame-ancestors 'none'` o `X-Frame-Options: DENY` contra clickjacking;
- `Referrer-Policy`;
- `server_tokens off`.

**Recomendación:** agregarlas a nivel `server` con `always`. Ojo: en nginx un
`add_header` dentro de un `location` anula los heredados, así que las
`location` que ya tienen `Cache-Control` deben repetirlas o usar un `include`.
Para HSTS, primero confirmar dónde termina TLS (proxy UCT).

### H8 — Valores por defecto inseguros en Compose · Baja · TAL-199

- `POSTGRES_PASSWORD: postgres` (`docker/compose.yml:50`, `compose.microservicios.yml:15`).
- `INTERNAL_API_TOKEN` con un valor por defecto conocido, escrito en el repo
  (`docker/compose.yml:41,132,135`).
- Los puertos se publican en todas las interfaces (`"5432:5432"`,
  `"8081:8080"`…`"8095:8095"`). En una máquina con la red abierta, cualquiera
  en la red accede a la base y tiene admin con el token por defecto.

**Recomendación:** publicar en `127.0.0.1:` y hacer que el token sea
obligatorio (`${INTERNAL_API_TOKEN:?…}`) o generado.

### H9 — Datos de infraestructura de Neon en el historial · Baja · TAL-199

Los commits `1fa1427` y `029a105` contienen el **host y el usuario** de una
base Neon en archivos de auditoría. **La contraseña no se filtró**: el script
la leía con un prompt seguro (`Read-Host -AsSecureString`). El árbol actual ya
no los contiene.

**Recomendación:** si esa base de Neon sigue activa, rotar su contraseña por
precaución o darla de baja. No hace falta reescribir el historial.

### H10 — El scraper no exige largo mínimo de token · Baja · TAL-199

Go ignora un `INTERNAL_API_TOKEN` de menos de 32 caracteres
(`authz.MinServiceTokenLength`). La API de control del scraper acepta
cualquier largo (`scraper/src/scraper/api/app.py:113-128`).

**Recomendación:** igualar la regla en `scraper/config.py`.

### H11 — Secrets creados con `--from-literal` · Baja · TAL-199

`deploy.sh:451,472` y `deploy.ps1:669,687` pasan el valor generado como
argumento de `kubectl`. Mientras corre, queda visible en la lista de procesos
de la máquina que despliega.

**Recomendación:** pasar el valor por stdin (`--from-env-file=/dev/stdin`) o
por un archivo temporal con permisos 600. Hay que cambiar los dos scripts
(ASCII y bash 3.2).

### H12 — Las reseñas públicas exponen el `userId` interno · Baja · TAL-197

`GET /reviews/*` devuelve `'userId', user_id`
(`features/reviews_watch_notifications.go:39,75,105`). Eso permite enumerar
los ids internos y correlacionar la actividad de un usuario.

**Recomendación:** devolver un nombre visible o nada. Antes, revisar qué usa
el frontend.

### H13 — Sin tope de filas por usuario · Baja · TAL-201

`saved_search`, `search_query`, favoritos y carrito aceptan inserciones sin
límite por usuario. Con una sesión válida se puede inflar la base.

**Recomendación:** topes por usuario en los handlers (por ejemplo, 50
búsquedas guardadas) y el límite general de H1.

### H14 — `docker/frontend/nginx.conf` es un resto engañoso · Baja · TAL-197

El Dockerfile usa `default.conf.template`. `nginx.conf` sigue versionado con
rutas viejas y **sin `limit_except` en `/products`**. Si alguien lo vuelve a
usar, publica las escrituras del catálogo.

**Recomendación:** borrarlo.

### H15 — PBKDF2 implementado a mano · Informativo · TAL-200

`auth.go:262-290` implementa PBKDF2 por su cuenta. Es correcto (comparación
en tiempo constante y validación del rango de iteraciones), pero Go 1.27 ya
trae `crypto/pbkdf2`, y mantener código criptográfico propio es un riesgo.

**Recomendación:** reemplazarlo por `crypto/pbkdf2` sin cambiar el formato del
hash, así no hace falta migrar contraseñas.

### H16 — Dependencias de Go con vulnerabilidades alcanzables · Media · chore (backend)

`govulncheck ./...` (v1.10.0, base al 2026-11-11) informa **13
vulnerabilidades alcanzables** desde el código: 11 de la librería estándar
(Go 1.27.1) y 2 de `golang.org/x/text v0.29.0`. Además hay 4 que no se llaman.

| ID | Paquete | Qué permite | Corregido en | Alcance real |
|---|---|---|---|---|
| GO-2026-6859 | `net/http` | Caída del servidor HTTP/1 por una carrera en `MaxBytesReader` | go1.27.4 | **Alcanzable:** `httpx.DecodeJSON` usa `MaxBytesReader` en todas las escrituras |
| GO-2026-6613, 6605 | `net/http` | Desincronización HTTP/1 tras `CONNECT` | go1.27.2 | Bajo: nginx no reenvía `CONNECT` |
| GO-2026-6608 | `net/textproto` | Saltarse el límite de memoria al parsear cabeceras MIME | go1.27.2 | Alcanzable en cualquier request |
| GO-2026-6617, 6612, 6611, 6610, 6603 | `net/http` (HTTP/2) | Caída, consumo de CPU y memoria en HTTP/2 | go1.27.2 | Bajo: las APIs hablan HTTP/1.1 sin TLS detrás de nginx (`proxy_http_version 1.1`) |
| GO-2026-6809, 6607 | `crypto/x509`, `crypto/tls` | Certificados intermedios comprometidos, ECH malformado | go1.27.3 / go1.27.2 | Bajo: solo en la conexión TLS a PostgreSQL externo |
| GO-2026-6629, 5970 | `golang.org/x/text` | Pánico o bucle infinito con entrada manipulada | v0.41.0 / v0.39.0 | Medio: `CanonicalCategorySlug` normaliza categorías que vienen de tiendas externas vía ingestion (`ingestion/processor.go:261`) |

**Matiz sobre la librería estándar:** `go.mod` declara `go 1.27.1`. La imagen
`golang:1.27-alpine` compila con el último parche 1.27.x **del momento del
build**, así que una imagen reconstruida probablemente ya esté corregida. El CI
(`go-version-file`) y cualquier build local con 1.27.1 no lo están, y la imagen
desplegada hoy depende de cuándo se construyó: no se verificó en el clúster.

**Recomendación:** subir `go.mod` a `go 1.27.4` (eso fija el mínimo también en
el CI) y `golang.org/x/text` a `v0.41.0` o superior; luego `go mod tidy`,
`go test ./...` y redeploy. Es un cambio de dependencias, sin código propio.

---

## 4. Controles verificados (positivos)

| Control | Evidencia |
|---|---|
| Todas las escrituras con `S` o `A` | Matriz §2 |
| Sin IDOR: filtro `user_id` de la sesión | `features/*.go`, `comparison/repository.go` |
| El gateway no publica ingestion, notifications, users ni pricing | `default.conf.template` |
| Catálogo de solo lectura en el gateway | `limit_except GET` |
| Body ≤ 1 MB, JSON estricto | `httpx/decode.go:14,49` |
| Paginación con máximo de 100 | `search`, `stores`, `categories`, `comparison`, `products` |
| Lote de ingestion acotado | `ingestion/handler.go:98` |
| Timeouts del servidor (5/15/35/60 s) y cabeceras ≤ 1 MB | `server/run.go:18-26` |
| `statement_timeout` 30 s, pool de 10 conexiones | `database/postgres.go:28,33` |
| Las URLs de ofertas solo admiten http/https (backend y frontend) | `ingestion/processor.go:320`, `pricing/handler.go:456`, `getSafeOfferUrl` |
| Sin `dangerouslySetInnerHTML` ni `eval` en el frontend | `grep` sobre `fronted/src` |
| Los logs no registran headers ni bodies | `httpx/logging.go:31` |
| Secrets generados con CSPRNG y fuera de git | `deploy.*`, `git ls-files` |
| Comparación de tokens en tiempo constante (Go y scraper) | `authz.go:89`, `app.py:121` |
| Scraper sin SSRF: solo URLs que reclama una spec | `api/manager.py:414` |
| Pods sin root, `readOnlyRootFilesystem`, sin escalada, sin token de SA | `k8s/base/**`, `k8s/components/**` |
| Dependencias del frontend | `pnpm audit`: «No known vulnerabilities found» |
| Dependencias de Python | `pip-audit 2.10.1`: «No known vulnerabilities found» en `scraper/` (proyecto), `crawler/` (proyecto) y `crawler/requirements.txt` |
| chi v5.3.2 y pgx v5.10.0 | `govulncheck`: sin vulnerabilidades propias |

**Nota sobre el scraper:** `scraper/pyproject.toml` no fija versiones
(`scrapling[fetchers]>=0.4`) y la imagen instala lo último en cada build.
`pip-audit` auditó esa misma resolución de hoy, pero el build no es
reproducible: una imagen futura puede traer una versión distinta.

---

## 5. Lo que no se ejecutó

| Verificación | Motivo |
|---|---|
| Versión de Go del binario desplegado | Requiere el clúster o la imagen publicada (H16) |
| Pruebas contra el clúster (`salud.py`, ráfaga real de logins) | La auditoría fue estática; los números de H1 son una medición local más el límite de CPU del manifiesto |
| Si la plataforma UCT admite NetworkPolicy | Requiere consultar el clúster (H4) |

---

## 6. Migraciones

Ningún hallazgo exige una migración: H5 y H6 se resuelven con las columnas
actuales de `user_session` (`004_platform_apis.sql`). Si al implementar se
quisiera un índice para la purga (por ejemplo, sobre `refresh_expires_at`), va
por **`feat/backend`**.

---

## 7. Cómo reproducir

```bash
# Rutas y middleware
grep -nE '\.(Get|Post|Put|Delete)\(|Use\(|With\(' backend/internal/http/router.go backend/internal/features/router.go

# Ausencia de rate limit
grep -rniE 'limit_req|ratelimit|limiter' backend docker k8s

# Costo de login y límite de CPU
grep -n passwordIterations backend/internal/features/auth.go
grep -nA6 'resources:' k8s/components/dominios/usuarios.yaml

# Cabeceras del gateway
grep -n add_header docker/frontend/default.conf.template

# Tokens en el navegador
grep -n localStorage fronted/src/services/api/personalization.ts

# Dependencias
cd fronted && pnpm audit
cd backend && govulncheck ./...
pip-audit scraper && pip-audit crawler && pip-audit -r crawler/requirements.txt
```
