"""Escenarios de TAL-165 (solicitudes) del visor de llamadas.

Cada escenario es un Caso de origen.py: a qué servicio va, con qué parámetros
y qué se espera. Los códigos esperados salen del código de develop y se
confirmaron contra el despliegue: un parámetro de consulta inválido es 400
invalid_parameter, un body que no decodifica es 400 (415 o 413 según el caso)
y un body que decodifica pero incumple reglas es 422 validation_failed.
"""

from __future__ import annotations

from decimal import Decimal, InvalidOperation

from origen import Caso, Visor

UUID_INEXISTENTE = "00000000-0000-0000-0000-000000000000"
RUN_INEXISTENTE = 2147483647  # máximo int32: ninguna corrida real llega a ese id


def lista_de(resp) -> list:
    """Devuelve el body como lista; si es {"items": [...]}, la lista de items."""
    datos = resp.json()
    return datos.get("items", []) if isinstance(datos, dict) else datos if isinstance(datos, list) else []


def precio(item: dict):
    """Precio de un producto u oferta para comparar: bestPrice, minPrice, price o totalPrice."""
    for clave in ("bestPrice", "minPrice", "price", "totalPrice"):
        valor = item.get(clave)
        if isinstance(valor, dict):
            valor = valor.get("price") or valor.get("totalPrice")
        if valor is None:
            continue
        try:
            return Decimal(str(valor))
        except InvalidOperation:
            return None
    return None


def largo_maximo(n: int):
    """Comprobación: la lista tiene como máximo `n` elementos."""
    return lambda resp, ctx: None if len(lista_de(resp)) <= n else f"vinieron {len(lista_de(resp))}, máximo {n}"


def largo_exacto(n: int):
    """Comprobación: la lista tiene exactamente `n` elementos."""
    return lambda resp, ctx: None if len(lista_de(resp)) == n else f"vinieron {len(lista_de(resp))}, quiero {n}"


def vacia(resp, ctx):
    """Comprobación: la lista viene vacía."""
    return None if lista_de(resp) == [] else f"quiero [], vinieron {len(lista_de(resp))}"


def ordenada(descendente: bool):
    """Comprobación: los precios de la lista van ordenados (ascendente o descendente)."""
    def revisar(resp, ctx):
        """Revisa el orden de los precios que tengan valor."""
        precios = [p for p in (precio(i) for i in lista_de(resp)) if p is not None]
        return None if precios == sorted(precios, reverse=descendente) else f"precios fuera de orden: {precios[:5]}"
    return revisar


def todas_disponibles(resp, ctx):
    """Comprobación: todas las ofertas tienen available=true."""
    malas = [o for o in lista_de(resp) if o.get("available") is not True]
    return None if not malas else f"{len(malas)} ofertas con available distinto de true"


def errores_en(*campos: str, sin: tuple[str, ...] = ()):
    """Comprobación: errors[] del 422 nombra cada campo de `campos` y ninguno de `sin`."""
    def revisar(resp, ctx):
        """Lee los campos de errors[] y los compara."""
        datos = resp.json()
        nombrados = {e.get("field") for e in datos.get("errors", [])} if isinstance(datos, dict) else set()
        faltan = [c for c in campos if c not in nombrados]
        sobran = [c for c in sin if c in nombrados]
        if faltan:
            return f"errors[] no nombra {', '.join(faltan)} (nombra {', '.join(sorted(map(str, nombrados)))})"
        return f"errors[] rechaza {', '.join(sobran)}, que es válido" if sobran else None
    return revisar


