# TAL-95 — Inventario de los microservicios desplegados

| Campo | Valor |
| --- | --- |
| Tarea | TAL-95: Identificar los microservicios, sus comandos de inicio, puertos, variables de entorno, dependencias y health checks |
| Fecha | 2026-10-31; actualizado el 2026-11-01 tras re-desplegar TAL-125 |
| Despliegue | clúster UCT, namespace `student-marcelo-vidal-22843887e32d`, topología `dominios` (TAL-103) |
| Imágenes | `docker.io/shell000/soloservis-backend:bf2a897` y `docker.io/shell000/soloservis-frontend:bf2a897` (antes `33b8358`) |
| Cómo se obtuvo | `kubectl` sobre los recursos en ejecución, `wget` dentro de cada pod, `pg_stat_activity` en PostgreSQL y el pod de test (`k8s/tests`) |
| Tipo | Solo lectura y documentación. No se modificó nada del clúster. |
| Antes | [TAL-105](TAL-105-rutas-y-dependencias.md): rutas, middlewares y llamadas entre las 14 APIs según el código (el anexo trae los puertos y variables de cada API por separado) |
| Después | [TAL-125](TAL-125-responsabilidades-y-datos.md): corrige D1, D2 y D4; este inventario ya refleja ese despliegue |

Todo lo de este documento sale del clúster, no del código. Los Secrets se nombran, nunca se muestra su valor.

---

## 1. Resumen

1. **Las 14 APIs corren en 3 pods, uno por dominio:** `comercial` (11 APIs), `usuarios` (auth y user) e `ingestion`. Los tres usan **la misma imagen**; solo cambia `API_DOMAIN`.
2. **En total hay 5 pods y 6 Services** (más los pods `Completed` de los Jobs de migraciones y de test): 5 de 12 pods y 6 de 10 Services de la cuota.
3. **Todos escuchan en el 8080** y se llaman por el DNS de su Service (`comercial:8080`, `usuarios:8080`, `ingestion:8080`). Solo el `frontend` se publica, por el Ingress.
4. **Una sola dependencia HTTP entre pods:** ingestion → notification, por `NOTIFICATION_API_URL=http://comercial:8080`. El resto de las llamadas entre APIs ocurre dentro de cada pod.
5. **Los 3 pods de API comparten la base `postgres`.** Cada proceso abre un pool (máximo 10 conexiones); hoy hay 2 abiertas por pod.
6. **Health:** los 3 responden `GET /health` con el nombre de su dominio y `"database":"connected"`. El pod de test pasó **29 de 29 verificaciones**.

---

## 2. Pods

| Pod | Tipo | Imagen | Comando de inicio | Puerto (contenedor → Service) | Requests / limits | Nodo |
| --- | --- | --- | --- | --- | --- | --- |
| `comercial` | Deployment | `soloservis-backend:bf2a897` | `./all-apis` con `API_DOMAIN=comercial` | 8080 → `comercial:8080` | 50m/128Mi · 300m/256Mi | agent-01 |
| `usuarios` | Deployment | `soloservis-backend:bf2a897` | `./all-apis` con `API_DOMAIN=usuarios` | 8080 → `usuarios:8080` | 25m/64Mi · 200m/128Mi | agent-02 |
| `ingestion` | Deployment | `soloservis-backend:bf2a897` | `./all-apis` con `API_DOMAIN=ingestion` | 8080 → `ingestion:8080` | 25m/64Mi · 200m/128Mi | agent-01 |
| `frontend` | Deployment | `soloservis-frontend:bf2a897` | nginx (entrypoint de la imagen, genera `default.conf` con envsubst) | 8080 → `frontend:80` | 25m/64Mi · 100m/128Mi | agent-01 |
| `postgres-0` | StatefulSet | `postgres:18` | entrypoint de la imagen oficial | 5432 → `postgres:5432` y `postgres-headless` | 100m/256Mi · 250m/512Mi | agent-02 |
| `soloservis-migrate` | Job (Completed) | `postgres:18` | `/bin/bash /scripts/migrate.sh` (aplica `migration_order.txt`) | — | — | — |
| `soloservis-smoke-test` | Job (Completed) | `curlimages/curl:8.11.1` | `sh /scripts/smoke-test.sh` | — | 25m/64Mi · 100m/64Mi | — |

