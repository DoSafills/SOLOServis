"""Escenario de TAL-167 (flujo completo) del visor de llamadas.

Recorre el camino de los datos entre servicios con la última corrida real de
cada tienda, sin escribir nada:

  scraper-api  ->  Ingestion API  ->  catálogo (comercial)  ->  gateway (nginx)

  1. scraper-api: está sano, conoce sus tiendas y su última corrida dejó un
     ingestionRunId (el id que le devolvió Ingestion API).
  2. Ingestion API: esa corrida existe, terminó en success, es de la misma
     tienda y recibió los mismos ítems que el scraper dice haber enviado.
  3. Catálogo: la tienda existe y tiene ofertas actualizadas durante esa
     corrida; el producto de una de ellas está activo, con el mismo precio en
     Pricing.
  4. Gateway: el producto se ve igual por el nginx del frontend y aparece al
     buscarlo por su nombre.

Una falla en un paso indica en qué tramo se corta la comunicación. Los pasos
que necesitan el token de servicio se omiten si el Secret no existe.
"""

from __future__ import annotations

import datetime as dt
import urllib.parse
from decimal import Decimal, InvalidOperation

from origen import Caso, Visor

MAX_TIENDAS = 3  # tiendas a recorrer, de las que tienen corrida con ingestionRunId


def fecha(texto: str | None) -> dt.datetime | None:
    """Convierte una fecha ISO 8601 (con Z u offset) a datetime con zona; None si no se puede."""
    if not texto:
        return None
    try:
        valor = dt.datetime.fromisoformat(texto.replace("Z", "+00:00"))
    except ValueError:
        return None
    return valor if valor.tzinfo else valor.replace(tzinfo=dt.UTC)


def mismo_precio(a, b) -> bool:
    """Compara dos precios que pueden venir como texto ("549990.00") o número."""
    try:
        return Decimal(str(a)) == Decimal(str(b))
    except (InvalidOperation, TypeError):
        return False


def ultima_corrida(corridas: list[dict], tienda: dict) -> dict | None:
    """Devuelve la corrida exitosa más reciente de una tienda que tenga ingestionRunId, o None.

    Solo cuentan las corridas ``success``: una corrida fallida (por ejemplo,
    Mercado Libre, que responde con un desafío JavaScript) no dejó datos que
    recorrer, y tomarla como referencia hacía fallar el visor por un motivo que
    no es de comunicación. scraper-api informa la tienda de cada corrida por el
    nombre del spec (o de la fuente), así que se compara con el slug y con el
    nombre."""
    nombres = {tienda.get("slug"), tienda.get("name")}
    propias = [r for r in corridas if r.get("store") in nombres and r.get("ingestionRunId")
               and r.get("status") == "success"]
    return max(propias, key=lambda r: r.get("startedAt") or "", default=None)


def flujo_completo(v: Visor, ctx: dict) -> None:
    """TAL-167: simula el recorrido de los datos entre servicios y detecta en qué tramo falla."""
    c = lambda *a, **k: v.llamar(Caso("167", *a, **k), ctx)  # noqa: E731

    # 1. scraper-api
    c("scraper-health", "scraper-api está vivo", "scraper", "/health", contiene='"ok"')
    c("scraper-ready", "scraper-api está listo (specs y base propia)", "scraper", "/ready", contiene='"ready"')
    r_tiendas = c("scraper-tiendas", "Tiendas que conoce el scraper", "scraper", "/stores", token="servicio",
                  contiene='"stores"')
    r_corridas = c("scraper-corridas", "Corridas del scraper", "scraper", "/runs", token="servicio",
                   contiene='"runs"')
    if r_tiendas is None or r_corridas is None:
        v.verificar("167", "flujo-sin-token", "Flujo scraper -> ingestion", "sin token de servicio no se puede "
                    "leer el scraper ni Ingestion API; se recorre solo catálogo -> gateway", aviso=True)
        tramo_catalogo(v, ctx, ctx.get("sid"), None, None)
        return
    tiendas = (r_tiendas.json() or {}).get("stores", [])
    corridas = (r_corridas.json() or {}).get("runs", [])
    # Los specs de servicio (issue #191) también aparecen en /stores para que
    # el CronJob los dispare, pero no tienen tienda ni ofertas en el catálogo
    # de productos: se omiten en este recorrido.
    tiendas = [
        tienda
        for tienda in tiendas
        if tienda.get("targetType", "product") == "product"
        and (tienda.get("storeId") or 0) > 0
    ]
    recorridas = 0
    for tienda in tiendas:
        corrida = ultima_corrida(corridas, tienda)
        if corrida is None:
            continue
        recorridas += 1
        if recorridas > MAX_TIENDAS:
            break
        print(f"\n── Tienda {tienda.get('name')} (storeId {tienda.get('storeId')}): corrida {corrida.get('runId')} "
              f"del scraper -> corrida {corrida.get('ingestionRunId')} de ingestion", flush=True)
        tramo_ingestion(v, ctx, tienda, corrida)
    v.verificar("167", "tiendas-con-corrida", "Hay al menos una tienda con corrida entregada a ingestion",
                None if recorridas else "ninguna corrida del scraper tiene ingestionRunId")


