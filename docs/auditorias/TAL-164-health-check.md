# TAL-164 — Health check del despliegue

| Campo | Valor |
| --- | --- |
| Tarea | TAL-164: crear scripts para verificar la disponibilidad y el estado de los servicios |
| Fecha | 2026-11-17 |
| Despliegue | clúster UCT, namespace `student-marcelo-vidal-22843887e32d`, topología `dominios`, tag `471c467` |
| Script | `scripts/testing/salud.py` |
| Relacionado | [TAL-183](TAL-183-pruebas-integrales.md) (pruebas integrales), TAL-184 (flujo de productos), pod de test `k8s/tests` |

---

## 1. Para qué sirve

El pod de test (fase 10 del deploy, 35 verificaciones) solo corre al desplegar. `salud.py` revisa el despliegue **en cualquier momento**, por ejemplo cuando alguien avisa que el sitio no funciona, y dice en una pasada qué está mal.

```bash
python scripts/testing/salud.py                        # resultado en la terminal
python scripts/testing/salud.py --detalle              # además, cada pedido: esperado y obtenido
python scripts/testing/salud.py --detalle --md salud.md  # en Markdown
python scripts/testing/reporte.py --postgres-docker --salud  # tests del código + estado del clúster, un solo reporte
```

Usa el `kubectl` y el namespace del contexto actual. Sale con código 1 si alguna comprobación falla y con 2 si no puede consultar el clúster.

**Dentro del reporte de testing (`reporte.py --salud`):** cada comprobación es una prueba. Los avisos cuentan como aprobados y se listan aparte, junto con la tabla de pedidos realizados. A diferencia de `--smoke`, no depende de que exista el Job del pod de test, que solo queda en el namespace justo después de un despliegue. Si no hay acceso al clúster, la suite aparece como "no ejecutada" con el motivo.

**Es de solo lectura:** `kubectl get`, pedidos `GET` y una sesión `READ ONLY` en PostgreSQL. No usa el token de servicio ni muestra secretos.

## 2. Qué revisa

| Sección | Comprobación | Falla si… |
| --- | --- | --- |
| Pods | Pods listos | algún pod no está Ready o está en `CrashLoopBackOff` / `ImagePullBackOff` |
| | Reinicios | (aviso) algún contenedor se reinició; muestra cuándo, el motivo y el código de salida |
| | Deployments | alguno no tiene todas sus réplicas listas; muestra el tag de imagen desplegado |
| | Jobs | (aviso) algún Job falló sin terminar bien |
| APIs | `/health` de cada proceso | no responde 200 o no informa `"database":"connected"` (`comercial`, `usuarios` e `ingestion`, o `backend` en la topología consolidada) |
| | `scraper-api /health`, `frontend /healthz` | no responden 200 |
| Gateway y acceso público | App del frontend | `/` no responde la app (HTML con `id="root"`) |
| | Enrutamiento del gateway | algún prefijo no llega al upstream que dice la config (ver abajo) |
| | Acceso público | el host del Ingress no responde `/healthz` |
| PostgreSQL | Responde | la consulta falla |
| | Migraciones | (aviso) faltan migraciones del manifiesto local, o hay aplicadas que el checkout no conoce |
| Recursos | Cuota del namespace | (aviso) algún recurso usa el 80 % o más |
| | Volúmenes | algún PVC no está en `Bound` |

### Cómo se comprueba el enrutamiento

Las rutas **no están escritas en el script**: se leen de la configuración que corre en el pod (`/etc/nginx/conf.d/default.conf`), regla por regla (`location` → `proxy_pass`, y los métodos de `limit_except`). Por cada prefijo:

1. se pide `/<prefijo>` por el gateway y directo a cada proceso de API (`comercial`, `usuarios`, `ingestion`);
2. la ruta **llega a su dominio** si la respuesta del gateway es igual a la del upstream configurado (código, `Content-Type` y cuerpo, sin el `request_id`, que cambia en cada pedido);
3. y **se distingue** si además es distinta de la de los otros procesos. Si coincide con varios, queda como aviso.

Si la respuesta del gateway es la de otro proceso, la falla dice cuál. Si es el HTML de la app, dice que nginx no enruta el prefijo.
## 3. Resultado en el clúster (2026-11-17)

**✅ El despliegue está sano: 15 OK, 0 fallas, 1 aviso.**

| Sección | Estado | Detalle |
| --- | --- | --- |
| Pods | ✅ | 6 de 6 listos (comercial, frontend, ingestion, postgres-0, scraper-api, usuarios), ningún reinicio, 5 de 5 Deployments con el tag `471c467`, ningún Job fallido |
| APIs | ✅ | `comercial`, `usuarios` e `ingestion` con `"database":"connected"`; `scraper-api` y frontend responden |
| Gateway | ✅ | `/` responde la app; 12 de 12 prefijos de la config desplegada llegan a su upstream |
| Acceso público | ✅ | `https://soloservis-marcelo-vidal.dev.censei.cl/healthz` → 200 en unos 200 ms |
| PostgreSQL | ✅ | 140 productos activos; 28 de 28 migraciones aplicadas |
| Volúmenes | ✅ | todos en `Bound` |
| Cuota | ⚠️ | `limits.memory` 1664Mi de 2G (87 %) |

