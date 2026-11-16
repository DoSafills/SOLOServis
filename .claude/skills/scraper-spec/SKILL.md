---
name: scraper-spec
description: Use when changing anything under scraper/, adding a store, editing stores/<slug>/spec.json, extraction rules, the pipeline, SQLite state or the ingestion delivery. Covers spec-first rules, the item contract and the verification commands.
---

# Scraper v2 — `scraper/`

El scraper es **spec-first**: una tienda nueva es un archivo
`stores/<slug>/spec.json`, no código Python. Documentación completa del formato
en `scraper/README.md`; contrato de datos en
`docs/estandares/formato-datos-extraidos.md`.

## Mapa

| Ruta | Qué hay |
| --- | --- |
| `stores/<slug>/spec.json` | Spec de cada tienda: campos, paginación, descubrimiento, `match.detect` |
| `src/scraper/pipeline.py` | Orquestación: fetch → extract → dedupe → ingesta → cierre del run |
| `src/scraper/fetch/` | Cliente HTTP con rate limit y reintentos |
| `src/scraper/extract/` | Motor de extracción y transforms |
| `src/scraper/ingestion.py` | Cliente de la Ingestion API (lotes de 1–100) |
| `src/scraper/db/repository.py` | SQLite: `DATA_SOURCE`, `SCRAPER_CONFIG`, `SCRAPE_RUN`, `SCRAPED_DATA` |
| `src/scraper/api/app.py` | API de control: `/runs`, `/tasks`, `/stores`, `/health`, `/ready` |
| `tests/` | `pytest`; `conftest.py` trae la fixture `settings` y `fixture_html` |

## Reglas

1. **No hay campos libres.** Los `ValueSpec` apuntan al contrato de ingestion
   (`product.name`, `offer.price`, `product.attributes.<clave>`). La API
   **rechaza campos desconocidos** y exige los atributos obligatorios de la
   categoría (por ejemplo `capacidad_litros` en `refrigeradores`).
2. **`storeId` consistente.** Debe coincidir con la tabla `store` del backend y
   con el SQLite del scraper. Está fijado por test en `tests/test_spec.py`.
3. **Un fallo de ingesta no aborta el crawl.** Se registra `ingestionError`, los
   ítems no enviados van a `out/rejected/run-{id}.jsonl` y el run local cierra
   con el motivo real.
4. **Contadores separados:** `records_*` son páginas; `items_*` son productos.
   Una página de listado puede producir N productos.
5. **No se saltan protecciones anti-bots.** Mercado Libre redirige a su
   verificación de cuenta: se deja fallando y se documenta.
6. **Reintentos:** red, timeout y `408/425/429/500/502/503/504`. Un `404` no se
   reintenta. Tope en `max_retries` con espera exponencial.

## Añadir una tienda

1. Crear `stores/<slug>/spec.json` copiando la forma de Falabella o Ripley.
2. `storeId`, `match.detect` y el `crawl` de descubrimiento.
3. Validar el spec antes de cualquier otra cosa:
   `./venv/bin/python -m scraper.cli spec stores/<slug>/spec.json` (o el
   comando que documente el README vigente).
4. Correr una página contra el sitio real en modo foreground.
5. `pytest tests/test_spec.py` — verifica unicidad y que los IDs estén fijados.

## Comandos

```bash
cd scraper
./venv/bin/python -m pytest -q
./venv/bin/ruff check src tests
./venv/bin/python -m pytest tests/test_pipeline.py -q -k dedupe
```

El venv del repo está en `scraper/venv` (Python 3.12). Si no existe:
`python3.12 -m venv venv && ./venv/bin/pip install -e '.[dev]'`.

## Antes de abrir el PR

- [ ] `pytest -q` en verde y `ruff check src tests` limpio.
- [ ] Si tocaste el spec, `tests/test_spec.py` sigue pasando.
- [ ] Si tocaste la ingesta, los tests de `test_ingestion.py` y
      `test_e2e_ingestion.py` pasan.
- [ ] `scraper/README.md` refleja el cambio si es de formato o de operación.

## Anti-patrones

- Parsear con lógica en Python lo que el spec puede declarar.
- Reintentar un `404` o un spec inválido.
- Suponer que la Ingestion API acepta cualquier atributo.
- Abrir un run remoto sin `store_id` o sin token y esperar que funcione.
