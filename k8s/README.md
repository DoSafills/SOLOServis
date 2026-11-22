# SOLOServis en Kubernetes

Guía para desplegar SOLOServis desde tu máquina en tu namespace del clúster de la universidad (UCT). Cada comando aparece en **PowerShell** (Windows) y en **bash** (Linux, macOS o Git Bash).

## Contenido

1. [Qué se despliega](#1-qué-se-despliega)
2. [Requisitos](#2-requisitos)
3. [Desplegar en UCT](#3-desplegar-en-uct)
4. [Qué hace el script, fase por fase](#4-qué-hace-el-script-fase-por-fase)
5. [Pod de test](#5-pod-de-test)
6. [Operación del día a día](#6-operación-del-día-a-día)
7. [Ejemplos de uso de la app desplegada](#7-ejemplos-de-uso-de-la-app-desplegada)
8. [Solución de problemas](#8-solución-de-problemas)
9. [Estructura de archivos](#9-estructura-de-archivos)

---

## 1. Qué se despliega

Las 14 APIs se agrupan en **3 pods, uno por contexto de negocio**. Los tres usan la misma imagen (`backend/cmd/all-apis`); solo cambia la variable `API_DOMAIN`, que decide qué APIs sirve cada proceso (`backend/internal/allapis/domains.go`).

| Pod (Deployment + Service) | `API_DOMAIN` | APIs | Rutas que atiende |
|---|---|---|---|
| `comercial` | `comercial` | product, store, service, pricing, search, comparison, favorites, cart, review, watchlist, notification | `/products`, `/categories`, `/stores`, `/services`, `/offers`, `/search`, `/comparisons`, `/favorites`, `/cart`, `/reviews`, `/watchlist`, `/notifications` |
| `usuarios` | `usuarios` | auth, user | `/auth`, `/users` |
| `ingestion` | `ingestion` | ingestion | `/ingestion` (solo red interna, la usa el scraper) |

Además corren `frontend` (nginx con la app de Vite), `postgres` (StatefulSet con PVC) y `scraper-api` (Deployment con PVC y API de control). En total: **6 pods, 7 Services y 1 CronJob** de disparo.

```
 navegador
    │ https://soloservis-<usuario>.dev.censei.cl   (Ingress, solo path /)
    ▼
 frontend (nginx) ── /auth ───────────────────────────► usuarios   (auth, user)
                  ── /products, /cart, /search, ... ──► comercial  (11 APIs)
                                                            ▲
 scraper-api (Deployment + PVC) ──► ingestion ── POST /notifications ┘
        ▲
        │ POST /runs (CronJob scraper-trigger, 03:00 America/Santiago)

 comercial, usuarios e ingestion ──► postgres (StatefulSet + PVC)
```

**Scraper v2:** `scraper-api` no es una API pública ni habla PostgreSQL. Es un
Deployment de 1 réplica (`strategy: Recreate`) con un PVC `scraper-data` que
guarda `scraper.db`, el catálogo por tienda y las tareas recurrentes en
`/data/tasks`. Expone `:8095` solo dentro del clúster (`/health`, `/ready`,
`/stores`, `/runs`, `/tasks`). El daemon agenda sus propias tareas; el CronJob
`scraper-trigger` no crawlea, solo llama `POST /runs` por tienda y sale, y es
opcional una vez que cada tienda tiene tarea. El detalle del paquete está en
[`scraper/README.md`](../scraper/README.md).

**Cómo se reparten las llamadas:**
- Una ruta que no es del dominio responde **404** en ese pod (por ejemplo, `/auth/login` en `comercial`). El nginx manda cada ruta al pod correcto.
- Las llamadas HTTP entre APIs del mismo dominio (cart → product, cart → pricing, review → store, ...) van al propio pod (`127.0.0.1:8080`).
- La **única llamada entre pods** es ingestion → notification: `NOTIFICATION_API_URL=http://comercial:8080`. Si falta esa variable, ingestion **no arranca** y dice cuál falta.
- `comercial` lee las tablas de sesión (`user_session`) para validar el token: es una dependencia por la base, no por HTTP.

El nginx del frontend (`docker/frontend/default.conf.template`) enruta por path, porque la plataforma UCT solo admite el path `/` en el Ingress. El navegador usa siempre el mismo origen, así que no hay CORS. Estas son las rutas públicas:

| Ruta | Pod | Métodos |
|---|---|---|
| `/products`, `/product-categories`, `/categories`, `/stores`, `/services` | comercial | GET |
| `/auth` | usuarios | GET, POST |
| `/favorites`, `/cart`, `/search`, `/comparisons`, `/reviews`, `/watchlist` | comercial | todos (cada API valida la sesión) |

**Pricing, User, Notification e Ingestion no se publican.** Solo se llaman por la red interna del clúster (ver [§7](#7-ejemplos-de-uso-de-la-app-desplegada)).

**Topología de respaldo: `consolidado`.** Las 14 APIs en un solo Deployment `backend` (3 pods y 4 Services en total). Sirve si la cuota no alcanza: `-Topologia consolidado` / `--topologia consolidado`. Para el frontend y para quien llame a las APIs, las dos topologías son idénticas.

---

## 2. Requisitos

| Herramienta | Para qué | Comprobar |
|---|---|---|
| Docker Desktop (o Docker Engine en Linux) | construir y publicar las imágenes | `docker version` |
| kubectl ≥ 1.27 | hablar con el clúster (incluye kustomize) | `kubectl version --client` |
| git | clonar el repo y etiquetar imágenes | `git --version` |
| curl | verificaciones del script (bash) | `curl --version` |
| plugin `kubectl oidc-login` | iniciar sesión con tu cuenta institucional | `kubectl oidc-login --version` |

Docker Desktop ya trae `kubectl`. Si no lo tienes:

**PowerShell**

```powershell
winget install -e --id Docker.DockerDesktop
winget install -e --id Kubernetes.kubectl
winget install -e --id Git.Git
```

**bash** (macOS con Homebrew; en Linux usa el gestor de tu distribución)

```bash
brew install --cask docker
brew install kubectl git
```

Clona el repo (o actualízalo) y entra a la carpeta:

**PowerShell**

```powershell
git clone https://github.com/DoSafills/SOLOServis.git
cd SOLOServis
git switch develop
git pull
```

**bash**

```bash
git clone https://github.com/DoSafills/SOLOServis.git
cd SOLOServis
git switch develop
git pull
```

> **Windows:** si PowerShell bloquea el script (`running scripts is disabled`), habilítalo **solo para esa terminal**:
> `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass`

---

## 3. Desplegar en UCT

### 3.1. Tu kubeconfig

El kubeconfig de la plataforma inicia sesión con `kubectl oidc-login`. Si no tienes el plugin, `kubectl` responde `unknown command "oidc-login"`. Instálalo una sola vez:

**PowerShell** (como administrador, con [Chocolatey](https://chocolatey.org/install))

```powershell
choco install kubelogin
```

> Sin Chocolatey puedes usar krew: `winget install -e --id Kubernetes.krew`, luego `kubectl krew install oidc-login`, y agrega `$HOME\.krew\bin` al PATH. Ojo: el paquete `int128.kubelogin` de winget solo trae `kubelogin.exe`, sin el `kubectl-oidc_login.exe` que necesita `kubectl`.

**bash**

```bash
brew install kubelogin              # macOS
kubectl krew install oidc-login     # Linux (con krew instalado)
```

Abre una terminal nueva y comprueba:

```bash
kubectl oidc-login --version
```

Luego:

1. Entra a <https://estudiantes.dev.censei.cl> y descarga tu kubeconfig (`estudiantes-<usuario>.kubeconfig`).
2. Guárdalo en `~/.kube/`. **No lo subas a git:** `.gitignore` ya ignora `*.kubeconfig`.

**PowerShell**

```powershell
New-Item -ItemType Directory -Force "$HOME\.kube" | Out-Null
Move-Item "$HOME\Downloads\estudiantes-<usuario>.kubeconfig" "$HOME\.kube\"
$env:KUBECONFIG = "$HOME\.kube\estudiantes-<usuario>.kubeconfig"
kubectl auth can-i list pods
```

**bash**

```bash
mkdir -p ~/.kube
mv ~/Downloads/estudiantes-<usuario>.kubeconfig ~/.kube/
export KUBECONFIG=~/.kube/estudiantes-<usuario>.kubeconfig
kubectl auth can-i list pods
```

La primera vez, `kubectl` abre el navegador para iniciar sesión (OIDC). La respuesta esperada es `yes`. El namespace (`student-<usuario>`) sale del kubeconfig.

> `$env:KUBECONFIG` / `export KUBECONFIG` solo duran mientras la terminal está abierta. Para las demás secciones, ábrela así o pasa `--kubeconfig <ruta>` en cada comando.

### 3.2. Docker Hub

El clúster descarga las imágenes de un registry, así que necesitas una cuenta en <https://hub.docker.com>. Tu usuario de Docker Hub es el `<usuario-docker>` de los comandos.

```bash
docker login
```

Los repositorios `soloservis-frontend` y `soloservis-backend` se crean **públicos** con el primer `docker push`, y así el clúster puede descargarlos sin credenciales. Si prefieres que sean privados, mira [§8](#8-solución-de-problemas) (`ImagePullBackOff`).

### 3.3. Desplegar

**PowerShell**

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct `
    -Kubeconfig "$HOME\.kube\estudiantes-<usuario>.kubeconfig" `
    -Registry docker.io/<usuario-docker>
```

**bash**

```bash
./scripts/k8s/deploy.sh --modo uct \
    --kubeconfig ~/.kube/estudiantes-<usuario>.kubeconfig \
    --registry docker.io/<usuario-docker>
```

Despliega la topología `dominios`. Para la de respaldo agrega `-Topologia consolidado` / `--topologia consolidado`.

Al terminar, la app queda en `https://soloservis-<usuario>.dev.censei.cl` (la plataforma redirige `http` a `https`). El script verifica las rutas públicas desde tu máquina y corre el [pod de test](#5-pod-de-test) dentro del clúster. La primera vez, el DNS puede tardar unos minutos en propagarse.

> **Advertencia de certificado:** por ahora la plataforma usa un certificado HTTPS que el navegador no reconoce. Que aparezca el aviso es normal (§12 del manual de la plataforma) y no indica un error del despliegue: acéptalo para entrar. Con curl agrega `-k`.

**Lo que conviene saber:**
- **Solo inventario:** agrega `-ToPhase 0` / `--to-phase 0`. Muestra recursos, cuota y StorageClasses sin modificar nada.
- **Tus datos se conservan:** la Fase 1 borra Deployments, Services, Jobs e Ingress de un despliegue anterior (también el de la otra topología), pero **conserva el PVC de PostgreSQL y los Secrets**. El Job de migraciones solo aplica las que falten (las registra en `schema_migrations`).
- **Otro dominio:** usa `-IngressHost` / `--ingress-host`.

### 3.4. Cuota

```bash
kubectl get resourcequota
```

| | Cuota del namespace | `dominios` | `consolidado` |
|---|---|---|---|
| Pods | 12 | 5 | 3 |
| Services | 10 | 6 | 4 |
| CPU reservada (requests) | 1 | 225m | 175m |
| Memoria reservada (requests) | 1G | 576Mi | 448Mi |
| CPU / memoria máxima (limits) | 2 / 2G | 1050m / 1152Mi | 650m / 896Mi |

Requests por pod en `dominios`: comercial 50m/128Mi, usuarios 25m/64Mi, ingestion 25m/64Mi, frontend 25m/64Mi y postgres 100m/256Mi. Los Jobs (migraciones y pod de test) suman un pod más mientras corren. El LimitRange de la plataforma exige como mínimo 25m y 64Mi por contenedor.

---

## 4. Qué hace el script, fase por fase

`deploy.ps1` y `deploy.sh` hacen lo mismo. Cada fase muestra un checkpoint (`=== FASE N ===`) y, si algo falla, se detiene con el diagnóstico (describe, logs y eventos) y te dice desde qué fase reintentar.

| Fase | Qué hace | Cómo se verifica |
|---|---|---|
| 0 | Inventario: herramientas, acceso al clúster, recursos actuales, cuota | solo lectura |
| 1 | Borra el despliegue anterior. **Conserva el PVC y los Secrets** | no quedan recursos `part-of=soloservis` |
| 2 | Elige la StorageClass (prueba que un PVC quede Bound y escribible) y genera `k8s/overlays/uct` | `kubectl kustomize` compila |
| 3 | `docker build` y `docker push` de las 3 imágenes (frontend, backend y scraper) | cada imagen OK |
| 4 | Crea los Secrets `postgres-secret` (contraseña aleatoria), `soloservis-internal` (token) y `soloservis-ai` (desde `AI_API_KEY` del entorno) si no existen, aplica las NetworkPolicy si la plataforma lo permite y despliega PostgreSQL | `postgres-0` Ready |
| 5 | Job de migraciones (`migration_order.txt`) | `schema_migrations` = archivos del manifiesto |
| 6 | APIs: `comercial`, `usuarios` e `ingestion` (o `backend`), más `scraper-api` | cada API Ready y con `/health` → `database: connected`; scraper Ready y `/health` → `ok` |
| 7 | Frontend (nginx) | Ready y `/products` responde a través de nginx |
| 8 | Ingress | `/`, `/products`, `/categories`, `/stores`, `/services`, `/search` desde tu máquina |
| 9 | Prueba de resiliencia: borra un pod de API y reinicia PostgreSQL | el pod se recrea, los datos persisten, las APIs reconectan |
| 10 | [Pod de test](#5-pod-de-test) | todas las verificaciones OK |

**Parámetros:**

| PowerShell | bash | Uso |
|---|---|---|
| `-Topologia` | `--topologia` | `dominios` (por defecto) o `consolidado` |
| `-Modo uct` | `--modo uct` | opcional; se mantiene por compatibilidad (el modo local se retiró) |
| `-Kubeconfig` / `-Context` / `-Namespace` | `--kubeconfig` / `--context` / `--namespace` | por defecto: `$KUBECONFIG` o `~/.kube/config`, y el namespace del kubeconfig |
| `-Registry` | `--registry` | `docker.io/<usuario-docker>` |
| `-Tag` | `--tag` | por defecto: el commit actual (`git rev-parse --short HEAD`) |
| `-FromPhase` / `-ToPhase` | `--from-phase` / `--to-phase` | ejecutar solo un rango de fases (0–10) |
| `-SkipCleanup` | `--skip-cleanup` | no borrar nada en la Fase 1 |
| `-SinRed` | `--sin-red` | no aplicar las NetworkPolicy y borrar las de un despliegue anterior |
| `-StorageClass`, `-IngressClass`, `-IngressHost`, `-ImagePullSecret` | `--storage-class`, `--ingress-class`, `--ingress-host`, `--image-pull-secret` | casos especiales |

El script guarda el registry, el tag y la StorageClass elegida en `k8s/overlays/uct/` (carpeta ignorada por git). Así, al reintentar desde una fase intermedia, no tienes que repetirlos.

---

## 5. Pod de test

Un Job (`soloservis-smoke-test`, imagen `curlimages/curl`) que corre `k8s/tests/smoke-test.sh` **dentro del clúster** y termina. **No escribe nada en la base:** solo hace lecturas, y los dos POST van sin token y se rechazan antes de tocarla. Detecta solo la topología (dominios o consolidado). Son 29 verificaciones de las APIs en `dominios`, más hasta 6 del scraper cuando `scraper-api` está desplegado.

| Verificación | Qué espera |
|---|---|
| `/health` de `comercial`, `usuarios` e `ingestion` | 200, `"service"` con el nombre del dominio y `"database":"connected"` |
| Una lectura por API pública (product, store, service, pricing, search, comparison, review, user) | 200 con datos |
| Rutas con sesión o internas, sin token: `/favorites`, `/cart`, `/comparisons/products`, `/comparisons/services/1`, `POST /reviews/product/{id}`, `/watchlist?userId=1`, `/notifications?userId=1`, `POST /notifications`, `/auth/validate`, `/ingestion/runs` | 401: mandar un `userId` no basta (TAL-125) |
| Ruta interna con `INTERNAL_API_TOKEN`: `GET /ingestion/runs` | 200 |
| Aislamiento: `comercial` no sirve `/auth` ni `/ingestion`; `usuarios` e `ingestion` no sirven `/products` | 404 |
| nginx del frontend: `/` → app, `/products` → comercial, `/auth` → usuarios | 200, 200, 401 |
| Scraper: `/health`, `/ready` | 200 y `"status"` esperado |
| Scraper sin token: `/stores`, `/runs` | 401 |
| Scraper con `INTERNAL_API_TOKEN`: `/stores`, `/runs` | 200 |

Lo corre la Fase 10 del deploy. Para correrlo solo:

**PowerShell**

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -FromPhase 10
```

**bash**

```bash
./scripts/k8s/deploy.sh --modo uct --from-phase 10
```

O a mano con `kubectl` (los Jobs son inmutables: se borra antes de repetirlo):

```bash
kubectl delete job soloservis-smoke-test --ignore-not-found
```

```bash
kubectl apply -k k8s/tests
```

```bash
kubectl wait --for=condition=complete job/soloservis-smoke-test --timeout=180s
```

```bash
kubectl logs job/soloservis-smoke-test
```

Cada línea dice `OK` o `FALLA` con el código HTTP y la URL; la última es `Resultado: N OK, M fallas`. El Job termina en `Complete` si todo pasa y en `Failed` si algo falla. Se borra solo una hora después.

### Visor de llamadas

Otro Job, aparte de la fase 10: `soloservis-llamadas` (`k8s/tests/llamadas`). Simula el uso de las APIs y el flujo scraper → ingestion → catálogo → gateway, y muestra cada llamada en dos ventanas:

- `origen` corre los escenarios y muestra lo que envía, lo que le vuelve y si coincide con lo esperado.
- `destino` es un sidecar que hace de proxy: muestra lo que le llega a cada servicio y lo que responde.

Cada llamada lleva un `X-Request-ID` propio (`t165-05-productos-pagina`), el mismo en las dos ventanas y en el AccessLog del pod real. Los escenarios son:

- TAL-165: parámetros de cada API, la misma llamada por el gateway y directo, lo que el gateway bloquea, autenticación y 10 lecturas simultáneas.
- TAL-166: valores válidos, inválidos, incompletos y límite, y la forma de cada salida (problem+json, `request_id`, camelCase, sin detalles internos).
- TAL-167: la última corrida real de cada tienda, del scraper a Ingestion API, al catálogo y al gateway.

**No escribe en la base:** las únicas escrituras son pedidos que Ingestion API y Auth API rechazan al decodificar o validar, antes de la consulta. El token de servicio sale de `soloservis-internal` y no se imprime. Ocupa un pod con 2 contenedores de 25m/64Mi (límite 100m/64Mi).

```bash
python scripts/testing/llamadas.py --ventanas
```

Recrea el Job, abre dos PowerShell (`ORIGEN` y `DESTINO`) con `kubectl logs -f` y, al terminar, deja en `docs/reportes/testing/llamadas-<fecha>.md` cada llamada lado a lado, con la línea del AccessLog del backend o del nginx que tiene el mismo id. `--tickets 166` corre uno solo y `--pausa 1` hace más lenta la secuencia. A mano:

```bash
kubectl delete job soloservis-llamadas --ignore-not-found
```

```bash
kubectl apply -k k8s/tests/llamadas
```

```bash
kubectl logs -f job/soloservis-llamadas -c origen
```

```bash
kubectl logs -f job/soloservis-llamadas -c destino
```

---

## 6. Operación del día a día

Los ejemplos usan `deployment/comercial`. Cambia el nombre por `usuarios`, `ingestion`, `frontend` o, en la topología consolidada, `backend`.

### Re-desplegar después de cambiar código

Cambiaste código y quieres verlo en el clúster: construye imágenes nuevas (Fase 3) y re-despliega sin borrar nada.

**PowerShell**

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -FromPhase 3 -SkipCleanup
```

**bash**

```bash
./scripts/k8s/deploy.sh --modo uct --from-phase 3 --skip-cleanup
```

> El tag de las imágenes es el commit actual, más la fecha si hay cambios sin commitear. Así cada build produce un tag nuevo y Kubernetes siempre reemplaza los pods.

Si agregaste una **migración nueva**, agrégala también a `backend/database/migration_order.txt`. La Fase 5 la aplica.

### Ver el estado

```bash
kubectl get pods
```

```bash
kubectl get deploy,svc,pvc,ingress
```

### Logs

Últimas 50 líneas de un pod de API, siguiendo en vivo (Ctrl+C para salir):

```bash
kubectl logs deployment/comercial --tail 50 -f
```

Los logs de las APIs son JSON (uno por request, con `request_id`). Para buscar errores:

**PowerShell**

```powershell
kubectl logs deployment/comercial --tail 500 | Select-String '"level":"ERROR"'
```

**bash**

```bash
kubectl logs deployment/comercial --tail 500 | grep '"level":"ERROR"'
```

### Scraper: disparar, mirar y detener

El scraper no se publica: se llega por `port-forward` (o desde el backend).

```bash
kubectl port-forward svc/scraper-api 8095:8095
TOKEN="$(kubectl get secret soloservis-internal -o jsonpath='{.data.INTERNAL_API_TOKEN}' | base64 -d)"
curl -sS -X POST http://127.0.0.1:8095/runs \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"store":"ripley-cl"}'
curl -sS http://127.0.0.1:8095/runs/1 -H "Authorization: Bearer $TOKEN"
curl -sS -X POST http://127.0.0.1:8095/runs/1/stop -H "Authorization: Bearer $TOKEN"
```

Crear una tarea recurrente (el daemon la agenda; sobrevive al pod porque vive
en `/data/tasks`):

```bash
curl -sS -X POST http://127.0.0.1:8095/tasks \
  -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' \
  -d '{"store":"ripley-cl","schedule":{"kind":"interval","every":"1h"}}'
curl -sS http://127.0.0.1:8095/tasks -H "Authorization: Bearer $TOKEN"
```

Freno de emergencia para todas las tareas y runs activos:

```bash
curl -sS -X POST http://127.0.0.1:8095/tasks/stop -H "Authorization: Bearer $TOKEN"
curl -sS -X POST http://127.0.0.1:8095/tasks/start -H "Authorization: Bearer $TOKEN"
```

El CronJob queda como disparo opcional. Si todas las tiendas ya tienen tarea,
se puede quitar de `k8s/base/kustomization.yaml`; mientras conviva, un trigger
que choque con una tarea recibe `409` y no duplica el crawl.

Disparar el CronJob sin esperar la hora:

```bash
kubectl create job --from=cronjob/scraper-trigger "scraper-manual-$(date +%s)"
kubectl logs job/scraper-trigger --tail 50   # o el nombre del Job creado
```

Ver el estado y el catálogo:

```bash
kubectl logs deployment/scraper-api --tail 100 -f
kubectl exec deployment/scraper-api -- ls -la /data/catalog
```

Un `409` significa que esa tienda ya tiene un run `pending`/`running`; es la
protección contra ejecuciones duplicadas, no un error del despliegue.

### Reiniciar un pod de API

```bash
kubectl rollout restart deployment/comercial
```

```bash
kubectl rollout status deployment/comercial
```

### Volver a la versión anterior (rollback)

```bash
kubectl rollout history deployment/comercial
```

```bash
kubectl rollout undo deployment/comercial
```

### Consola de PostgreSQL

```bash
kubectl exec -it postgres-0 -- psql -U postgres -d soloservis
```

Dentro de `psql`: `\dt` lista las tablas, `SELECT version FROM schema_migrations;` muestra las migraciones aplicadas y `\q` sale.

Para conectarte desde DBeaver, pgAdmin u otro cliente: abre un port-forward y conéctate a `localhost:15432` (usuario `postgres`, base `soloservis`). La contraseña está en el Secret `postgres-secret` (ver [Contraseña de PostgreSQL](#contraseña-de-postgresql)).

```bash
kubectl port-forward svc/postgres 15432:5432
```

### Contraseña de PostgreSQL

No está en git: la Fase 4 crea `postgres-secret` con una contraseña aleatoria la primera vez y después lo reutiliza. PostgreSQL guarda la contraseña dentro del volumen al crear la base, así que **cambiar solo el Secret no basta**.

Leerla sin mostrarla en pantalla (la copia al portapapeles):

```powershell
[Text.Encoding]::UTF8.GetString([Convert]::FromBase64String((kubectl get secret postgres-secret -o jsonpath='{.data.POSTGRES_PASSWORD}'))) | Set-Clipboard
```

Cambiarla (la base, el Secret y las APIs, en ese orden y seguido):

```powershell
$b = New-Object byte[] 24
[Security.Cryptography.RandomNumberGenerator]::Create().GetBytes($b)
$pw = ([BitConverter]::ToString($b) -replace '-', '').ToLower()
"ALTER USER postgres WITH PASSWORD '$pw';" | kubectl exec -i postgres-0 -c postgres -- sh -c 'psql -X -q -v ON_ERROR_STOP=1 -U $POSTGRES_USER -d $POSTGRES_DB'
kubectl create secret generic postgres-secret --from-literal=POSTGRES_USER=postgres --from-literal=POSTGRES_DB=soloservis "--from-literal=POSTGRES_PASSWORD=$pw" --dry-run=client -o yaml | kubectl apply -f -
kubectl rollout restart deployment/comercial deployment/usuarios deployment/ingestion
Remove-Variable pw, b
```

Si borras `postgres-secret` pero conservas el PVC, la Fase 4 se detiene: una contraseña nueva no coincidiría con la del volumen. Recréalo con la contraseña anterior, o borra también el PVC si no necesitas los datos.

### Clave del proveedor de IA

El asistente de comparación necesita una clave que **emite un tercero** (DeepSeek en el clúster), así que la Fase 4 no puede generarla. Vive en el Secret `soloservis-ai`, fuera de git, y la lee el dominio `comercial`.

Lo normal es exportarla antes de desplegar y dejar que la Fase 4 cree el Secret:

```powershell
$env:AI_API_KEY = 'sk-...'     # PowerShell
```
```bash
export AI_API_KEY='sk-...'     # bash
```

Si el Secret ya existe, la Fase 4 lo reutiliza y **no** lee `AI_API_KEY`. Si no existe y la variable tampoco, el despliegue **no se detiene**: avisa y el asistente responde `503 ai_unavailable`; el resto del sitio funciona igual.

Para crearlo o rotarlo a mano, sin dejar la clave en el historial:

```bash
read -rs AI_API_KEY && printf '%s' "$AI_API_KEY" > /tmp/ai.key
kubectl create secret generic soloservis-ai --from-file=AI_API_KEY=/tmp/ai.key --dry-run=client -o yaml | kubectl apply -f -
rm -f /tmp/ai.key && unset AI_API_KEY
kubectl rollout restart deployment/comercial
```

Cambiar de proveedor (Gemini, Groq, Ollama) no exige recompilar: se editan `AI_PROVIDER`, `AI_BASE_URL` y `AI_MODEL` en `k8s/components/dominios/comercial.yaml`, se actualiza el Secret y se reinicia el deployment. Nunca imprimas el Secret: `kubectl get secret -o yaml` devuelve la clave en base64, que es reversible.

### Borrar todo

En tu namespace no puedes borrar el namespace, así que se borran los recursos. ⚠️ Esto incluye la base de datos:

```bash
kubectl delete all,ingress,configmap,job,pvc -l app.kubernetes.io/part-of=soloservis
```

```bash
kubectl delete secret postgres-secret soloservis-internal soloservis-ai --ignore-not-found
```

---

## 7. Ejemplos de uso de la app desplegada

En los ejemplos, `$BASE` es la URL de la app:

**PowerShell**

```powershell
$BASE = "https://soloservis-<usuario>.dev.censei.cl"
```

**bash**

```bash
BASE=https://soloservis-<usuario>.dev.censei.cl
```

> En PowerShell usa siempre `curl.exe`, no `curl`, porque `curl` es un alias de `Invoke-WebRequest`.
>
> El certificado HTTPS aún no es de confianza (ver [§3.3](#33-desplegar)): agrega `-k` a cada `curl`/`curl.exe`. Para `Invoke-RestMethod` en PowerShell 5.1, desactiva la verificación **solo en esa terminal y solo para estas pruebas**:
> `[Net.ServicePointManager]::ServerCertificateValidationCallback = { $true }`

### Consultas públicas

**PowerShell**

```powershell
curl.exe -s -k "$BASE/categories"
curl.exe -s -k "$BASE/categories/refrigeradores"
curl.exe -s -k "$BASE/search/products?q=refrigerador&limit=3"
curl.exe -s -k "$BASE/search/facets?category=refrigeradores"
curl.exe -s -k "$BASE/stores"
```

**bash**

```bash
curl -s -k "$BASE/categories"
curl -s -k "$BASE/categories/refrigeradores"
curl -s -k "$BASE/search/products?q=refrigerador&limit=3"
curl -s -k "$BASE/search/facets?category=refrigeradores"
curl -s -k "$BASE/stores"
```

### Registro y sesión

`/auth` lo atiende el pod `usuarios`; `/comparisons` y `/watchlist` los atiende `comercial`, que valida el mismo token leyendo `user_session`. Las rutas con datos de un usuario toman el usuario del token: un `userId` en la URL o en el body se ignora.

**PowerShell**

```powershell
# Invoke-RestMethod para el POST: PowerShell 5.1 le quita las comillas al JSON si se lo pasas a curl.exe.
$body = '{"name":"Ana Prueba","email":"ana@test.cl","password":"Clave12345!"}'
$session = Invoke-RestMethod -Method Post -Uri "$BASE/auth/register" -ContentType 'application/json' -Body $body
$TOKEN = $session.accessToken
curl.exe -s -k -H "Authorization: Bearer $TOKEN" "$BASE/auth/validate"
curl.exe -s -k -H "Authorization: Bearer $TOKEN" "$BASE/comparisons/products"
curl.exe -s -k -H "Authorization: Bearer $TOKEN" "$BASE/watchlist"
```

**bash**

```bash
TOKEN=$(curl -s -k -X POST -H 'Content-Type: application/json' \
  -d '{"name":"Ana Prueba","email":"ana@test.cl","password":"Clave12345!"}' \
  "$BASE/auth/register" | sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p')
curl -s -k -H "Authorization: Bearer $TOKEN" "$BASE/auth/validate"
curl -s -k -H "Authorization: Bearer $TOKEN" "$BASE/comparisons/products"
curl -s -k -H "Authorization: Bearer $TOKEN" "$BASE/watchlist"
```

Si el correo ya existe, usa `/auth/login` con `{"email": ..., "password": ...}`.

### APIs internas (Pricing, Ingestion, User, Notification)

No están publicadas. Para probarlas, abre un port-forward al pod de su dominio:

| API | Service |
|---|---|
| pricing, notification | `svc/comercial` |
| user | `svc/usuarios` |
| ingestion | `svc/ingestion` |

En la topología consolidada, todas están en `svc/backend`.

**PowerShell**

```powershell
kubectl port-forward svc/ingestion 8094:8080
```

En otra terminal, lee el token de servicio y consulta las ejecuciones del scraper:

```powershell
$raw = kubectl get secret soloservis-internal -o jsonpath='{.data.INTERNAL_API_TOKEN}'
$INTERNAL = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($raw))
curl.exe -s -H "Authorization: Bearer $INTERNAL" "http://localhost:8094/ingestion/runs"
```

**bash**

```bash
kubectl port-forward svc/ingestion 8094:8080
```

```bash
INTERNAL=$(kubectl get secret soloservis-internal -o jsonpath='{.data.INTERNAL_API_TOKEN}' | base64 -d)
curl -s -H "Authorization: Bearer $INTERNAL" "http://localhost:8094/ingestion/runs"
```

El contrato completo para el scraper está en `backend/README.md`, §12.7. **No compartas el token:** permite escribir en el catálogo.

---

## 8. Solución de problemas

Antes de buscar en la tabla, corre `python scripts/testing/salud.py`: en una pasada revisa pods, `/health` de cada proceso, gateway, host público, migraciones y cuota, y dice qué falla. Es de solo lectura.

| Síntoma | Causa probable | Qué hacer |
|---|---|---|
| `unknown command "oidc-login" for "kubectl"` | falta el plugin de login de la plataforma | instálalo como en [§3.1](#31-tu-kubeconfig) y abre una terminal nueva |
| Fase 0: `sin acceso al namespace` | kubeconfig vencido o login OIDC no completado | descarga de nuevo el kubeconfig y corre `kubectl auth can-i list pods` |
| `kubectl get nodes` / `get namespaces`: `Forbidden` | la plataforma no permite ver recursos del clúster | es esperado; trabaja dentro de tu namespace |
| Navegador: "la conexión no es privada"; curl: error 60 / `SEC_E_UNTRUSTED_ROOT` | la plataforma aún no emite certificados de confianza | acepta el aviso; con curl usa `-k` |
| Fase 3: `docker push` denegado | no hiciste `docker login` o el usuario del registry no es el tuyo | `docker login` y `-Registry docker.io/<tu-usuario>` |
| Fase 3: `Cannot connect to the Docker daemon` | Docker Desktop no está abierto | ábrelo y corre desde la Fase 3 |
| Fase 6: `ImagePullBackOff` | repositorio privado en Docker Hub | hazlo público, o crea `kubectl create secret docker-registry regcred --docker-username=<u> --docker-password=<token>` y usa `-ImagePullSecret regcred` |
| Fase 6: pods `Pending` con `exceeded quota` | otro despliegue tuyo ocupa la cuota | `kubectl get pods,resourcequota`; borra lo que no uses, corre desde la Fase 1 o usa `-Topologia consolidado` |
| Fase 6: `CrashLoopBackOff` | la API no conecta a la base o falla al arrancar | `kubectl logs deployment/<pod> --previous` |
| Fase 6: ingestion termina con `faltan URLs de APIs de otro dominio` | falta `NOTIFICATION_API_URL` en su Deployment | revisa `k8s/components/dominios/ingestion.yaml` |
| Fase 5: `faltan migraciones` | un `.sql` nuevo no está en `migration_order.txt` | agrégalo al manifiesto y corre desde la Fase 5 |
| Cualquier fase: `texto mal codificado en N lineas del render` | un archivo trae tildes dañadas (`Educaci├│n`, `EducaciÃ³n`), o la terminal leyó la salida de `kubectl` con otra página de códigos. El script se detiene antes de aplicar | revisa las líneas que muestra: si están así en git, corrige el archivo; si en git están bien, ejecuta el script con PowerShell 5.1 o 7 sin cambiar la codificación de la consola |
| Fase 7: el frontend no arranca (`host not found in upstream`) | falta el Service de algún pod de API | corre desde la Fase 6 (crea las APIs antes que el frontend) |
| Fase 8: `sin respuesta: DNS o red` | el DNS aún no se propaga | espera unos minutos y abre la URL en el navegador |
| Fase 10: líneas `FALLA` | una ruta respondió otro código | el log muestra la URL y el inicio de la respuesta; revisa `kubectl logs deployment/<pod>` |
| PowerShell: `running scripts is disabled` | política de ejecución | `Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass` |
| bash: `/usr/bin/env: 'bash\r'` | el script quedó con finales de línea CRLF | `git checkout -- scripts/k8s/deploy.sh` (`.gitattributes` lo fuerza a LF) |

Para diagnosticar cualquier pod:

```bash
kubectl describe pod <pod>
```

```bash
kubectl get events --sort-by=.lastTimestamp
```

---

## 9. Estructura de archivos

```
k8s/
├── base/                         común a las dos topologías
│   ├── postgres/                 Services y StatefulSet + PVC (el Secret lo crea el script)
│   ├── migrations/               Job + migrate.sh (aplica migration_order.txt y registra schema_migrations)
│   ├── frontend/                 Deployment (nginx sin root, solo lectura) + Service
│   ├── scraper/                  Deployment + PVC + Service + ConfigMap + CronJob de disparo
│   ├── ingress.yaml              host *.dev.censei.cl
│   └── kustomization.yaml
├── components/
│   ├── dominios/                 comercial, usuarios e ingestion (Deployment + Service) + destinos del nginx
│   ├── consolidado/              Deployment "backend" con las 14 APIs + destino del nginx (respaldo)
│   ├── espera-postgres/          initContainer que espera a PostgreSQL; lo incluyen las dos topologías
│   └── red/                      NetworkPolicy: niega todo y abre solo lo que la app usa (si hay permiso)
├── tests/                        pod de test: Job + smoke-test.sh (solo lecturas)
│   └── llamadas/                 visor de llamadas: Job con origen y destino
└── overlays/uct/                 GENERADO por el script (ignorado por git): namespace, imágenes,
                                  StorageClass, host y los .sql de backend/database/migrations
scripts/k8s/deploy.ps1            despliegue por fases (PowerShell 5.1+)
scripts/k8s/deploy.sh             el mismo despliegue en bash (Linux, macOS, Git Bash)
docker/all-apis/Dockerfile        imagen del backend (cmd/all-apis); la usan los 3 pods de API
docker/frontend/                  build de Vite + nginx (default.conf.template, proxy.conf, security-headers.conf)
docker/scraper/Dockerfile         imagen del scraper v2 (paquete scraper/ + specs)
docker/<api>/Dockerfile           imagen de cada API por separado (solo docker/compose.microservicios.yml)
```

**Seguridad:**
- **Postgres:** la contraseña de `postgres-secret` la genera el script al azar en cada namespace y no está en git. Solo es accesible dentro del clúster (Service `ClusterIP`, el Ingress publica solo el frontend). `docker/compose.yml` sigue con credenciales de desarrollo, solo para uso local.
- **Token interno:** el token de `soloservis-internal` lo genera el script al azar en cada namespace y no está en git. El pod de test lo usa sin imprimirlo.
- **Orden de arranque:** cada pod de API tiene un initContainer (`espera-postgres`, con `pg_isready`) y queda en `Init` hasta que PostgreSQL acepta conexiones. Además, la API hace ping a la base al iniciar y su readiness (`/health`) exige la conexión. No suma a la cuota: corre antes que el contenedor principal y pide menos que él.
- **Pods:** todos corren sin root, con `readOnlyRootFilesystem` y sin capabilities. Cumplen Pod Security `restricted`.
- **Red (TAL-197):** `components/red` niega todo el tráfico por defecto y permite solo Ingress → frontend, frontend → APIs, APIs → PostgreSQL, scraper → ingestion, ingestion → comercial, el CronJob y los Jobs hacia lo que usan, DNS y la salida a internet del scraper (y de comercial, para el proveedor de IA). El script la aplica solo si `kubectl auth can-i create networkpolicies` responde `yes`. **Hoy UCT responde `no`**: el rol del namespace solo puede leerlas, así que se omiten y la Fase 0 lo informa. Si algún día se aplican y algo deja de responder, `-SinRed` las borra en el siguiente deploy.
- **Gateway (TAL-197):** nginx envía CSP, `X-Content-Type-Options`, `X-Frame-Options`, `Referrer-Policy` y `Permissions-Policy` (`docker/frontend/security-headers.conf`), oculta su versión y rechaza con 400 las rutas con `%2F`, `%5C` o segmentos `..`. HSTS lo pone el Ingress de UCT.
