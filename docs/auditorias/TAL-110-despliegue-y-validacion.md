# TAL-110 — Despliegue y validación de la arquitectura consolidada

| Campo | Valor |
| --- | --- |
| Tarea | TAL-110: Desplegar los componentes, comprobar Pods y Services, verificar cada endpoint de salud y validar la comunicación con PostgreSQL, el frontend y el API Gateway |
| Fecha | 2026-11-12 |
| Despliegue | clúster UCT, namespace `student-marcelo-vidal-22843887e32d`, topología `dominios` |
| Imágenes | `docker.io/shell000/soloservis-backend`, `-frontend` y `-scraper`, tag `0f414ea` |
| Cómo se obtuvo | `scripts/k8s/deploy.ps1 -Modo uct -FromPhase 3 -SkipCleanup` (fases 3 a 10), `kubectl` sobre los recursos en ejecución, consultas de solo lectura a PostgreSQL y corridas reales del scraper |
| Relacionado | [TAL-95](TAL-95-inventario-despliegue.md) (inventario anterior, tag `bf2a897`), TAL-98 (PostgreSQL y secretos) y TAL-109 (Dockerfiles y Compose) |

Todo lo de este documento sale del clúster. Los Secrets se nombran, nunca se muestra su valor.

---

## 1. Resumen

1. **El despliegue completo pasó:** las fases 3 a 10 del script terminaron en OK y el pod de test pasó **29 de 29 verificaciones**.
2. **6 pods y 7 Services en ejecución**, dentro de la cuota: los 3 dominios de API, `frontend`, `postgres` y `scraper-api`.
3. **Los 3 dominios responden `/health` con `"database":"connected"`** y esperan a PostgreSQL antes de arrancar (initContainer `espera-postgres`, TAL-98).
4. **PostgreSQL conserva los datos** al reiniciar el pod: 23 de 23 migraciones antes y después.
5. **El frontend (nginx) enruta cada ruta pública al pod de su dominio** y las 6 rutas públicas responden 200 por el Ingress.
6. **El scraper entrega datos reales de punta a punta:** scraper → Ingestion API → PostgreSQL → sitio público. Entraron 82 productos de Falabella y Ripley sin rechazos.

---

## 2. Pods y Services

| Pod | Estado | Reinicios | Imagen |
| --- | --- | --- | --- |
| `comercial` | Running 1/1 | 0 | `soloservis-backend:0f414ea` |
| `usuarios` | Running 1/1 | 0 | `soloservis-backend:0f414ea` |
| `ingestion` | Running 1/1 | 0 | `soloservis-backend:0f414ea` |
| `frontend` | Running 1/1 | 0 | `soloservis-frontend:0f414ea` |
| `scraper-api` | Running 1/1 | 0 | `soloservis-scraper:0f414ea` |
| `postgres-0` | Running 1/1 | 0 | `postgres:18` |

Además quedan en `Completed` los pods de los Jobs: `soloservis-migrate`, `soloservis-smoke-test` y las ejecuciones del CronJob `scraper-trigger`.

| Service | Tipo | Puerto |
| --- | --- | --- |
| `comercial`, `usuarios`, `ingestion` | ClusterIP | 8080 |
| `frontend` | ClusterIP | 80 |
| `scraper-api` | ClusterIP | 8095 |
| `postgres`, `postgres-headless` | ClusterIP | 5432 |

Solo `frontend` se publica, por el Ingress `soloservis` (host `soloservis-marcelo-vidal.dev.censei.cl`). PostgreSQL, las APIs y el scraper no son accesibles desde fuera del clúster.

**Cuota del namespace:**

| Recurso | Usado | Límite |
| --- | --- | --- |
| Pods | 6 | 12 |
| Services | 7 | 10 |
| PVC | 2 (`postgres-data-postgres-0` 5Gi, `scraper-data` 1Gi) | 3 |
| CPU reservada (requests) | 275m | 1 |
| Memoria reservada (requests) | 704Mi | 1G |
| CPU / memoria máxima (limits) | 1550m / 1664Mi | 2 / 2G |

---

## 3. Resultado por fase