def tramo_ingestion(v: Visor, ctx: dict, tienda: dict, corrida: dict) -> None:
    """Paso 2: la corrida del scraper llegó a Ingestion API con los mismos datos.

    Recibe la tienda y la corrida tal como las informa scraper-api."""
    local = dict(ctx, irun=corrida["ingestionRunId"], tsid=tienda.get("storeId"))
    slug = tienda.get("slug", "tienda")
    r = v.llamar(Caso("167", f"ingestion-corrida-{slug}", f"Ingestion tiene la corrida {local['irun']}",
                      "ingestion", "/ingestion/runs/{irun}", token="servicio"), local)
    run = r.json() if r is not None and r.estado == 200 else None
    if not isinstance(run, dict):
        return
    contadores = corrida.get("counters") or {}
    v.verificar("167", f"estado-{slug}", f"{slug}: la corrida terminó en success en los dos lados",
                None if run.get("status") == "success" else
                f"ingestion dice {run.get('status')!r}, el scraper {corrida.get('status')!r}")
    v.verificar("167", f"tienda-{slug}", f"{slug}: scraper e ingestion hablan de la misma tienda",
                None if run.get("storeId") == tienda.get("storeId") else
                f"storeId {run.get('storeId')} en ingestion, {tienda.get('storeId')} en el spec del scraper")
    enviados = contadores.get("itemsSent")
    v.verificar("167", f"items-{slug}", f"{slug}: ingestion recibió los ítems que el scraper envió",
                None if enviados is not None and run.get("recordsFound") == enviados else
                f"scraper envió {enviados}, ingestion registró {run.get('recordsFound')}")
    v.verificar("167", f"fallidos-{slug}", f"{slug}: ingestion no rechazó ítems",
                None if not run.get("recordsFailed") else f"{run.get('recordsFailed')} ítems fallidos", aviso=True)
    tramo_catalogo(v, ctx, tienda.get("storeId"), fecha(run.get("startedAt")), slug)


def tramo_catalogo(v: Visor, ctx: dict, store_id, desde: dt.datetime | None, slug: str | None) -> None:
    """Pasos 3 y 4: lo que dejó la corrida en el catálogo se ve igual por el gateway.

    Recibe el id de la tienda, el inicio de la corrida de ingestion (o None si
    no se pudo leer) y el slug de la tienda para nombrar las llamadas."""
    slug = slug or "tienda"
    local = dict(ctx, tsid=store_id)
    if store_id is None:
        return
    v.llamar(Caso("167", f"catalogo-tienda-{slug}", "La tienda existe en el catálogo", "comercial",
                  "/stores/{tsid}", contiene='"name"'), local)
    r = v.llamar(Caso("167", f"catalogo-ofertas-{slug}", "Ofertas disponibles de la tienda", "comercial",
                      "/stores/{tsid}/offers?available=true&limit=100", forma="lista"), local)
    ofertas = r.json() if r is not None and isinstance(r.json(), list) else []
    if desde is not None:
        # La API entrega lastUpdated en segundos (RFC 3339 sin fracción) y el
        # inicio de la corrida trae microsegundos: una oferta actualizada en el
        # mismo segundo en que empezó la corrida parecería anterior.
        desde = desde.replace(microsecond=0)
        frescas = [o for o in ofertas if (fecha(o.get("lastUpdated")) or dt.datetime.min.replace(
            tzinfo=dt.UTC)) >= desde]
        v.verificar("167", f"ofertas-frescas-{slug}", f"{slug}: hay ofertas actualizadas por la corrida",
                    None if frescas else f"ninguna de las {len(ofertas)} ofertas tiene lastUpdated posterior a "
                                         f"{desde.isoformat()}")
    else:
        frescas = ofertas
    if not frescas:
        return
    oferta = frescas[0]
    local.update(fpid=oferta["productId"], fq=urllib.parse.quote(" ".join(
        str(oferta.get("productName", "")).split()[:5])))
    v.llamar(Caso("167", f"catalogo-producto-{slug}", "Product API: el producto de la oferta", "comercial",
                  "/products/{fpid}", contiene="{fpid}"), local)
    r = v.llamar(Caso("167", f"pricing-ofertas-{slug}", "Pricing API: las ofertas del producto", "comercial",
                      "/offers/product/{fpid}", forma="lista"), local)
    precios = [o for o in (r.json() if r is not None and isinstance(r.json(), list) else [])
               if str(o.get("storeId")) == str(store_id)]
    v.verificar("167", f"precio-{slug}", f"{slug}: Store API y Pricing API dan el mismo precio",
                None if precios and mismo_precio(precios[0].get("price"), oferta.get("price")) else
                f"Store API {oferta.get('price')}, Pricing API "
                f"{precios[0].get('price') if precios else '(sin oferta de la tienda)'}")
    v.llamar(Caso("167", f"directo-producto-{slug}", "Directo a comercial: el producto", "comercial",
                  "/products/{fpid}"), local)
    v.llamar(Caso("167", f"gateway-producto-{slug}", "Por el gateway: el mismo producto", "frontend",
                  "/products/{fpid}", igual_a=f"directo-producto-{slug}"), local)
    v.llamar(Caso("167", f"gateway-busqueda-{slug}", "Por el gateway: el producto aparece al buscar su nombre",
                  "frontend", "/search/products?q={fq}&limit=50", forma="lista", contiene="{fpid}"), local)
    v.llamar(Caso("167", f"gateway-tiendas-{slug}", "Por el gateway: la oferta aparece entre las tiendas",
                  "frontend", "/stores/offers?product={fpid}", forma="lista", contiene="{fpid}"), local)