- El comando `./all-apis` es el `CMD` de `docker/all-apis/Dockerfile`; el Deployment no lo sobrescribe. Corre con el usuario 10001, sin root, con el sistema de archivos de solo lectura y sin capabilities.
- Al arrancar, cada pod lo registra en su log: `Dominio comercial escuchando en :8080` (y lo mismo con `usuarios` e `ingestion`).
- **Uso de la cuota** (`kubectl get resourcequota`): pods 5/12, Services 6/10, requests 225m/1 CPU y 576Mi/1G, limits 1050m/2 CPU y 1152Mi/2G, PVC 1/3.
- Datos: PVC `postgres-data-postgres-0`, `Bound`, `cinder-ceph`, 5Gi.

---

## 3. Los 14 microservicios: dónde corre cada uno

| API | Pod | Cómo se llega dentro del clúster | Pública por el nginx |
| --- | --- | --- | --- |
| product | comercial | `comercial:8080/products`, `/categories`, `/product-categories` | sí, solo GET |
| store | comercial | `comercial:8080/stores` | sí, solo GET |
| service | comercial | `comercial:8080/services` | sí, solo GET |
| pricing | comercial | `comercial:8080/offers` | no |
| search | comercial | `comercial:8080/search` | sí |
| comparison | comercial | `comercial:8080/comparisons` | sí |
| favorites | comercial | `comercial:8080/favorites` | sí |
| cart | comercial | `comercial:8080/cart` | sí |
| review | comercial | `comercial:8080/reviews` | sí |
| watchlist | comercial | `comercial:8080/watchlist` | sí |
| notification | comercial | `comercial:8080/notifications` | no |
| auth | usuarios | `usuarios:8080/auth` | sí, GET y POST |
| user | usuarios | `usuarios:8080/users` | no |
| ingestion | ingestion | `ingestion:8080/ingestion` | no (la usa el scraper con el token interno) |

Cada pod solo atiende las rutas de su dominio: las demás responden **404** (verificado por el pod de test, §7).

**Entrada pública:** `https://soloservis-marcelo-vidal.dev.censei.cl` → Ingress `soloservis` (clase `nginx`, path `/`, anotación `external-dns.alpha.kubernetes.io/target: proxy.inf.uct.cl`) → Service `frontend:80` → nginx. El nginx manda `/auth` a `usuarios:8080` y el resto de las rutas públicas a `comercial:8080` (variables `*_UPSTREAM` del Deployment `frontend`).

---

## 4. Variables de entorno

Leídas de los Deployments en ejecución (`kubectl get deploy -o jsonpath`).

| Variable | comercial | usuarios | ingestion | Origen |
| --- | --- | --- | --- | --- |
| `PORT` | `8080` | `8080` | `8080` | literal |
| `API_DOMAIN` | `comercial` | `usuarios` | `ingestion` | literal |
| `POSTGRES_USER`, `POSTGRES_PASSWORD`, `POSTGRES_DB` | ✔ | ✔ | ✔ | Secret `postgres-secret` |
| `DATABASE_URL` | ✔ | ✔ | ✔ | literal que arma la URL con las 3 anteriores: `postgres://$(POSTGRES_USER):$(POSTGRES_PASSWORD)@postgres:5432/$(POSTGRES_DB)?sslmode=disable` |
| `FRONTEND_URL` | ✔ | ✔ | ✔ | ConfigMap `soloservis-config`: `https://soloservis-marcelo-vidal.dev.censei.cl` |
| `INTERNAL_API_TOKEN` | ✔ | ✔ | ✔ | Secret `soloservis-internal` (opcional). `comercial` lo valida en `POST /notifications` y en las escrituras de pricing; `ingestion` lo valida y lo envía a notification |
| `NOTIFICATION_API_URL` | — | — | `http://comercial:8080` | literal |

- **No hay ninguna otra `*_API_URL`.** En `comercial`, el binario apunta las llamadas entre sus APIs (`PRODUCT_API_URL`, `PRICING_API_URL`, `STORE_API_URL`, `SERVICE_API_URL`, `NOTIFICATION_API_URL`) a `http://127.0.0.1:8080`, el propio pod (`backend/internal/allapis/domains.go`, `InternalURLs`). `usuarios` no llama a nadie.
- `LOG_LEVEL` no está definida: las APIs usan `info`.
- **frontend:** `PRODUCT_UPSTREAM`, `STORE_UPSTREAM`, `SERVICE_UPSTREAM`, `FAVORITES_UPSTREAM`, `CART_UPSTREAM`, `SEARCH_UPSTREAM`, `COMPARISON_UPSTREAM`, `REVIEW_UPSTREAM` y `WATCHLIST_UPSTREAM` = `comercial:8080`; `AUTH_UPSTREAM` = `usuarios:8080`.
- **postgres:** `PGDATA=/var/lib/postgresql/18/docker` y las credenciales de `postgres-secret`.
- Secrets del namespace: `postgres-secret` (credenciales de desarrollo, en git) y `soloservis-internal` (token generado por el script de deploy, fuera de git).

