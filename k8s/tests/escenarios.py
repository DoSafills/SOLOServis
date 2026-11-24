"""Escenarios de TAL-165 (solicitudes) y TAL-166 (entradas y salidas) del visor de llamadas.

Cada escenario es un Caso de origen.py: a qué servicio va, con qué parámetros
y qué se espera. Los códigos esperados salen del código de develop y se
confirmaron contra el despliegue: un parámetro de consulta inválido es 400
invalid_parameter, un body que no decodifica es 400 (415 o 413 según el caso)
y un body que decodifica pero incumple reglas es 422 validation_failed.

Las escrituras de TAL-166 van solo a Ingestion API y Auth API, y todas se
rechazan al decodificar o validar, antes de la consulta a la base. Por eso:
ningún body válido, nunca un storeId que exista con datos aceptables, y
ningún registro de usuario que pase la validación.
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


def entradas_y_salidas(v: Visor, ctx: dict) -> None:
    """TAL-166: datos válidos, inválidos, incompletos y valores límite; y la forma de cada salida.

    Cada respuesta pasa además por las comprobaciones generales de origen.py:
    X-Request-ID de vuelta, problem+json completo, camelCase y sin detalles internos."""
    c = lambda *a, **k: v.llamar(Caso("166", *a, **k), ctx)  # noqa: E731
    p400 = {"espera": 400, "forma": "problem", "codigo": "invalid_parameter"}
    p404 = {"espera": 404, "forma": "problem", "codigo": "not_found"}

    # Paginación: límites del rango 1..100 y valores que no son números.
    c("limit-minimo", "limit=1, el mínimo válido", "comercial", "/products?limit=1", forma="lista",
      comprueba=largo_exacto(1))
    c("limit-maximo", "limit=100, el máximo válido", "comercial", "/products?limit=100", forma="lista",
      comprueba=largo_maximo(100))
    c("limit-cero", "limit=0, bajo el mínimo", "comercial", "/products?limit=0", **p400)
    c("limit-sobre-maximo", "limit=101, sobre el máximo", "comercial", "/products?limit=101", **p400)
    c("limit-negativo", "limit negativo", "comercial", "/products?limit=-1", **p400)
    c("limit-texto", "limit que no es número", "comercial", "/products?limit=abc", **p400)
    c("limit-decimal", "limit con decimales", "comercial", "/products?limit=1.5", **p400)
    c("offset-negativo", "offset negativo", "comercial", "/products?offset=-1", **p400)
    c("offset-enorme", "offset más allá del último: lista vacía", "comercial", "/products?offset=999999&limit=1",
      forma="lista", comprueba=vacia)
    c("stores-limit", "limit=101 en tiendas", "comercial", "/stores?limit=101", **p400)
    c("search-limit", "limit=101 en la búsqueda", "comercial", "/search/products?limit=101", **p400)
    c("ingestion-limit", "limit=0 en las corridas de ingestion", "ingestion", "/ingestion/runs?limit=0",
      token="servicio", **p400)

    # Identificadores: inexistentes, mal formados y fuera de rango.
    c("producto-inexistente", "Producto con un UUID que no existe", "comercial", f"/products/{UUID_INEXISTENTE}",
      **p404)
    c("producto-id-malo", "Producto con un id que no es UUID", "comercial", "/products/no-es-uuid", **p400)
    c("tienda-inexistente", "Tienda que no existe", "comercial", "/stores/999999", **p404)
    c("tienda-id-texto", "Tienda con id de texto", "comercial", "/stores/abc", **p400)
    c("tienda-id-int32", "Tienda con id mayor que int32", "comercial", "/stores/2147483648", **p400)
    c("oferta-inexistente", "Oferta que no existe", "comercial", "/offers/999999999", **p404)
    c("oferta-id-texto", "Oferta con id de texto", "comercial", "/offers/abc", **p400)
    c("mejor-precio-inexistente", "Mejor precio de un producto que no existe", "comercial",
      f"/offers/product/{UUID_INEXISTENTE}/best", **p404)
    c("servicio-id-malo", "Servicio con id que no es UUID", "comercial", "/services/999999", **p400)
    c("categoria-inexistente", "Categoría que no existe", "comercial", "/categories/no-existe", **p404)
    c("compatibles-id-malo", "Compatibles con id que no es UUID", "comercial", "/comparisons/compatible/no-uuid",
      **p400)
    c("run-id-texto", "Corrida de ingestion con id de texto", "ingestion", "/ingestion/runs/abc", token="servicio",
      **p400)
    c("run-inexistente", "Corrida de ingestion que no existe", "ingestion", f"/ingestion/runs/{RUN_INEXISTENTE}",
      token="servicio", **p404)

    # Filtros: valores fuera del dominio, fechas y combinaciones.
    c("available-malo", "available que no es booleano", "comercial", "/stores/offers?product={pid}&available=si",
      **p400)
    c("product-malo", "Ofertas con product que no es UUID", "comercial", "/stores/offers?product=abc", **p400)
    c("fechas-invertidas", "Historial con from posterior a to", "comercial",
      "/offers/{oid}/price-history?from=2026-12-01&to=2026-01-01", **p400)
    c("fecha-mal-formada", "Historial con una fecha que no es fecha", "comercial",
      "/offers/{oid}/price-history?from=ayer", **p400)
    c("item-type-malo", "itemType fuera de product/service", "comercial", "/categories?itemType=foo", **p400)
    c("sort-malo", "sort con un valor que no existe", "comercial", "/search/products?sort=precio", **p400)

    # Texto de búsqueda: vacío, límite de palabras, largo, caracteres especiales.
    c("q-vacia", "q vacía: búsqueda sin texto", "comercial", "/search/products?q=&limit=1", forma="lista")
    c("q-cinco-palabras", "q con 5 palabras, el máximo", "comercial",
      "/search/products?q=uno+dos+tres+cuatro+cinco&limit=1", forma="lista")
    c("q-seis-palabras", "q con 6 palabras, sobre el máximo", "comercial",
      "/search/products?q=uno+dos+tres+cuatro+cinco+seis&limit=1", **p400)
    c("q-larga", "q de 300 caracteres", "comercial", "/search/products?q=" + "a" * 300, **p400)
    c("q-inyeccion", "q con intento de inyección SQL: se trata como texto", "comercial",
      "/search/products?q=%27%20OR%201%3D1--&limit=3", forma="lista", comprueba=vacia)
    c("q-tildes", "q con ñ y tildes", "comercial", "/search/products?q=ni%C3%B1o%20fr%C3%ADo&limit=1", forma="lista")

    # Comparación: de 2 a 5 productos.
    c("comparar-uno", "Comparar un solo producto", "comercial", "/comparisons/products/compare?ids={pid}", **p400)
    c("comparar-seis", "Comparar seis productos", "comercial",
      "/comparisons/products/compare?ids=" + ",".join(["{pid}"] * 6), **p400)
    c("comparar-id-malo", "Comparar con un id que no es UUID", "comercial",
      "/comparisons/products/compare?ids={pid},no-uuid", **p400)

    # Bodies de Auth API: todos fallan antes de buscar al usuario.
    c("login-vacio", "Login sin email ni password (incompleto)", "usuarios", "/auth/login", metodo="POST", body={},
      espera=422, forma="problem", codigo="validation_failed", comprueba=errores_en("email", "password"))
    c("login-json-roto", "Login con JSON cortado", "usuarios", "/auth/login", metodo="POST", body='{"email":',
      espera=400, forma="problem", codigo="malformed_json")
    c("login-campo-extra", "Login con un campo que no existe", "usuarios", "/auth/login", metodo="POST",
      body={"email": "prueba@visor.test", "password": "x", "rol": "admin"}, espera=400, forma="problem",
      codigo="unknown_field")
    c("login-content-type", "Login con Content-Type text/plain", "usuarios", "/auth/login", metodo="POST",
      body='{"email":"prueba@visor.test","password":"x"}', tipo_body="text/plain", espera=415, forma="problem",
      codigo="unsupported_media_type")
    c("registro-vacio", "Registro sin datos", "usuarios", "/auth/register", metodo="POST", body={}, espera=422,
      forma="problem", codigo="validation_failed")

    # Bodies de Ingestion API con el token de servicio: todos se rechazan antes de la base.
    runs = "/ingestion/runs"
    c("run-sin-token", "Abrir corrida sin token", "ingestion", runs, metodo="POST",
      body={"source": {"name": "visor"}, "storeId": 0}, espera=401, forma="problem", codigo="unauthorized")
    c("run-content-type", "Abrir corrida con Content-Type text/plain", "ingestion", runs, metodo="POST",
      token="servicio", body='{"source":{"name":"visor"},"storeId":0}', tipo_body="text/plain", espera=415,
      forma="problem", codigo="unsupported_media_type")
    c("run-json-roto", "Abrir corrida con JSON cortado", "ingestion", runs, metodo="POST", token="servicio",
      body='{"source":', espera=400, forma="problem", codigo="malformed_json")
    c("run-campo-extra", "Abrir corrida con un campo que no existe", "ingestion", runs, metodo="POST",
      token="servicio", body={"source": {"name": "visor"}, "storeId": 0, "prioridad": 1}, espera=400,
      forma="problem", codigo="unknown_field")
    c("run-tipo-malo", "Abrir corrida con storeId de texto", "ingestion", runs, metodo="POST", token="servicio",
      body={"source": {"name": "visor"}, "storeId": "uno"}, espera=400, forma="problem", codigo="invalid_type")
    c("run-incompleta", "Abrir corrida sin datos", "ingestion", runs, metodo="POST", token="servicio", body={},
      espera=422, forma="problem", codigo="validation_failed", comprueba=errores_en("source.name", "storeId"))
    c("run-nombre-limite", "source.name de 150 caracteres (límite válido) con storeId 0", "ingestion", runs,
      metodo="POST", token="servicio", body={"source": {"name": "v" * 150}, "storeId": 0}, espera=422,
      forma="problem", codigo="validation_failed", comprueba=errores_en("storeId", sin=("source.name",)))
    c("run-nombre-largo", "source.name de 151 caracteres", "ingestion", runs, metodo="POST", token="servicio",
      body={"source": {"name": "v" * 151}, "storeId": 0}, espera=422, forma="problem", codigo="validation_failed",
      comprueba=errores_en("source.name", "storeId"))
    c("run-url-mala", "source.baseUrl que no es http(s)", "ingestion", runs, metodo="POST", token="servicio",
      body={"source": {"name": "visor", "baseUrl": "ftp://visor.test"}, "storeId": 0}, espera=422,
      forma="problem", codigo="validation_failed", comprueba=errores_en("source.baseUrl", "storeId"))
    c("run-body-grande", "Body de más de 1 MB", "ingestion", runs, metodo="POST", token="servicio",
      body='{"source":{"name":"' + "v" * (1 << 20) + '"},"storeId":0}', espera=413, forma="problem",
      codigo="body_too_large")
    items = f"{runs}/{RUN_INEXISTENTE}/items"
    c("items-run-texto", "Enviar ítems a una corrida con id de texto", "ingestion", f"{runs}/abc/items",
      metodo="POST", token="servicio", body={"items": []}, **p400)
    c("items-run-cero", "Enviar ítems a la corrida 0", "ingestion", f"{runs}/0/items", metodo="POST",
      token="servicio", body={"items": []}, **p400)
    c("items-vacio", "Lote sin ítems", "ingestion", items, metodo="POST", token="servicio", body={"items": []},
      espera=422, forma="problem", codigo="validation_failed", comprueba=errores_en("items"))
    c("items-101", "Lote de 101 ítems, sobre el máximo de 100", "ingestion", items, metodo="POST", token="servicio",
      body={"items": [{"externalId": f"visor-{i}"} for i in range(101)]}, espera=422, forma="problem",
      codigo="validation_failed", comprueba=errores_en("items"))
    c("items-run-inexistente", "Lote válido para una corrida que no existe", "ingestion", items, metodo="POST",
      token="servicio", body={"items": [{"externalId": "visor-1", "product": {"name": "Producto del visor"}}]},
      **p404)
    c("finish-estado-malo", "Cerrar una corrida con un estado que no existe", "ingestion",
      f"{runs}/{RUN_INEXISTENTE}/finish", metodo="POST", token="servicio", body={"status": "terminado"},
      espera=422, forma="problem", codigo="validation_failed", comprueba=errores_en("status"))
