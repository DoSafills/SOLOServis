import { useEffect, useMemo, useState } from "react";
import type { Page, Product, Store, StoreDetail, StoreOffer, StoreProduct } from "../../types";
import { getStores, getStoreById } from "../../services/api/api";
import { Badge, Breadcrumb, EmptyState, Rating } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
  storeId?: string;
}

function StoreLogo({
  name,
  logo,
  className,
  fallbackClassName = "text-sm",
}: {
  name: string;
  logo: string;
  className: string;
  fallbackClassName?: string;
}) {
  const [imageFailed, setImageFailed] = useState(false);

  return (
    <div
      className={`flex shrink-0 items-center justify-center overflow-hidden rounded-xl border border-bd bg-surface-2 ${className}`}
    >
      {logo && !imageFailed ? (
        <img
          src={logo}
          alt={`${name} logo`}
          className="h-full w-full object-contain p-1.5"
          onError={() => setImageFailed(true)}
        />
      ) : (
        <span className={`px-2 font-bold text-prime ${fallbackClassName}`}>
          {name.slice(0, 2).toLocaleUpperCase("es")}
        </span>
      )}
    </div>
  );
}

function StoreList({ navigate }: { navigate: (page: Page) => void }) {
  const [stores, setStores] = useState<Store[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [query, setQuery] = useState("");
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let cancelled = false;

    async function loadStores() {
      setLoading(true);
      setError(null);

      try {
        const result = await getStores();

        if (cancelled) return;

        setStores(result);
      } catch (err) {
        if (cancelled) return;

        setError(err instanceof Error ? err.message : "Error al cargar las tiendas");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadStores();

    return () => {
      cancelled = true;
    };
  }, [reload]);

  const filteredStores = stores.filter((store) =>
    store.name.toLocaleLowerCase().includes(query.trim().toLocaleLowerCase()),
  );

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:py-12">
      <Breadcrumb
        items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Tiendas" }]}
      />

      <header className="mt-6 mb-7 flex flex-col gap-4 border-b border-bd pb-6 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="mb-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-prime">
            Directorio
          </p>
          <h1 className="text-xl font-semibold tracking-tight text-text sm:text-2xl">
            Tiendas y proveedores
          </h1>
          <p className="mt-1.5 text-sm text-muted">
            Compara tiendas, despacho y reputación en un solo lugar.
          </p>
        </div>

        <div className="w-full sm:w-auto">
          <label className="sr-only" htmlFor="store-search">
            Buscar tienda o proveedor
          </label>
          <input
            id="store-search"
            type="search"
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Buscar tienda o proveedor..."
            maxLength={100}
            className="w-full rounded-lg border border-bd bg-surface-2 px-3.5 py-2.5 text-sm text-text outline-none transition-colors placeholder:text-muted focus:border-prime sm:w-80"
          />
        </div>
      </header>

      {loading ? (
        <div className="text-center py-20">
          <p className="text-muted">Cargando tiendas...</p>
        </div>
      ) : error ? (
        <div className="rounded-2xl border border-bd bg-surface p-8 text-center" role="alert">
          <p className="text-sm text-warn">{error}</p>
          <button
            type="button"
            onClick={() => setReload((attempt) => attempt + 1)}
            className="mt-4 rounded-lg border border-prime/40 bg-prime-muted px-4 py-2 text-sm font-semibold text-prime transition hover:border-prime hover:bg-prime/20"
          >
            Reintentar carga de Store API
          </button>
        </div>
      ) : filteredStores.length === 0 ? (
        <EmptyState
          title={stores.length === 0 ? "No hay tiendas disponibles" : "No encontramos tiendas"}
          description={
            stores.length === 0
              ? "La Store API no devolvió tiendas para mostrar."
              : `No hay tiendas que coincidan con "${query}".`
          }
          action={query ? { label: "Limpiar búsqueda", onClick: () => setQuery("") } : undefined}
        />
      ) : (
        <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
          {filteredStores.map((store) => (
            <button
              key={store.id}
              onClick={() => navigate({ id: "store-detail", storeId: store.id })}
              className="group flex h-full flex-col rounded-2xl border border-bd bg-surface p-5 text-left shadow-lg shadow-black/15 transition-all duration-300 hover:-translate-y-1 hover:border-prime/50 hover:shadow-xl hover:shadow-black/25 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime"
            >
              <div className="flex min-w-0 items-center gap-3">
                <StoreLogo name={store.name} logo={store.logo} className="h-14 w-14" />

                <div className="min-w-0 flex-1">
                  <h2 className="truncate text-base font-semibold text-text transition-colors group-hover:text-prime">
                    {store.name}
                  </h2>
                  <Badge variant={store.reputation === "Excelente" ? "best" : "available"}>
                    {store.reputation}
                  </Badge>
                </div>
              </div>

              <div className="mt-4">
                <Rating value={store.rating} count={store.reviewCount} />
              </div>

              <div className="mt-4 space-y-2 border-t border-bd pt-4">
                <div className="flex items-center justify-between gap-4 text-xs">
                  <span className="text-muted">Productos</span>
                  <span className="font-semibold text-text">
                    {store.productCount.toLocaleString("es-CL")}
                  </span>
                </div>

                <div className="flex items-start justify-between gap-4 text-xs">
                  <span className="text-muted">Despacho</span>
                  <span className="text-right font-medium text-text">{store.dispatchTime}</span>
                </div>

                <div className="flex items-center justify-between gap-4 text-xs">
                  <span className="text-muted">Sitio</span>
                  <span
                    className="max-w-[70%] truncate font-semibold text-prime"
                    title={store.website}
                  >
                    {store.website}
                  </span>
                </div>
              </div>

              <span className="mt-auto flex items-center justify-between border-t border-bd pt-4 text-xs font-semibold text-prime">
                Ver tienda
                <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" className="h-4 w-4">
                  <path
                    d="M4.5 10h11m-4.5-4.5L15.5 10 11 14.5"
                    stroke="currentColor"
                    strokeWidth="1.7"
                    strokeLinecap="round"
                    strokeLinejoin="round"
                  />
                </svg>
              </span>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function StoreProductCard({
  product,
  storeId,
  storeName,
  onAddToCart,
}: {
  product: StoreProduct;
  storeId: string;
  storeName: string;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
}) {
  const [adding, setAdding] = useState(false);
  const [message, setMessage] = useState<string | null>(null);
  const [imageFailed, setImageFailed] = useState(false);
  const mappedProduct: Product = {
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    category: product.category,
    subcategory: "",
    image: product.image,
    images: [],
    description: product.description,
    rating: 0,
    reviewCount: 0,
    specs: {},
    offers: [],
    priceHistory: [],
    offerPriceHistory: [],
    tags: [],
  };

  const mappedOffer: StoreOffer = {
    offerId: product.offerId,
    storeId,
    storeName,
    price: product.price,
    listPrice: product.listPrice,
    available: product.available,
    shipping: product.shippingCost,
    shippingFree: product.shippingFree,
    stock: product.stock,
    condition: product.condition,
    url: product.productUrl,
    lastUpdated: product.lastUpdated,
  };

  return (
    <div className="group relative isolate flex min-h-[22rem] flex-col overflow-hidden rounded-[1.5rem] border border-bd bg-surface shadow-lg shadow-black/20 transition-all duration-300 hover:-translate-y-1 hover:border-prime/50 hover:shadow-2xl hover:shadow-black/30">
      <div className="absolute inset-y-0 left-0 w-[58%] bg-slate-950/60">
        {product.image && !imageFailed ? (
          <img
            src={product.image}
            alt={product.name}
            loading="lazy"
            className="h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-[1.03]"
            onError={() => setImageFailed(true)}
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm font-medium text-muted">
            Sin imagen
          </div>
        )}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-gradient-to-r from-transparent via-transparent to-slate-950/25"
        />
      </div>

      <div
        aria-hidden="true"
        className="absolute inset-y-0 right-0 w-[66%]"
        style={{
          background: "linear-gradient(135deg, #343B40 0%, #22282C 42%, #191D20 100%)",
          clipPath: "polygon(18% 0, 100% 0, 100% 100%, 0 100%)",
        }}
      />

      <div className="relative z-10 ml-[38%] flex w-[62%] min-w-0 flex-1 flex-col justify-center gap-2 p-3 pl-8 sm:gap-2.5 sm:pl-10">
        <div className="min-w-0">
          <p className="mb-1 text-[9px] font-semibold uppercase tracking-[0.16em] text-prime">
            {product.brand || "Sin marca"}
          </p>
          <h3 className="line-clamp-2 text-sm font-bold leading-snug text-text transition-colors group-hover:text-prime">
            {product.name}
          </h3>
          <div className="mt-1.5">
            <Badge variant={product.available ? "available" : "unavailable"}>
              {product.available ? "Disponible" : "No disponible"}
            </Badge>
          </div>
        </div>

        <div className="border-t border-bd pt-2">
          <div className="text-lg font-bold tracking-tight text-prime">
            {Number(product.price).toLocaleString("es-CL")} {product.currency}
          </div>

          {Number(product.listPrice) > Number(product.price) && (
            <div className="mt-0.5 text-[10px] text-muted line-through">
              {Number(product.listPrice).toLocaleString("es-CL")} {product.currency}
            </div>
          )}
        </div>

        <div className="space-y-1.5 border-t border-bd pt-2">
          <div className="flex justify-between gap-2 text-[10px]">
            <span className="shrink-0 text-muted">Despacho</span>
            <span className="truncate text-right font-medium text-muted-2">
              {product.shippingFree
                ? "Gratis"
                : `${Number(product.shippingCost).toLocaleString("es-CL")} ${product.currency}`}
            </span>
          </div>

          <div className="flex justify-between gap-2 text-[10px]">
            <span className="shrink-0 text-muted">Stock</span>
            <span className="truncate text-right font-medium text-muted-2">
              {product.stock === null ? "No informado" : product.stock}
            </span>
          </div>

          <div className="flex justify-between gap-2 text-[10px]">
            <span className="shrink-0 text-muted">Modelo</span>
            <span className="truncate text-right font-medium text-muted-2">
              {product.model || "—"}
            </span>
          </div>
          <div className="flex justify-between gap-2 text-[10px]">
            <span className="shrink-0 text-muted">Actualización</span>
            <span
              className="truncate text-right font-medium text-muted-2"
              title={
                product.lastUpdated
                  ? new Date(product.lastUpdated).toLocaleString("es-CL")
                  : undefined
              }
            >
              {product.lastUpdated
                ? new Date(product.lastUpdated).toLocaleString("es-CL")
                : "No informada"}
            </span>
          </div>
        </div>

        <button
          type="button"
          disabled={!product.available || product.offerId <= 0 || adding}
          onClick={() => {
            setAdding(true);
            setMessage(null);
            void onAddToCart(mappedProduct, mappedOffer)
              .then((added) =>
                setMessage(added ? "Oferta agregada a la cesta." : "No se pudo agregar la oferta."),
              )
              .catch(() => setMessage("No se pudo conectar para agregar la oferta."))
              .finally(() => setAdding(false));
          }}
          className="w-full rounded-lg bg-prime px-2 py-2 text-[10px] font-semibold text-[#181714] shadow-lg shadow-black/20 transition-all hover:-translate-y-0.5 hover:bg-prime-dark hover:shadow-xl active:translate-y-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-800/70 disabled:text-slate-400 disabled:shadow-none"
        >
          {adding ? "Agregando..." : "Agregar al carrito"}
        </button>
        {message && (
          <p role="status" className="text-[10px] text-muted">
            {message}
          </p>
        )}
      </div>
    </div>
  );
}

function StoreDetail({ storeId, navigate, onAddToCart }: Required<Props>) {
  const [store, setStore] = useState<StoreDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [productSearch, setProductSearch] = useState("");
  const [reload, setReload] = useState(0);

  useEffect(() => {
    let cancelled = false;

    async function loadStore() {
      setLoading(true);
      setError(null);

      try {
        const result = await getStoreById(storeId);

        if (cancelled) return;

        setStore(result);
      } catch (err) {
        if (cancelled) return;

        setError(err instanceof Error ? err.message : "Error al cargar la tienda");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadStore();

    return () => {
      cancelled = true;
    };
  }, [storeId, reload]);

  const filteredProducts = useMemo(() => {
    if (!store) return [];

    const search = productSearch.trim().toLowerCase();

    if (!search) {
      return store.products;
    }

    return store.products.filter((product) => product.name.toLowerCase().includes(search));
  }, [store, productSearch]);

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8 sm:py-12">
        <p className="py-8 text-center text-sm text-muted">Cargando tienda...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8 sm:py-12">
        <div className="rounded-2xl border border-bd bg-surface p-8 text-center">
          <p role="alert" className="text-sm text-warn">
            {error}
          </p>
          <button
            type="button"
            onClick={() => setReload((attempt) => attempt + 1)}
            className="mt-4 rounded-lg border border-prime/40 bg-prime-muted px-4 py-2 text-sm font-semibold text-prime transition hover:border-prime hover:bg-prime/20"
          >
            Reintentar carga de Store API
          </button>
        </div>
      </div>
    );
  }

  if (!store) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8 sm:py-12">
        <EmptyState title="Tienda no encontrada" description="No pudimos encontrar esta tienda." />
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:py-12">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Tiendas", onClick: () => navigate({ id: "stores" }) },
          { label: store.name },
        ]}
      />

      <div className="mb-8 flex flex-col overflow-hidden rounded-[1.5rem] border border-bd bg-surface shadow-lg shadow-black/20 md:flex-row">
        <div className="flex h-40 w-full shrink-0 items-center justify-center overflow-hidden border-b border-bd bg-surface-2 p-6 md:h-auto md:min-h-[220px] md:w-64 md:border-b-0 md:border-r">
          <StoreLogo
            name={store.name}
            logo={store.logo}
            className="h-full w-full rounded-none border-0 bg-transparent"
            fallbackClassName="text-5xl"
          />
        </div>

        <div className="flex min-w-0 flex-1 flex-col justify-between gap-6 p-5 sm:p-6 md:flex-row">
          <div className="min-w-0 flex-1">
            <div className="mb-2 flex flex-wrap items-center gap-3">
              <h1 className="text-xl font-semibold tracking-tight text-text sm:text-2xl">
                {store.name}
              </h1>
              <Badge variant={store.reputation === "Excelente" ? "best" : "available"}>
                {store.reputation}
              </Badge>
            </div>

            <div className="flex flex-wrap items-center gap-3">
              <Rating value={store.rating} count={store.reviewCount} />
              {store.website && (
                <a
                  href={store.website}
                  target="_blank"
                  rel="noreferrer"
                  className="text-xs font-semibold text-prime transition-colors hover:text-prime-dark hover:underline"
                >
                  Visitar sitio
                </a>
              )}
            </div>

            <p className="mt-3 text-sm text-muted">{store.conditions}</p>
          </div>

          <div className="grid shrink-0 grid-cols-2 gap-3 text-center md:w-64">
            <div className="rounded-xl border border-bd bg-surface-2 px-4 py-3">
              <div className="price text-lg font-bold text-prime">
                {store.productCount.toLocaleString("es-CL")}
              </div>
              <div className="text-xs text-muted">Productos</div>
            </div>

            <div className="rounded-xl border border-bd bg-surface-2 px-4 py-3">
              <div className="text-sm font-bold text-text">{store.dispatchTime}</div>
              <div className="mt-1 text-xs text-muted">Despacho</div>
            </div>
          </div>
        </div>
      </div>

      <section className="mb-5 flex flex-col gap-4 border-b border-bd pb-5 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="mb-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-prime">
            Catálogo de tienda
          </p>
          <h2 className="text-lg font-semibold tracking-tight text-text">
            Productos disponibles en {store.name}
          </h2>
        </div>
        <div className="w-full sm:w-80">
          <label className="sr-only" htmlFor="store-product-search">
            Buscar producto por nombre
          </label>
          <input
            id="store-product-search"
            type="search"
            value={productSearch}
            onChange={(event) => setProductSearch(event.target.value)}
            placeholder="Buscar producto por nombre..."
            maxLength={100}
            className="w-full rounded-lg border border-bd bg-surface-2 px-3.5 py-2.5 text-sm text-text outline-none transition-colors placeholder:text-muted focus:border-prime"
          />
        </div>
      </section>

      <div className="mb-4 flex items-center justify-between">
        <p className="text-xs text-muted">
          {filteredProducts.length} {filteredProducts.length === 1 ? "producto" : "productos"}
        </p>
      </div>

      {filteredProducts.length === 0 ? (
        <EmptyState
          title="No se encontraron productos"
          description="Prueba con otro nombre de producto."
          action={
            productSearch
              ? { label: "Limpiar búsqueda", onClick: () => setProductSearch("") }
              : undefined
          }
        />
      ) : (
        <div className="mb-10 grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3 2xl:grid-cols-4">
          {filteredProducts.map((product) => (
            <StoreProductCard
              key={product.offerId}
              product={product}
              storeId={store.id}
              storeName={store.name}
              onAddToCart={onAddToCart}
            />
          ))}
        </div>
      )}

      <section className="mt-8">
        <div className="mb-4 border-b border-bd pb-4">
          <p className="mb-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-prime">
            Información
          </p>
          <h2 className="text-lg font-semibold tracking-tight text-text">
            Ubicaciones de {store.name}
          </h2>
        </div>

        {store.locations.length === 0 ? (
          <p className="rounded-xl border border-bd bg-surface p-4 text-sm text-muted">
            No hay ubicaciones disponibles.
          </p>
        ) : (
          <div className="grid grid-cols-1 gap-4 md:grid-cols-2">
            {store.locations.map((location) => (
              <div
                key={location.id}
                className="rounded-2xl border border-bd bg-surface p-5 shadow-lg shadow-black/10"
              >
                <div className="text-sm font-semibold text-text">
                  {location.address || "Dirección no disponible"}
                </div>

                <div className="mt-2 text-sm text-muted">
                  {[
                    location.commune,
                    location.city,
                    location.region,
                    location.postalCode,
                    location.country,
                  ]
                    .filter(Boolean)
                    .join(", ")}
                </div>
              </div>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}

export default function StoresPage(props: Props) {
  if (props.storeId) {
    return <StoreDetail {...props} storeId={props.storeId} />;
  }

  return <StoreList navigate={props.navigate} />;
}