---

## 5. Dependencias

```
                    Ingress (soloservis-marcelo-vidal.dev.censei.cl)
                                   │
                                frontend ──/auth──► usuarios
                                   │
                                   └──resto──► comercial ◄──POST /notifications── ingestion ◄── scraper
                                                   │                                  │            (token)
             comercial, usuarios, ingestion ───────┴──────────► postgres ◄────────────┘
```

| Desde | Hacia | Para qué | Evidencia en el clúster |
| --- | --- | --- | --- |
| frontend | `comercial:8080`, `usuarios:8080` | rutas públicas | pod de test: `frontend /products` → 200 (comercial), `frontend /auth` → 401 (usuarios) |
| ingestion | `comercial:8080` (notification) | avisar bajas de precio | desde el pod `ingestion`, `wget http://comercial:8080/health` → `{"status":"ok",...}` |
| comercial (cart, favorites, review, watchlist) | `127.0.0.1:8080` (product, pricing, store, service) | verificar que el producto, la oferta, la tienda o el servicio existen | dentro del mismo pod; no sale a la red |
| comercial, usuarios, ingestion | `postgres:5432` | datos | `pg_stat_activity`: 2 conexiones `idle` desde cada IP de pod (10.42.160.181 comercial, 10.42.79.87 usuarios, 10.42.160.167 ingestion) |
| comercial, usuarios | tablas de `usuarios` (`user_session`, `user_account`) | validar el token de sesión con `authz.RequireSession`, la única validación desde TAL-125 | dependencia por la base, de solo lectura, no por HTTP |
| soloservis-migrate | `postgres:5432` | aplicar migraciones | Job `Complete`, 21 de 21 en `schema_migrations` |

- **Conexiones a la base:** cada pod de API tiene un solo pool de hasta 10 conexiones, compartido por todas sus APIs. Con 3 pods son como máximo 30, de las 100 de `max_connections`.
- **Si cae `comercial`:** ingestion sigue guardando las ofertas; solo falla el aviso a notification, que queda como warning en el log (TAL-105 §5).
- **Si cae `usuarios`:** no se puede iniciar sesión, pero las sesiones abiertas siguen funcionando en `comercial`, porque valida el token leyendo la base.

---

## 6. Health checks

| Pod | Endpoint | Respuesta real | readinessProbe | livenessProbe |
| --- | --- | --- | --- | --- |
| comercial | `GET /health` | `{"status":"ok","service":"comercial","database":"connected"}` | HTTP `/health`, cada 10 s, timeout 3 s, 3 fallos | TCP 8080, cada 20 s, 3 fallos |
| usuarios | `GET /health` | `{"status":"ok","service":"usuarios","database":"connected"}` | igual | igual |
| ingestion | `GET /health` | `{"status":"ok","service":"ingestion","database":"connected"}` | igual | igual |
| frontend | `GET /healthz` | `ok` (lo responde nginx, sin tocar las APIs) | HTTP `/healthz`, cada 10 s | HTTP `/healthz`, cada 20 s |
| postgres-0 | `pg_isready` | — | exec, cada 5 s | exec, cada 10 s, 6 fallos |

- `/health` hace ping a PostgreSQL: con la base caída responde **503** y el pod sale de rotación, pero la liveness (TCP) no lo reinicia. Cuando vuelve la base, el pod se recupera solo.
- Se vio en la Fase 9 del deploy, que reinicia `postgres-0` a propósito: los eventos muestran `Readiness probe failed: HTTP probe failed with statuscode: 503` en los 3 pods durante el reinicio, sin reinicios de contenedor (`RESTARTS 0`), y después los 3 vuelven a `Ready`.
- `/health` lo responde el router del dominio (`allapis.NewDomainRouter`) con el nombre del dominio, para saber qué pod contestó (TAL-125). En la topología consolidada dice `all-apis`.

