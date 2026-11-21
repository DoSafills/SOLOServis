# TAL-183 — Pruebas integrales de SOLOServis

| Campo | Valor |
| --- | --- |
| Tarea | TAL-183: verificar el funcionamiento conjunto del frontend, backend, APIs y scraper |
| Fecha | 2026-11-17 |
| Despliegue | clúster UCT, namespace `student-marcelo-vidal-22843887e32d`, topología `dominios` |
| Imágenes | `docker.io/shell000/soloservis-backend`, `-frontend` y `-scraper`, tag `471c467` (`develop` tras el PR #112) |
| Cómo se obtuvo | reporte de testing local (`scripts/testing/reporte.py --postgres-docker`), CI de GitHub Actions sobre `develop`, `deploy.ps1 -Modo uct -FromPhase 3 -SkipCleanup`, `kubectl` y consultas de solo lectura a PostgreSQL |
| Relacionado | [TAL-110](TAL-110-despliegue-y-validacion.md) (despliegue anterior), [TAL-174](TAL-174-duplicados.md) (duplicados), [TAL-235](TAL-235-ci.md) (CI), TAL-169 (identidad de productos del scraper), TAL-231 (tildes del seed), TAL-238 (historial de precios) |

Todo lo de este documento sale de ejecuciones reales. Los Secrets se nombran, nunca se muestra su valor.

---

## 1. Resumen

1. **El backend pasa sus 893 tests**, incluidos los de integración contra PostgreSQL 18, y el CI de `develop` aprueba sus 6 jobs. La única falla es de formato en 2 archivos del frontend (sección 2).
2. **El despliegue de `develop` pasó completo:** 6 pods en ejecución y el pod de test con **35 OK y 0 fallas** (sección 3).
3. **Frontend, gateway y APIs funcionan juntos:** las rutas públicas responden 200 a través del nginx del frontend y los 3 dominios informan `"database":"connected"` (sección 4).
4. **El scraper funciona de punta a punta sin duplicar:** la corrida programada del 17-11 entró por la Ingestion API sin rechazos, creó solo 6 productos nuevos y registró 10 cambios reales de precio (sección 5).
5. **Durante las pruebas aparecieron 4 fallas, ya corregidas:** tildes dañadas por `deploy.ps1`, una migración bloqueada por esas tildes, un Deployment mal indentado y el scraper creando duplicados (sección 6).

---

## 2. Suites automáticas

### Reporte local

`python scripts/testing/reporte.py --postgres-docker` sobre `471c467`, con un PostgreSQL 18 desechable y las migraciones en el orden de `migration_order.txt`. Reporte completo: [2026-11-17-0928-471c467.md](../reportes/testing/2026-11-17-0928-471c467.md).

| Suite | Resultado |
| --- | --- |
| Backend (Go), con integración, `go test -p 1 ./...` | ✅ 893 de 893 |
| Frontend: `pnpm lint`, `pnpm build`, `pnpm audit` | ✅ |
| Frontend: `pnpm format:check` | ❌ 2 archivos sin formato (`ProductCategoriesPage.tsx`, `products-client.ts`) |
| Scraper v2 y crawler v1 | ⚪ no se ejecutaron: al Python local le faltan `pytest` y `scrapling`. El scraper sí corre en el CI (abajo) |

Los dos archivos del frontend tienen finales de línea LF: es formato real, no el problema conocido de CRLF. El CI no lo detecta porque `format:check` está fuera del job del frontend.

### CI (GitHub Actions)

Corrida `44020952309` sobre `develop` (merge del PR #112, 2026-11-17): **6 de 6 jobs aprobados.**

| Job | Resultado | Detalle |
| --- | --- | --- |
| Backend (Go) | ✅ | build, vet y tests (sin base: los de integración se omiten) |
| Scraper (pytest + ruff) | ✅ | 150 tests y `ruff` sin observaciones |
| Frontend (build + lint) | ✅ | |
| Skills sincronizados | ✅ | `.opencode/skills` y `.claude/skills` iguales |
| Docker Compose válido | ✅ | |
| Manifiestos de Kubernetes válidos | ✅ | kubeconform estricto: dominios 20/20, consolidado 16/16, pod de test 2/2 |

El job de manifiestos es nuevo (PR #112). Habría detectado antes del despliegue el error de indentación del Deployment del scraper (sección 6).

### Tests de integración en secuencia

Los paquetes de Go comparten la misma base de prueba. En paralelo se pisan los datos y fallan de forma intermitente: `TestComparisonIntegration_Compatibles`, `TestCatalogIntegration_ListPaginaYFiltroAntiguo` y `TestIntegration_ProductAPIEscrituraConAtributos`, también sin cambios en el código. Con `go test -p 1 ./...` pasan siempre. Así quedó documentado en `backend/README.md`, `AGENTS.md` y los skills; `reporte.py` ya los corría en secuencia.

---

## 3. Despliegue

`scripts/k8s/deploy.ps1 -Modo uct -Topologia dominios -Registry docker.io/shell000 -FromPhase 3 -SkipCleanup`, desde `develop` en `471c467`.

| Pod | Estado | Reinicios | Imagen |
| --- | --- | --- | --- |
| `comercial` | Running 1/1 | 0 | `soloservis-backend:471c467` |
| `usuarios` | Running 1/1 | 0 | `soloservis-backend:471c467` |
| `ingestion` | Running 1/1 | 0 | `soloservis-backend:471c467` |
| `frontend` | Running 1/1 | 0 | `soloservis-frontend:471c467` |
| `scraper-api` | Running 1/1 | 0 | `soloservis-scraper:471c467` |
| `postgres-0` | Running 1/1 | 0 | `postgres:18` |

- **Migraciones:** el Job `soloservis-migrate` terminó en `Complete`; quedan 28 registradas en `schema_migrations`.
- **Pod de test (fase 10):** `Resultado: 35 OK, 0 fallas`.
- **Cuota:** 6 de 12 pods, 7 de 10 Services, 2 de 3 PVC, 275m de 1 CPU y 704Mi de 1G de requests.

---

## 4. Frontend, gateway y APIs

Pedidos hechos desde dentro del clúster (`kubectl exec deploy/frontend`) contra el Service `frontend`, que es el API Gateway:

| Ruta | Dominio | Respuesta |
| --- | --- | --- |
| `/` | frontend | 200 |
| `/products?limit=1` | comercial | 200 |
| `/stores` | comercial | 200 |
| `/services` | comercial | 200 |
| `/search/products?q=refrigerador` | comercial | 200 |
| `/search/products?category=refrigeradores&limit=1` | comercial | 200, con `attributes.capacidad_litros` |
| `/products/27e2b77f-8c21-4629-9bbf-18939badeab3` | comercial | 200, con el historial de precios (`priceHistory`) |

> **Corrección (TAL-184):** la primera versión de esta tabla incluía `/offers/92/price-history` con respuesta 200. Ese 200 era la página HTML de la app, no la API: el gateway no publica `/offers` porque Pricing es una API interna (ver `k8s/README.md`). El frontend lee el historial desde el detalle del producto.

Salud de cada proceso:

```
comercial   {"status":"ok","service":"comercial","database":"connected"}
usuarios    {"status":"ok","service":"usuarios","database":"connected"}
ingestion   {"status":"ok","service":"ingestion","database":"connected"}
scraper-api {"status":"ok"}
```

### Historial de precios (TAL-238)

El detalle de producto devuelve cada punto del historial con la oferta, la tienda, la moneda y la hora exacta. Así el frontend puede cruzar cada punto con su oferta. Ejemplo real (refrigerador Midea de Ripley):

```json
{"date": "2026-11-16", "price": "159990.00", "offerId": 92, "storeId": 4, "currency": "CLP", "recordedAt": "2026-11-16T06:00:11.918602Z"}
```

`GET /offers/92/price-history` y `GET /offers/product/{id}/price-history` también incluyen `currency`. Se consultaron directo contra el Service `comercial`, porque el gateway no publica `/offers`.

---

## 5. Scraper de punta a punta

### Limpieza de duplicados (migración 021, TAL-169)

Ingestion reconocía un producto solo por SKU o por marca + modelo, y el scraper no envía ninguno: cada corrida creaba los productos otra vez. Ingestion ahora identifica el producto por tienda + URL de la oferta, y la migración 021 unió los duplicados existentes:

```
dedupe_scraped_products: 88 productos conservados, 400 copias borradas, 400 registros de historial traspasados
```

| | Antes | Después |
| --- | --- | --- |
| Productos | 534 | 134 |
| Ofertas | 574 | 174 |
| Historial de precios | 620 | 620 |

No se perdió ningún registro de historial.

### Corrida programada del 17-11 (03:00 de Chile)

El CronJob `scraper-trigger` pidió las 3 tiendas al `scraper-api`. Mercado Libre se queda en su verificación anti-bots: es esperado y no se intenta saltar.

| Corrida (ingestion) | Tienda | Ítems | Procesados | Fallidos | Estado |
| --- | --- | --- | --- | --- | --- |
| 19 | Falabella | 27 | 27 | 0 | `success` |
| 20 | Ripley | 56 | 56 | 0 | `success` |

| | Antes | Después | Diferencia |
| --- | --- | --- | --- |
| Productos | 134 | 140 | +6 |
| Ofertas | 174 | 180 | +6 |
| Historial de precios | 620 | 636 | +16 |

- **Los 6 productos nuevos son publicaciones nuevas** (3 de Falabella y 3 de Ripley), con URLs que no estaban en la base. Antes de TAL-169, cada corrida duplicaba unos 80.
- **El historial creció solo con datos reales:** 6 registros son el precio inicial de las ofertas nuevas y 10 son cambios de precio en ofertas existentes. Ninguno repite el precio anterior.

| Oferta | Precio anterior | Precio nuevo |
| --- | --- | --- |
| 57 | 332.991 | 419.990 |
| 79 | 219.990 | 239.990 |
| 80 | 149.990 | 169.990 |
| 85 | 1.299.990 | 1.349.990 |
| 97 | 419.990 | 427.490 |
| 112 | 829.990 | 839.990 |
| 117 | 639.990 | 659.990 |
| 120 | 379.990 | 519.990 |
| 130 | 369.990 | 339.990 |
| 138 | 599.990 | 619.990 |

---

## 6. Fallas encontradas y corregidas

| Falla | Causa | Corrección |
| --- | --- | --- |
| Tildes dañadas en el clúster (`Electrodom├®sticos`) | PowerShell 5.1 leía la salida de `kubectl kustomize` con la página de códigos de la consola (850): los `.sql` del ConfigMap de migraciones llegaban con los acentos rotos | `deploy.ps1` fija la salida de la consola en UTF-8 (PR #110) |
| La migración 019 fallaba ("No se encontró la jerarquía esperada") | Buscaba los nombres del seed con la tilde correcta y en la base estaban dañados | Nueva migración `018_fix_refrigeradores_seed_encoding.sql`, antes de la 019 (TAL-231, PR #108) |
| La fase 6 fallaba con `scraper-api: ERROR (kubectl apply)` | `spec:` del pod mal indentado en `k8s/base/scraper/deployment.yaml` desde el PR #84; `kubectl kustomize` no lo detecta | Indentación corregida (PR #110) y validación de manifiestos en el CI (PR #112) |
| El scraper duplicaba los productos en cada corrida | Ingestion no tenía cómo reconocer un producto sin SKU ni modelo | Identidad por tienda + URL y migración 021 (TAL-169) |

---

## 7. Pendientes

| Pendiente | Detalle |
| --- | --- |
| Un duplicado anterior a TAL-169 | Refrigerador Midea MDRS710 de Ripley: el producto 29 (seed, con SKU y modelo, oferta 57) y el 86 (scraper, oferta 126) tienen la misma URL. La 021 no lo tocó porque solo une productos sin SKU ni modelo. Las corridas nuevas actualizan la oferta 57; la 126 quedó sin actualizar desde el 16-11. Va en un ticket aparte. |
| Puntos repetidos en el historial | Los productos que estaban duplicados heredaron un punto por copia, a veces con el mismo precio y a milisegundos de distancia. No es un dato falso, pero ensucia el gráfico. Relacionado con TAL-229. |
| ~90 textos del seed con tildes dañadas | Regiones, descripciones de servicios y proveedores, por el mismo problema de `deploy.ps1`. Solo se corrigieron los que bloqueaban la 019. |
| Código de sqlc desactualizado | Regenerar todo cambia los tipos de 7 funciones de productos que no tienen relación con estos cambios. En TAL-238 se aplicó a mano solo el fragmento necesario. |
| Tests de integración en el CI | Hoy el CI los omite porque no tiene base. Falta un job con PostgreSQL 18 que corra `go test -p 1 ./...`. |
| Formato del frontend | `ProductCategoriesPage.tsx` y `products-client.ts` no pasan `pnpm format:check`, y el CI no corre ese chequeo. |
| Tests del frontend | No hay ninguno; solo `pnpm check`. |
