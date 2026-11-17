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
python scripts/testing/salud.py               # resultado en la terminal
python scripts/testing/salud.py --md salud.md # además, en Markdown
```

Usa el `kubectl` y el namespace del contexto actual. Sale con código 1 si alguna comprobación falla y con 2 si no puede consultar el clúster.

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
| Gateway y acceso público | Rutas del gateway | `/`, `/products`, `/search`, `/services` y `/auth` no llegan a su dominio. Una ruta de API que responde HTML de la app también es falla: es una ruta que nginx no enruta |
| | Acceso público | el host del Ingress no responde `/healthz` |
| PostgreSQL | Responde | la consulta falla |
| | Migraciones | (aviso) faltan migraciones del manifiesto local, o hay aplicadas que el checkout no conoce |
| Recursos | Cuota del namespace | (aviso) algún recurso usa el 80 % o más |
| | Volúmenes | algún PVC no está en `Bound` |

## 3. Resultado en el clúster (2026-11-17 19:01)

**✅ El despliegue está sano: 14 OK, 0 fallas, 1 aviso.**

| Sección | Estado | Detalle |
| --- | --- | --- |
| Pods | ✅ | 6 de 6 listos (comercial, frontend, ingestion, postgres-0, scraper-api, usuarios), ningún reinicio, 5 de 5 Deployments con el tag `471c467`, ningún Job fallido |
| APIs | ✅ | `comercial`, `usuarios` e `ingestion` con `"database":"connected"`; `scraper-api` y frontend responden |
| Gateway | ✅ | 5 de 5 rutas llegan a su dominio |
| Acceso público | ✅ | `https://soloservis-marcelo-vidal.dev.censei.cl/healthz` → 200 en 223 ms |
| PostgreSQL | ✅ | 140 productos activos; 28 de 28 migraciones aplicadas |
| Volúmenes | ✅ | todos en `Bound` |
| Cuota | ⚠️ | `limits.memory` 1664Mi de 2G (87 %) |

Cuota completa: `limits.cpu` 1550m/2 · `limits.memory` 1664Mi/2G · PVC 2/3 · pods 6/12 · `requests.cpu` 275m/1 · `requests.memory` 704Mi/1G · `requests.storage` 6Gi/10Gi · Services 7/10.

**Sobre el aviso de memoria:** no impide desplegar. Los Deployments actualizan con `maxSurge: 0` (y `scraper-api` con `Recreate`), así que nunca levantan un pod de más durante un despliegue. Sí deja poco margen (unos 240Mi de límite) para agregar cargas nuevas, por ejemplo un CronJob de respaldo.

## 4. Pruebas de que detecta fallas

Con respuestas simuladas (sin tocar el clúster), el script marcó:

| Situación simulada | Resultado |
| --- | --- |
| `comercial` responde `"database":"disconnected"` | ❌ FALLA: la base no está conectada |
| `usuarios` no responde | ❌ FALLA: HTTP 0 |
| `/services` cae en la app (HTML 200) | ❌ FALLA: respondió HTML de la app, no la API |
| Pod en `CrashLoopBackOff`, no Ready | ❌ FALLA en pods y en Deployments (0 de 1 réplicas) |
| Contenedor con 4 reinicios | ⚠️ AVISO con el momento, el motivo (`Error`) y el código de salida |

## 5. Limitaciones

- **El certificado HTTPS de la plataforma no es de confianza** (ver `k8s/README.md`). El chequeo del host público lo acepta igual, como cuando se acepta el aviso en el navegador.
- **Las migraciones se comparan con el manifiesto del checkout local.** Si tu rama tiene migraciones que todavía no se despliegan, aparecen como aviso y no como falla.
- **No revisa el contenido de los datos:** el flujo de productos de punta a punta lo cubre `scripts/testing/flujo_productos.py` (TAL-184).