def solicitudes(v: Visor, ctx: dict) -> None:
    """TAL-165: simula solicitudes a cada API con distintos parámetros y escenarios."""
    c = lambda *a, **k: v.llamar(Caso("165", *a, **k), ctx)  # noqa: E731

    # 1. Una lectura por API pública, con sus parámetros.
    c("productos-pagina", "Productos con limit y offset", "comercial", "/products?limit=2&offset=0", forma="lista",
      total=True, comprueba=largo_maximo(2))
    c("productos-pagina-2", "Segunda página: empieza donde terminó la primera", "comercial",
      "/products?limit=1&offset=1", forma="lista",
      comprueba=lambda r, x: None if not lista_de(r) or lista_de(r)[0]["id"] == x.get("pid2") else
      "el primero de offset=1 no es el segundo de offset=0")
    c("productos-categoria", "Productos filtrados por categoría", "comercial",
      "/products?category=refrigeradores&limit=3", forma="lista", comprueba=largo_maximo(3))
    c("producto-detalle", "Detalle de un producto", "comercial", "/products/{pid}", contiene="{pid}")
    c("tiendas", "Tiendas paginadas", "comercial", "/stores?limit=2", forma="lista", total=True,
      comprueba=largo_maximo(2))
    c("tienda-detalle", "Tienda con sus productos y sucursales", "comercial", "/stores/{sid}",
      contiene='"locations"')
    c("tienda-ofertas", "Ofertas disponibles de una tienda", "comercial", "/stores/{sid}/offers?available=true&limit=3",
      forma="lista", comprueba=todas_disponibles)
    c("ofertas-producto-tiendas", "Ofertas de un producto en todas las tiendas", "comercial",
      "/stores/offers?product={pid}&limit=5", forma="lista")
    c("oferta", "Una oferta", "comercial", "/offers/{oid}", contiene='"price"')
    c("mejor-precio", "Mejor precio actual de un producto", "comercial", "/offers/product/{pid}/best",
      contiene='"price"')
    c("historial-oferta", "Historial de precios de una oferta", "comercial", "/offers/{oid}/price-history?limit=2",
      forma="lista", total=True, comprueba=largo_maximo(2))
    c("historial-rango", "Historial del producto entre dos fechas", "comercial",
      "/offers/product/{pid}/price-history?from=2026-01-01&to=2026-12-31&limit=5", forma="lista", total=True)
    c("buscar-filtros", "Búsqueda con texto, categoría y orden por precio", "comercial",
      "/search/products?q=refrigerador&category=refrigeradores&sort=price_asc&limit=5", forma="lista", total=True,
      comprueba=ordenada(False))
    c("buscar-desc", "Búsqueda ordenada de mayor a menor precio", "comercial",
      "/search/products?category=refrigeradores&sort=price_desc&limit=5", forma="lista", comprueba=ordenada(True))
    c("buscar-atributo", "Búsqueda por atributo (capacidad mínima)", "comercial",
      "/search/products?category=refrigeradores&attr.capacidad_litros.min=300&limit=3", forma="lista")
    c("facetas", "Filtros disponibles de una categoría", "comercial", "/search/facets?category=refrigeradores",
      contiene='"brands"')
    c("buscar-categorias", "Búsqueda de categorías sin distinguir mayúsculas", "comercial",
      "/search/categories?q=OPTICA", forma="lista", contiene="internet-fibra-optica")
    c("buscar-marcas", "Búsqueda de marcas por prefijo", "comercial", "/search/brands?q=sam", forma="lista",
      contiene="Samsung")
    c("comparar", "Comparar dos productos", "comercial", "/comparisons/products/compare?ids={pid},{pid2}",
      contiene='"products"')
    c("compatibles", "Alternativas de la misma categoría", "comercial", "/comparisons/compatible/{pid}?limit=3",
      forma="lista", total=True, comprueba=largo_maximo(3))
    c("categorias", "Categorías paginadas", "comercial", "/categories?limit=2", contiene='"items"')
    c("categorias-tipo", "Categorías de servicios", "comercial", "/categories?itemType=service",
      comprueba=lambda r, x: None if all(i.get("itemType") == "service" for i in lista_de(r)) else
      "vino una categoría que no es de servicios")
    c("categoria-detalle", "Una categoría por slug", "comercial", "/categories/refrigeradores",
      contiene='"refrigeradores"')
    c("arbol-categorias", "Árbol de categorías antiguo", "comercial", "/product-categories", forma="lista")
    c("servicios", "Servicios (Service API no pagina)", "comercial", "/services", forma="lista")
    c("servicio-detalle", "Detalle de un servicio", "comercial", "/services/{svc}", contiene="{svc}")
    c("resenas", "Reseñas de un producto (Review API)", "comercial", "/reviews/product/{pid}", forma="lista")
    c("resenas-producto", "Reseñas de un producto (Product API)", "comercial", "/products/{pid}/reviews",
      forma="lista")
    c("usuarios", "User API responde", "usuarios", "/users", contiene="user-api-ready")

    # 2. La misma solicitud por el gateway (nginx del frontend) y directo al dominio.
    for slug, ruta in (("producto", "/products/{pid}"), ("buscar", "/search/products?q=refrigerador&limit=3"),
                       ("tiendas", "/stores?limit=2"), ("categorias", "/categories?limit=2"),
                       ("compatibles", "/comparisons/compatible/{pid}?limit=2"),
                       ("resenas", "/reviews/product/{pid}")):
        c(f"directo-{slug}", f"Directo a comercial: {ruta}", "comercial", ruta)
        c(f"gateway-{slug}", f"Por el gateway, igual que directo: {ruta}", "frontend", ruta,
          igual_a=f"directo-{slug}")
    c("directo-auth", "Directo a usuarios: /auth/validate sin token", "usuarios", "/auth/validate", espera=401,
      forma="problem", codigo="unauthorized")
    c("gateway-auth", "Por el gateway: /auth llega a usuarios", "frontend", "/auth/validate", espera=401,
      forma="problem", codigo="unauthorized", igual_a="directo-auth")

    # 3. Lo que el gateway no deja pasar: escrituras públicas y APIs internas.
    for metodo, ruta in (("POST", "/products"), ("PUT", "/products/{pid}"), ("DELETE", "/stores/{sid}"),
                         ("PUT", "/auth/login")):
        c(f"gateway-bloquea-{metodo}", f"El gateway bloquea {metodo} {ruta}", "frontend", ruta, metodo=metodo,
          espera=403, forma="html", body={} if metodo != "DELETE" else None, salidas=False)
    for ruta in ("/ingestion/runs", "/notifications", "/users", "/offers/product/{pid}"):
        c(f"gateway-no-expone-{ruta.split('/')[1]}", f"El gateway no expone {ruta}: responde la app",
          "frontend", ruta, forma="html", contiene='id="root"', salidas=False)

    # 4. Autenticación: sin token, con token falso y con el token de servicio.
    for servicio, ruta in (("comercial", "/favorites"), ("comercial", "/cart"), ("comercial", "/watchlist"),
                           ("comercial", "/notifications"), ("comercial", "/comparisons/products"),
                           ("ingestion", "/ingestion/runs")):
        c(f"sin-token{ruta.replace('/', '-')}", f"{ruta} sin token", servicio, ruta, espera=401, forma="problem",
          codigo="unauthorized")
        c(f"token-falso{ruta.replace('/', '-')}", f"{ruta} con un token que no existe", servicio, ruta,
          token="invalido", espera=401, forma="problem", codigo="unauthorized")
    c("userid-no-basta", "El userId del cliente no reemplaza la sesión", "comercial", "/watchlist?userId=1",
      espera=401, forma="problem", codigo="unauthorized")
    c("token-servicio", "Ingestion con el token de servicio", "ingestion", "/ingestion/runs?limit=2",
      token="servicio", forma="lista", total=True)
    c("scraper-sin-token", "API del scraper sin token", "scraper", "/runs", espera=401, forma="texto")
    c("scraper-token", "API del scraper con el token de servicio", "scraper", "/stores", token="servicio",
      contiene='"stores"')

    # 5. Varias solicitudes a la vez por el gateway.
    v.rafaga(Caso("165", "rafaga", "10 lecturas simultáneas por el gateway", "frontend", "/products?limit=3",
                  forma="lista"), ctx, 10)