---

## 7. Verificación con el pod de test

Job `soloservis-smoke-test` (Fase 10 del deploy), del 2026-11-01 a las 18:36 UTC: **29 OK, 0 fallas**, en 17 segundos.

| Grupo | Verificaciones | Resultado |
| --- | --- | --- |
| Health de los 3 pods de API | 3 | 200, el nombre del dominio y `database: connected` |
| Una lectura por API pública (product, store, service, pricing, search, comparison, review, user) | 8 | 200 |
| Rutas con sesión o internas sin token (favorites, cart, comparaciones de productos y de servicios, `POST /reviews`, watchlist y notifications con `userId=1`, `POST /notifications`, auth, ingestion) | 10 | 401 |
| Ruta interna con `INTERNAL_API_TOKEN` (`GET /ingestion/runs`) | 1 | 200 |
| Aislamiento entre dominios | 4 | 404 |
| nginx del frontend (`/`, `/products`, `/auth`) | 3 | 200, 200, 401 |

Las 14 APIs quedan cubiertas: 8 con una lectura y 6 (favorites, cart, watchlist, notification, auth, ingestion) con la respuesta de su guarda de sesión o de token. Desde TAL-125, watchlist y notification ya no responden a un `userId` sin token.

**Fase 9 del deploy (resiliencia):** al borrar el pod de `comercial`, Kubernetes lo recreó; tras reiniciar `postgres-0`, los datos persistieron (21 migraciones) y las 3 APIs se reconectaron solas.

---

## 8. Antes y después

| | Antes (consolidado, hasta TAL-103) | Ahora (dominios) |
| --- | --- | --- |
| Pods de API | 1 (`backend`, las 14 APIs) | 3 (`comercial`, `usuarios`, `ingestion`) |
| Pods / Services totales | 3 / 4 | 5 / 6 |
| Requests | 175m / 448Mi | 225m / 576Mi |
| Llamadas entre pods | ninguna | 1 (ingestion → comercial) |
| Si se cae un pod de API | se caen las 14 APIs | se cae solo su dominio |

La topología consolidada sigue disponible como respaldo (`-Topologia consolidado`).

---

## 9. Hallazgos

| # | Severidad | Hallazgo | Propuesta | Estado |
| --- | --- | --- | --- | --- |
| D1 | 🟡 baja | `/health` de `comercial` e `ingestion` decía `"service":"soloservis-api"`: no identificaba al pod (TAL-105 H6) | que `/health` informe el dominio o la API | ✅ resuelto en TAL-125: dice `comercial`, `usuarios`, `ingestion` |
| D2 | 🟡 baja | `FRONTEND_URL` (origen de CORS) era `http://…`, pero la plataforma sirve la app por `https://…`. No afectaba: el navegador llama a las APIs por el mismo origen, a través del nginx, y no necesita CORS | usar `-IngressScheme https` como valor por defecto del deploy | ✅ resuelto en TAL-125: `https://…` |
| D3 | ⚪ info | La Fase 8 falló al probar las rutas públicas desde el equipo que despliega: el DNS dejó de resolver un momento mientras se recreaba el Ingress, y Windows guardó en caché la respuesta negativa. Con la IP forzada (`curl --resolve`) todas respondieron 200 | `ipconfig /flushdns`, o reintentar `-FromPhase 8` unos minutos después | no se repitió en el re-despliegue del 2026-11-01 |
| D4 | ⚪ info | El log de arranque decía `running on http://localhost:8080`, aunque el pod escucha en todas sus interfaces | cosmético | ✅ resuelto en TAL-125: `escuchando en :8080` |

---

## 10. Cómo reproducir este inventario

```bash
kubectl get deploy,statefulset,pods,svc,ingress,pvc -l app.kubernetes.io/part-of=soloservis -o wide
```

```bash
kubectl get resourcequota
```

```bash
kubectl exec deploy/comercial -- wget -qO- http://127.0.0.1:8080/health
```

```bash
kubectl exec deploy/ingestion -- printenv API_DOMAIN NOTIFICATION_API_URL
```

```bash
kubectl logs job/soloservis-smoke-test
```

Para repetir el pod de test: `.\scripts\k8s\deploy.ps1 -Modo uct -FromPhase 10` o `./scripts/k8s/deploy.sh --modo uct --from-phase 10` (ver `k8s/README.md` §5).
