# SOLOServis en Kubernetes

Guía para desplegar SOLOServis desde tu máquina, en tu propio Kubernetes local o en tu namespace del clúster de la universidad (UCT). Cada comando aparece en **PowerShell** (Windows) y en **bash** (Linux, macOS o Git Bash).

## Contenido

1. [Qué se despliega](#1-qué-se-despliega)
2. [Requisitos](#2-requisitos)
3. [Modo local (Docker Desktop)](#3-modo-local-docker-desktop)
4. [Modo UCT (clúster de la universidad)](#4-modo-uct-clúster-de-la-universidad)
5. [Qué hace el script, fase por fase](#5-qué-hace-el-script-fase-por-fase)
6. [Operación del día a día](#6-operación-del-día-a-día)
7. [Ejemplos de uso de la app desplegada](#7-ejemplos-de-uso-de-la-app-desplegada)
8. [Solución de problemas](#8-solución-de-problemas)
9. [Estructura de archivos](#9-estructura-de-archivos)

---

## 1. Qué se despliega

La misma aplicación se puede desplegar de dos formas. Solo cambia cómo se agrupan las APIs:

| | **Modo `local`** | **Modo `uct`** |
|---|---|---|
| Dónde | Kubernetes de Docker Desktop, en tu máquina | Tu namespace `student-<usuario>` en la plataforma UCT |
| APIs | **14 Deployments**, uno por API (microservicios, igual que `docker/compose.yml`) | **1 Deployment `backend`** con las 14 APIs en un solo proceso (`backend/cmd/all-apis`) |
| Imágenes | Se construyen en tu máquina; no hace falta registry | Se publican en Docker Hub (`docker.io/<tu-usuario>`) |
| Cómo se entra | `kubectl port-forward` → <http://localhost:8080> | Ingress → `http://soloservis-<usuario>.dev.censei.cl` |
| Pods / Services | 16 / 17 | 3 / 4 |

**¿Por qué dos topologías?** La cuota de cada namespace UCT es de **12 pods, 10 Services, 1 CPU y 1 GB de memoria reservada**, y 14 APIs separadas no caben. `all-apis` sirve las mismas rutas con el mismo código: cada request llega sin cambios al router de su API. Para el frontend y para quien llame a las APIs, las dos topologías son idénticas.

```
Modo local (microservicios)                    Modo uct (consolidado)

 navegador                                      navegador
    │ kubectl port-forward :8080                   │ Ingress (solo path /)
    ▼                                              ▼
 frontend (nginx) ──/products──► product-api    frontend (nginx) ──/products, /auth, ...──► backend
                  ──/auth──────► auth-api                                                (14 APIs)
                  ──/cart──────► cart-api                                                    │
                  ...  (una API por ruta)                                                    ▼
                         │                                                               postgres
                         ▼                                                              (StatefulSet
                      postgres (StatefulSet + PVC)                                       + PVC)
```

El nginx del frontend (`docker/frontend/default.conf.template`) enruta por path. La plataforma UCT solo admite el path `/` en el Ingress, y el navegador usa siempre el mismo origen, así que no hay CORS. Estas son las rutas públicas:

| Ruta | API | Métodos |
|---|---|---|
| `/products`, `/product-categories`, `/categories` | Product | GET |
| `/stores` | Store | GET |
| `/services` | Service | GET |
| `/auth` | Auth | GET, POST |
| `/favorites`, `/cart`, `/search`, `/comparisons`, `/reviews`, `/watchlist` | la de su nombre | todos (cada API valida la sesión) |

**Pricing, User, Notification e Ingestion no se publican.** Solo se llaman por la red interna del clúster (ver [§7](#7-ejemplos-de-uso-de-la-app-desplegada)).

---

## 2. Requisitos

| Herramienta | Para qué | Comprobar |
|---|---|---|
| Docker Desktop (o Docker Engine en Linux) | construir imágenes (y Kubernetes local) | `docker version` |
| kubectl ≥ 1.27 | hablar con el clúster (incluye kustomize) | `kubectl version --client` |
| git | clonar el repo y etiquetar imágenes | `git --version` |
| curl | verificaciones del script (bash) | `curl --version` |

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

## 3. Modo local (Docker Desktop)

### 3.1. Activar Kubernetes

1. Abre Docker Desktop → **Settings → Kubernetes**.
2. Marca **Enable Kubernetes**. Si te pregunta por el tipo de clúster, elige **kubeadm**: con ese tipo, el clúster ve las imágenes que construyes con `docker build` sin publicarlas.
3. **Apply & restart** y espera a que el indicador de Kubernetes quede en verde (2 a 5 minutos la primera vez).
4. Recomendado: en **Settings → Resources**, asigna al menos **4 CPU y 6 GB de memoria**.

Comprueba que el contexto existe y responde:

```bash
kubectl config get-contexts
```

```bash
kubectl --context docker-desktop get nodes
```

Deberías ver el nodo `docker-desktop` en estado `Ready`.

### 3.2. Desplegar

**PowerShell**

```powershell
.\scripts\k8s\deploy.ps1 -Modo local
```

**bash**

```bash
./scripts/k8s/deploy.sh --modo local
```

La primera vez tarda entre 5 y 10 minutos, porque construye 15 imágenes. Cada fase termina con un checkpoint (`=== FASE N ===`). Si algo falla, el script se detiene, muestra la causa y te dice desde qué fase reintentar.

### 3.3. Abrir la app

Deja esta terminal abierta mientras uses la app:

```bash
kubectl --context docker-desktop -n soloservis port-forward svc/frontend 8080:80
```

Abre <http://localhost:8080>.

> Para no repetir `--context docker-desktop -n soloservis` en cada comando, deja ese namespace por defecto (solo cambia tu `~/.kube/config` local):
>
> ```bash
> kubectl config use-context docker-desktop
> ```
>
> ```bash
> kubectl config set-context --current --namespace soloservis
> ```

### 3.4. Apagar o borrar

Para borrar todo (incluida la base de datos local):

```bash
kubectl --context docker-desktop delete namespace soloservis
```

Para apagar Kubernetes sin borrar nada, desmarca **Enable Kubernetes** en Docker Desktop.

---

## 4. Modo UCT (clúster de la universidad)

### 4.1. Tu kubeconfig

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

### 4.2. Docker Hub

El clúster descarga las imágenes de un registry, así que necesitas una cuenta en <https://hub.docker.com>. Tu usuario de Docker Hub es el `<usuario-docker>` de los comandos.

```bash
docker login
```

Los repositorios `soloservis-frontend` y `soloservis-backend` se crean **públicos** con el primer `docker push`, y así el clúster puede descargarlos sin credenciales. Si prefieres que sean privados, mira [§8](#8-solución-de-problemas) (`ImagePullBackOff`).

### 4.3. Desplegar

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

Al terminar, la app queda en `http://soloservis-<usuario>.dev.censei.cl`. El script lo imprime y verifica las rutas públicas desde tu máquina. La primera vez, el DNS puede tardar unos minutos en propagarse.

**Lo que conviene saber:**
- **Solo inventario:** agrega `-ToPhase 0` / `--to-phase 0`. Muestra recursos, cuota y StorageClasses sin modificar nada.
- **Tus datos se conservan:** la Fase 1 borra Deployments, Services, Jobs e Ingress de un despliegue anterior, pero **conserva el PVC de PostgreSQL y los Secrets**. El Job de migraciones solo aplica las que falten (las registra en `schema_migrations`).
- **Otro dominio:** usa `-IngressHost` / `--ingress-host`.

### 4.4. Cuota

```bash
kubectl get resourcequota
```

El modo `uct` ocupa unos 175m de CPU y 450Mi de memoria reservados, y 650m de CPU y 900Mi de límite: alrededor de la mitad de la cuota. Si la universidad la amplía (al menos 18 pods y 18 Services), puedes desplegar microservicios también ahí con `-Topologia microservicios` / `--topologia microservicios`.

---

## 5. Qué hace el script, fase por fase

`deploy.ps1` y `deploy.sh` hacen lo mismo. Cada fase muestra un checkpoint y, si algo falla, se detiene con el diagnóstico (describe, logs y eventos).

| Fase | Qué hace | Cómo se verifica |
|---|---|---|
| 0 | Inventario: herramientas, acceso al clúster, recursos actuales, cuota | solo lectura |
| 1 | Crea el namespace (local) y borra el despliegue anterior. **Conserva el PVC y los Secrets** | no quedan recursos `part-of=soloservis` |
| 2 | Elige la StorageClass (UCT: prueba que un PVC quede Bound y escribible) y genera `k8s/overlays/<modo>` | `kubectl kustomize` compila |
| 3 | `docker build` de las imágenes (y `docker push` en UCT) | cada imagen OK |
| 4 | Crea el Secret `soloservis-internal` (token) si no existe y despliega PostgreSQL | `postgres-0` Ready |
| 5 | Job de migraciones (`migration_order.txt`) | `schema_migrations` = archivos del manifiesto |
| 6 | APIs (14 Deployments o `backend`) | cada una Ready y con `/health` → `database: connected` |
| 7 | Frontend (nginx) | Ready y `/products` responde a través de nginx |
| 8 | Acceso: Ingress (UCT) o port-forward temporal (local) | `/`, `/products`, `/categories`, `/stores`, `/services`, `/search` desde tu máquina |
| 9 | Prueba de resiliencia: borra un pod de API y reinicia PostgreSQL | el pod se recrea, los datos persisten, las APIs reconectan |

**Parámetros:**

| PowerShell | bash | Uso |
|---|---|---|
| `-Modo local\|uct` | `--modo local\|uct` | obligatorio |
| `-Topologia` | `--topologia` | `microservicios` o `consolidado` (por defecto, según el modo) |
| `-Kubeconfig` / `-Context` / `-Namespace` | `--kubeconfig` / `--context` / `--namespace` | por defecto: `~/.kube/config`, `docker-desktop` y `soloservis` en local; en UCT, el namespace del kubeconfig |
| `-Registry` | `--registry` | UCT: `docker.io/<usuario-docker>` |
| `-Tag` | `--tag` | por defecto: el commit actual (`git rev-parse --short HEAD`) |
| `-FromPhase` / `-ToPhase` | `--from-phase` / `--to-phase` | ejecutar solo un rango de fases (0–9) |
| `-SkipCleanup` | `--skip-cleanup` | no borrar nada en la Fase 1 |
| `-LocalPort` | `--local-port` | puerto de la prueba de la Fase 8 (local, por defecto 8080) |
| `-StorageClass`, `-IngressClass`, `-IngressHost`, `-ImagePullSecret` | `--storage-class`, `--ingress-class`, `--ingress-host`, `--image-pull-secret` | casos especiales (UCT) |

El script guarda el registry, el tag y la StorageClass elegida en `k8s/overlays/<modo>/` (carpeta ignorada por git). Así, al reintentar desde una fase intermedia, no tienes que repetirlos.

---

## 6. Operación del día a día

Los ejemplos de esta sección usan el modo local. En UCT, quita `--context docker-desktop -n soloservis` (el kubeconfig ya fija el namespace) y cambia `deployment/product-api` por `deployment/backend`.

### Re-desplegar después de cambiar código

Cambiaste código y quieres verlo en el clúster: construye imágenes nuevas (Fase 3) y re-despliega sin borrar nada.

**PowerShell**

```powershell
.\scripts\k8s\deploy.ps1 -Modo local -FromPhase 3 -SkipCleanup
```

**bash**

```bash
./scripts/k8s/deploy.sh --modo local --from-phase 3 --skip-cleanup
```

> El tag de las imágenes es el commit actual, más la fecha si hay cambios sin commitear. Así cada build produce un tag nuevo y Kubernetes siempre reemplaza los pods.

Si agregaste una **migración nueva**, agrégala también a `backend/database/migration_order.txt`. La Fase 5 la aplica.

### Ver el estado

```bash
kubectl --context docker-desktop -n soloservis get pods
```

```bash
kubectl --context docker-desktop -n soloservis get deploy,svc,pvc,ingress
```

### Logs

Últimas 50 líneas de una API, siguiendo en vivo (Ctrl+C para salir):

```bash
kubectl --context docker-desktop -n soloservis logs deployment/product-api --tail 50 -f
```

Los logs de las APIs son JSON (uno por request, con `request_id`). Para buscar errores:

**PowerShell**

```powershell
kubectl --context docker-desktop -n soloservis logs deployment/product-api --tail 500 | Select-String '"level":"ERROR"'
```

**bash**

```bash
kubectl --context docker-desktop -n soloservis logs deployment/product-api --tail 500 | grep '"level":"ERROR"'
```

### Reiniciar una API

```bash
kubectl --context docker-desktop -n soloservis rollout restart deployment/product-api
```

```bash
kubectl --context docker-desktop -n soloservis rollout status deployment/product-api
```

### Volver a la versión anterior (rollback)

```bash
kubectl --context docker-desktop -n soloservis rollout history deployment/product-api
```

```bash
kubectl --context docker-desktop -n soloservis rollout undo deployment/product-api
```

### Consola de PostgreSQL

```bash
kubectl --context docker-desktop -n soloservis exec -it postgres-0 -- psql -U postgres -d soloservis
```

Dentro de `psql`: `\dt` lista las tablas, `SELECT version FROM schema_migrations;` muestra las migraciones aplicadas y `\q` sale.

Para conectarte desde DBeaver, pgAdmin u otro cliente: abre un port-forward y conéctate a `localhost:15432` (usuario `postgres`, contraseña `postgres`, base `soloservis`).

```bash
kubectl --context docker-desktop -n soloservis port-forward svc/postgres 15432:5432
```

### Borrar todo

Local:

```bash
kubectl --context docker-desktop delete namespace soloservis
```

UCT (en tu namespace no puedes borrarlo, así que se borran los recursos). ⚠️ Esto incluye la base de datos:

```bash
kubectl delete all,ingress,configmap,job,pvc -l app.kubernetes.io/part-of=soloservis
```

```bash
kubectl delete secret postgres-secret soloservis-internal --ignore-not-found
```

---

## 7. Ejemplos de uso de la app desplegada

En los ejemplos, `$BASE` es la URL de la app:

**PowerShell**

```powershell
$BASE = "http://localhost:8080"                              # local (con el port-forward de 3.3 abierto)
$BASE = "http://soloservis-<usuario>.dev.censei.cl"          # UCT
```

**bash**

```bash
BASE=http://localhost:8080                                   # local (con el port-forward de 3.3 abierto)
BASE=http://soloservis-<usuario>.dev.censei.cl               # UCT
```

> En PowerShell usa siempre `curl.exe`, no `curl`, porque `curl` es un alias de `Invoke-WebRequest`.

### Consultas públicas

**PowerShell**

```powershell
curl.exe -s "$BASE/categories"
curl.exe -s "$BASE/categories/refrigeradores"
curl.exe -s "$BASE/search/products?q=refrigerador&limit=3"
curl.exe -s "$BASE/search/facets?category=refrigeradores"
curl.exe -s "$BASE/stores"
```

**bash**

```bash
curl -s "$BASE/categories"
curl -s "$BASE/categories/refrigeradores"
curl -s "$BASE/search/products?q=refrigerador&limit=3"
curl -s "$BASE/search/facets?category=refrigeradores"
curl -s "$BASE/stores"
```

### Registro y sesión

**PowerShell**

```powershell
# Invoke-RestMethod para el POST: PowerShell 5.1 le quita las comillas al JSON si se lo pasas a curl.exe.
$body = '{"name":"Ana Prueba","email":"ana@test.cl","password":"Clave12345!"}'
$session = Invoke-RestMethod -Method Post -Uri "$BASE/auth/register" -ContentType 'application/json' -Body $body
$TOKEN = $session.accessToken
curl.exe -s -H "Authorization: Bearer $TOKEN" "$BASE/auth/validate"
curl.exe -s -H "Authorization: Bearer $TOKEN" "$BASE/comparisons/products"
```

**bash**

```bash
TOKEN=$(curl -s -X POST -H 'Content-Type: application/json' \
  -d '{"name":"Ana Prueba","email":"ana@test.cl","password":"Clave12345!"}' \
  "$BASE/auth/register" | sed -n 's/.*"accessToken":"\([^"]*\)".*/\1/p')
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/auth/validate"
curl -s -H "Authorization: Bearer $TOKEN" "$BASE/comparisons/products"
```

Si el correo ya existe, usa `/auth/login` con `{"email": ..., "password": ...}`.

### APIs internas (Pricing, Ingestion, User, Notification)

No están publicadas. Para probarlas, abre un port-forward directo a la API:

- **local:** `svc/pricing-api` (8085), `svc/ingestion-api` (8094), ...
- **UCT:** `svc/backend` (8080), que atiende todas las rutas.

**PowerShell** (local; en UCT, `svc/backend 8094:8080`)

```powershell
kubectl --context docker-desktop -n soloservis port-forward svc/ingestion-api 8094:8094
```

En otra terminal, lee el token de servicio y consulta las ejecuciones del scraper:

```powershell
$raw = kubectl --context docker-desktop -n soloservis get secret soloservis-internal -o jsonpath='{.data.INTERNAL_API_TOKEN}'
$INTERNAL = [Text.Encoding]::UTF8.GetString([Convert]::FromBase64String($raw))
curl.exe -s -H "Authorization: Bearer $INTERNAL" "http://localhost:8094/ingestion/runs"
```

**bash** (local; en UCT, `svc/backend 8094:8080`)

```bash
kubectl --context docker-desktop -n soloservis port-forward svc/ingestion-api 8094:8094
```

```bash
INTERNAL=$(kubectl --context docker-desktop -n soloservis get secret soloservis-internal -o jsonpath='{.data.INTERNAL_API_TOKEN}' | base64 -d)
curl -s -H "Authorization: Bearer $INTERNAL" "http://localhost:8094/ingestion/runs"
```

El contrato completo para el scraper está en `backend/README.md`, §12.7. **No compartas el token:** permite escribir en el catálogo.

---

## 8. Solución de problemas

| Síntoma | Causa probable | Qué hacer |
|---|---|---|
| Fase 0: `no existe el contexto 'docker-desktop'` | Kubernetes no está activado | Docker Desktop → Settings → Kubernetes → Enable |
| Fase 0 (UCT): `sin acceso al namespace` | kubeconfig vencido o login OIDC no completado | descarga de nuevo el kubeconfig y corre `kubectl auth can-i list pods` |
| Fase 3: `docker push` denegado | no hiciste `docker login` o el usuario del registry no es el tuyo | `docker login` y `-Registry docker.io/<tu-usuario>` |
| Fase 6: `ErrImageNeverPull` / `ImagePullBackOff` (local) | el clúster no ve las imágenes locales (tipo `kind`) | en Docker Desktop elige el clúster **kubeadm**, o reinicia Kubernetes |
| Fase 6: `ImagePullBackOff` (UCT) | repositorio privado en Docker Hub | hazlo público, o crea `kubectl create secret docker-registry regcred --docker-username=<u> --docker-password=<token>` y usa `-ImagePullSecret regcred` |
| Fase 6 (UCT): pods `Pending` con `exceeded quota` | otro despliegue tuyo ocupa la cuota | `kubectl get pods,resourcequota`; borra lo que no uses o corre desde la Fase 1 |
| Fase 6: `CrashLoopBackOff` | la API no conecta a la base o falla al arrancar | `kubectl logs deployment/<api> --previous` |
| Fase 5: `faltan migraciones` | un `.sql` nuevo no está en `migration_order.txt` | agrégalo al manifiesto y corre desde la Fase 5 |
| Fase 7: el frontend no arranca (`host not found in upstream`) | falta el Service de alguna API | corre desde la Fase 6 (crea las APIs antes que el frontend) |
| Fase 8 (UCT): `sin respuesta: DNS o red` | el DNS aún no se propaga | espera unos minutos y abre la URL en el navegador |
| `port-forward`: `address already in use` | el puerto 8080 está ocupado (compose, Vite, ...) | usa otro puerto: `port-forward svc/frontend 8081:80` |
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
├── base/                         común a los dos modos
│   ├── postgres/                 Secret (credenciales de desarrollo), Services, StatefulSet + PVC
│   ├── migrations/               Job + migrate.sh (aplica migration_order.txt y registra schema_migrations)
│   ├── frontend/                 Deployment (nginx sin root, solo lectura) + Service
│   ├── ingress.yaml              host *.dev.censei.cl (se elimina en modo local)
│   └── kustomization.yaml
├── components/
│   ├── microservicios/           14 Deployments + Services, uno por API (modo local)
│   └── consolidado/              Deployment "backend" (all-apis) + destino del nginx (modo uct)
└── overlays/<modo>/              GENERADO por el script (ignorado por git): namespace, imágenes,
                                  StorageClass, host y los .sql de backend/database/migrations
scripts/k8s/deploy.ps1            despliegue por fases (PowerShell 5.1+)
scripts/k8s/deploy.sh             el mismo despliegue en bash (Linux, macOS, Git Bash)
docker/<api>/Dockerfile           imagen de cada API (binario estático, usuario 10001)
docker/all-apis/Dockerfile        imagen del backend consolidado
docker/frontend/                  build de Vite + nginx (default.conf.template, proxy.conf)
```

**Seguridad:**
- **Postgres:** `base/postgres/secret.yaml` tiene credenciales **de desarrollo** (las mismas de `docker/compose.yml`). Para un entorno real se reemplazan por un Secret creado fuera de git.
- **Token interno:** el token de `soloservis-internal` lo genera el script al azar en cada namespace y no está en git.
- **Pods:** todos corren sin root, con `readOnlyRootFilesystem` y sin capabilities. Cumplen Pod Security `restricted`.