Enrutamiento leído de la config desplegada y respuesta de cada proceso al mismo pedido:

| Prefijo | Upstream (config) | Métodos | Gateway | `comercial` | `usuarios` | `ingestion` |
| --- | --- | --- | --- | --- | --- | --- |
| `/products` | `comercial:8080` | GET | 200 JSON | **200 JSON** | 404 problem+json | 404 problem+json |
| `/product-categories` | `comercial:8080` | GET | 200 JSON | **200 JSON** | 404 problem+json | 404 problem+json |
| `/categories` | `comercial:8080` | GET | 200 JSON | **200 JSON** | 404 problem+json | 404 problem+json |
| `/stores` | `comercial:8080` | GET | 200 JSON | **200 JSON** | 404 problem+json | 404 problem+json |
| `/services` | `comercial:8080` | GET | 200 JSON | **200 JSON** | 404 problem+json | 404 problem+json |
| `/auth` | `usuarios:8080` | GET POST | 404 text/plain | 404 problem+json | **404 text/plain** | 404 problem+json |
| `/favorites` | `comercial:8080` | todos | 401 problem+json | **401 problem+json** | 404 problem+json | 404 problem+json |
| `/cart` | `comercial:8080` | todos | 401 problem+json | **401 problem+json** | 404 problem+json | 404 problem+json |
| `/search` | `comercial:8080` | todos | 404 text/plain | **404 text/plain** | 404 problem+json | 404 problem+json |
| `/comparisons` | `comercial:8080` | todos | 404 text/plain | **404 text/plain** | 404 problem+json | 404 problem+json |
| `/reviews` | `comercial:8080` | todos | 404 text/plain | **404 text/plain** | 404 problem+json | 404 problem+json |
| `/watchlist` | `comercial:8080` | todos | 401 problem+json | **401 problem+json** | 404 problem+json | 404 problem+json |

En negrita, el upstream con el que coincide la respuesta del gateway. Un 404 también sirve para identificar el dominio: el proceso dueño del prefijo responde su propio 404 en `text/plain`, y los demás, el 404 `problem+json` de "ruta de otro dominio".

**Hallazgo:** ese 404 en `text/plain` (`404 page not found`) del dueño del prefijo no sigue el estándar TAL-78, que pide `application/problem+json`. Aparece en la raíz de `/auth`, `/search`, `/comparisons` y `/reviews`.

Cuota completa: `limits.cpu` 1550m/2 · `limits.memory` 1664Mi/2G · PVC 2/3 · pods 6/12 · `requests.cpu` 275m/1 · `requests.memory` 704Mi/1G · `requests.storage` 6Gi/10Gi · Services 7/10.

**Sobre el aviso de memoria:** no impide desplegar. Los Deployments actualizan con `maxSurge: 0` (y `scraper-api` con `Recreate`), así que nunca levantan un pod de más durante un despliegue. Sí deja poco margen (unos 240Mi de límite) para agregar cargas nuevas, por ejemplo un CronJob de respaldo.

## 4. Pruebas de que detecta fallas

Con respuestas simuladas (sin tocar el clúster), el script marcó:

| Situación simulada | Resultado |
| --- | --- |
| `comercial` responde `"database":"disconnected"` | ❌ FALLA: la base no está conectada |
| `usuarios` no responde | ❌ FALLA: HTTP 0 |
| Pod en `CrashLoopBackOff`, no Ready | ❌ FALLA en pods y en Deployments (0 de 1 réplicas) |
| Contenedor con 4 reinicios | ⚠️ AVISO con el momento, el motivo (`Error`) y el código de salida |

Y contra el clúster real, alterando solo las reglas que el script cree leer de nginx:

| Regla alterada | Resultado |
| --- | --- |
| `/auth` → `comercial:8080` (en la config real va a `usuarios`) | ❌ FALLA: "la respuesta del gateway es la de usuarios" |
| `/offers` → `comercial:8080` (nginx no lo enruta: Pricing es interna) | ❌ FALLA: "el gateway respondió la app (nginx no enruta el prefijo)" |
| `/products` → `comercial:8080` (correcta, de control) | ✅ llega a su upstream |

## 5. Limitaciones

- **El certificado HTTPS de la plataforma no es de confianza** (ver `k8s/README.md`). El chequeo del host público lo acepta igual, como cuando se acepta el aviso en el navegador.
- **Las migraciones se comparan con el manifiesto del checkout local.** Si tu rama tiene migraciones que todavía no se despliegan, aparecen como aviso y no como falla.
- **No revisa el contenido de los datos:** el flujo de productos de punta a punta lo cubre `scripts/testing/flujo_productos.py` (TAL-184).