| Fase | Qué valida | Resultado |
| --- | --- | --- |
| 3 | Build y push de las 3 imágenes | OK, tag `0f414ea` |
| 4 | Secrets y PostgreSQL | OK: `soloservis-internal` y `postgres-secret` existentes, se reutilizan |
| 5 | Job de migraciones | OK: 23 de 23 |
| 6 | APIs y scraper | OK: `comercial`, `usuarios`, `ingestion` Ready con PostgreSQL conectado; `scraper-api` Ready |
| 7 | Frontend | OK: `/products` responde a través de nginx |
| 8 | Ingress y rutas públicas | OK: 6 de 6 con 200 |
| 9 | Recuperación y persistencia | OK: Kubernetes recreó el pod de `comercial`; mismos datos tras reiniciar PostgreSQL; las 3 APIs reconectaron |
| 10 | Pod de test dentro del clúster | OK: 29 OK, 0 fallas |

---

## 4. Endpoints de salud

```
comercial: {"status":"ok","service":"comercial","database":"connected"}
usuarios:  {"status":"ok","service":"usuarios","database":"connected"}
ingestion: {"status":"ok","service":"ingestion","database":"connected"}
scraper /ready: {"status":"ready","database":"ok (13 runs)","stores":3}
```

`/health` de cada dominio hace ping a PostgreSQL: es la readiness del pod, así que un pod sin base no recibe tráfico. El `/ready` del scraper revisa su SQLite propio y que los 3 specs carguen.

---

## 5. Comunicación con PostgreSQL

| Qué | Evidencia |
| --- | --- |
| Conexión de cada API | `"database":"connected"` en los 3 dominios (sección 4) |
| Credenciales | `DATABASE_URL` se arma desde el Secret `postgres-secret`, que ya no está en git: lo crea o reutiliza la fase 4 (TAL-98) |
| Orden de arranque | Los 3 pods de API muestran el initContainer `espera-postgres` en `Completed`: no arrancan hasta que `pg_isready` responde (TAL-98) |
| Migraciones | Job `soloservis-migrate` Completed; `schema_migrations` con 23 de 23 |
| Persistencia | Fase 9: se reinició `postgres-0` y `schema_migrations` siguió en 23; las 3 APIs reconectaron solas |
| Lecturas reales | El pod de test lee por cada API (`/products`, `/stores`, `/services`, `/offers`, `/search`, `/reviews`, `/users`) |

---

## 6. Frontend y API Gateway

El API Gateway es el nginx del `frontend`: la plataforma UCT solo permite una ruta `/` por hostname en el Ingress, así que el enrutamiento por path se hace ahí.

```
Navegador → Ingress (/) → frontend (nginx) → comercial:8080 | usuarios:8080
```

| Ruta pública | Código |
| --- | --- |
| `/` | 200 |
| `/products?limit=1` | 200 |
| `/categories` | 200 |
| `/stores` | 200 |
| `/services` | 200 |
| `/search/products?q=a&limit=1` | 200 |

Verificado desde fuera del clúster, por `https://soloservis-marcelo-vidal.dev.censei.cl`. El certificado es el de la plataforma (no confiable para el navegador): con `curl` se usa `-k`.

El pod de test confirma además el **aislamiento entre dominios** (cada pod responde 404 a las rutas de los otros) y que las rutas protegidas responden **401** sin token.

---

## 7. Scraper → Ingestion API → PostgreSQL

Corridas lanzadas con `POST /runs` en `scraper-api`:

| Tienda | Estado | Encontrados | Enviados | Rechazados |
| --- | --- | --- | --- | --- |
| falabella-cl | success | 25 | 25 | 0 |
| ripley-cl | success | 57 | 57 | 0 |

Cada corrida llama a `POST /ingestion/runs` (201), `/items` (200) y `/finish` (200) en `http://ingestion:8080`, con el token del Secret `soloservis-internal`.

**Efecto en PostgreSQL:**

| Tabla | Antes | Después |
| --- | --- | --- |
| `product` | 38 | 121 |
| `product_offer` | 78 | 161 |
| Ofertas de Falabella (tienda 1) | 22 | 48 |
| Ofertas de Ripley (tienda 4) | 14 | 71 |
| Ofertas de Paris (tienda 2) | 18 | 18 |
| `product_price_history` | 124 | 207 |

Las ofertas de Ripley quedaron en la tienda correcta y las de Paris no cambiaron. En `scrape_run` las corridas 7 (Falabella, 25 de 25) y 8 (Ripley, 57 de 57) quedaron en `success`. Las corridas 3 a 6 son de las pruebas previas al arreglo de la sección 8.

**En el sitio público:** `/search/products?q=refrigerador` devuelve 88 resultados, y un producto ingresado se lee con su atributo (`GET /products/{id}` → `capacidad_litros: 262`).

---

## 8. Problemas encontrados y corregidos

Ninguno aparecía en Docker Compose: salieron al desplegar y validar en el clúster.

| Problema | Causa | Corrección |
| --- | --- | --- |
| La contraseña de PostgreSQL estaba en git | `k8s/base/postgres/secret.yaml` | La fase 4 crea o reutiliza `postgres-secret`; la contraseña de UCT se cambió (PR #61) |
| `scraper-api` en CrashLoopBackOff | El Service `scraper-api` inyectaba `SCRAPER_API_PORT=tcp://…` y pisaba el puerto | `enableServiceLinks: false` (PR #63) |
| CronJob `scraper-trigger` colgado 4 días | Pedía 10m/32Mi; el LimitRange exige 25m/64Mi y el pod se rechazaba | Requests 25m/64Mi y `activeDeadlineSeconds: 600` (PR #63) |
| El scraper no enviaba nada (`itemsSent: 0`) | `execute()` se llamaba sin el cliente de ingestion | Se pasa `IngestionClient.from_settings()` (PR #64) |
| Ofertas de Ripley registradas como Paris | `storeId` del spec no coincidía con la base | Ripley 2→4 y Mercado Libre 3→6 (PR #64) |
| Las APIs se reiniciaban en bucle si la base no estaba lista | No había espera en los manifiestos | initContainer `espera-postgres` (PR #65) |
| Ingestion rechazaba los 83 productos | La categoría `refrigeradores` exige `capacidad_litros` y los specs no lo extraían | Opción `regex` en los specs: el atributo sale del nombre (PR #67) |

---

## 9. Pendientes

| Hallazgo | Detalle |
| --- | --- |
| Mercado Libre bloquea al scraper | Redirige el tráfico automático a su verificación de cuenta (`/gz/account-verification`). Sus corridas fallan sin afectar a las otras tiendas. Opción a futuro: su API oficial |
| Los IDs de tienda dependen del entorno | `005_demo_catalog.sql` inserta las tiendas sin `ORDER BY`. Si otra base asigna otros IDs, los `storeId` de los specs no coinciden. Opción a futuro: que ingestion resuelva la tienda por nombre |
| Volúmenes antiguos del scraper | El SQLite del scraper guarda `store_id` por fuente. En UCT se alinearon Ripley (4) y Mercado Libre (6) a mano; un volumen antiguo en otro entorno responde 409 hasta hacer lo mismo |
| Certificado HTTPS | Es el de la plataforma: el navegador muestra un aviso |

---

## 10. Cómo reproducir

```powershell
.\scripts\k8s\deploy.ps1 -Modo uct -Registry docker.io/<usuario> -FromPhase 3 -SkipCleanup
```

```bash
kubectl get pods,svc,pvc,ingress
kubectl get resourcequota
kubectl exec deploy/comercial -c comercial -- wget -qO- http://127.0.0.1:8080/health
kubectl logs job/soloservis-smoke-test
```

Corrida del scraper (el token se lee del entorno del pod, no se imprime):

```bash
kubectl exec deploy/scraper-api -- python -c "import os,json,urllib.request as u; r=u.Request('http://127.0.0.1:8095/runs', data=json.dumps({'store':'falabella-cl'}).encode(), method='POST', headers={'Authorization':'Bearer '+os.environ['SCRAPER_API_TOKEN'],'Content-Type':'application/json'}); print(u.urlopen(r).read().decode())"
```
