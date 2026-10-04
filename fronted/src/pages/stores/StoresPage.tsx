import { useEffect, useMemo, useState } from "react";
import type { Page, Store, StoreDetail, StoreProduct } from "../../types";
import { getStores, getStoreById } from "../../services/api/api";
import { Badge, Breadcrumb, Rating } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
  storeId?: string;
}

function StoreList({ navigate }: { navigate: (page: Page) => void }) {
  const [stores, setStores] = useState<Store[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

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
  }, []);

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Tiendas" }]}
      />

      <h1 className="text-2xl font-bold text-text mb-2">Tiendas y proveedores</h1>

      <p className="text-sm text-muted mb-8">
        Directorio de tiendas comparadas en nuestra plataforma
      </p>

      {loading ? (
        <div className="text-center py-20">
          <p className="text-muted">Cargando tiendas...</p>
        </div>
      ) : error ? (
        <div className="text-center py-20">
          <p className="text-warn">{error}</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {stores.map((store) => (
            <button
              key={store.id}
              onClick={() => navigate({ id: "store-detail", storeId: store.id })}
              style={{ background: "#111111", border: "1px solid #2A2A2A" }}
              className="rounded-2xl p-5 text-left hover:border-prime transition-all duration-200 group"
            >
              <div className="flex items-center gap-3 mb-4">
                <div
                  style={{
                    background: "rgba(232,0,27,0.12)",
                    border: "1px solid rgba(232,0,27,0.25)",
                  }}
                  className="w-12 h-12 rounded-xl flex items-center justify-center overflow-hidden"
                >
                  {store.logo ? (
                    <img
                      src={store.logo}
                      alt={store.name}
                      className="w-full h-full object-contain"
                    />
                  ) : (
                    <span className="text-sm font-bold text-prime">
                      {store.name.slice(0, 2).toUpperCase()}
                    </span>
                  )}
                </div>

                <div>
                  <div className="text-sm font-bold text-text group-hover:text-prime transition-colors">
                    {store.name}
                  </div>

                  <Badge variant={store.reputation === "Excelente" ? "best" : "available"}>
                    {store.reputation}
                  </Badge>
                </div>
              </div>

              <Rating value={store.rating} count={store.reviewCount} />

              <div className="mt-4 space-y-1.5">
                <div className="flex justify-between text-xs">
                  <span className="text-muted">Productos</span>
                  <span className="text-muted-2 font-semibold">
                    {store.productCount.toLocaleString("es-CL")}
                  </span>
                </div>

                <div className="flex justify-between text-xs">
                  <span className="text-muted">Despacho</span>
                  <span className="text-muted-2 font-semibold">{store.dispatchTime}</span>
                </div>

                <div className="flex justify-between text-xs">
                  <span className="text-muted">Sitio</span>
                  <span className="text-prime font-semibold">{store.website}</span>
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

function StoreProductCard({ product }: { product: StoreProduct }) {
  return (
    <div
      style={{ background: "#111111", border: "1px solid #2A2A2A" }}
      className="rounded-2xl overflow-hidden hover:border-prime transition-colors"
    >
      <div className="h-56 bg-surface flex items-center justify-center p-5">
        {product.image ? (
          <img src={product.image} alt={product.name} className="w-full h-full object-contain" />
        ) : (
          <div className="text-sm text-muted">Imagen no disponible</div>
        )}
      </div>

      <div className="p-5">
        <div className="flex items-start justify-between gap-3 mb-2">
          <div>
            <p className="text-xs text-muted mb-1">{product.brand || "Sin marca"}</p>

            <h3 className="text-sm font-semibold text-text leading-5">{product.name}</h3>
          </div>

          <Badge variant={product.available ? "available" : "unavailable"}>
            {product.available ? "Disponible" : "No disponible"}
          </Badge>
        </div>

        <div className="mt-4">
          <div className="text-xl font-bold text-prime">
            {Number(product.price).toLocaleString("es-CL")} {product.currency}
          </div>

          {Number(product.listPrice) > Number(product.price) && (
            <div className="text-xs text-muted line-through mt-1">
              {Number(product.listPrice).toLocaleString("es-CL")} {product.currency}
            </div>
          )}
        </div>

        <div className="mt-4 pt-4 border-t border-border space-y-2">
          <div className="flex justify-between text-xs gap-4">
            <span className="text-muted">Despacho</span>
            <span className="text-muted-2 font-medium text-right">
              {product.shippingFree
                ? "Gratis"
                : `${Number(product.shippingCost).toLocaleString("es-CL")} ${product.currency}`}
            </span>
          </div>

          <div className="flex justify-between text-xs gap-4">
            <span className="text-muted">Stock</span>
            <span className="text-muted-2 font-medium text-right">
              {product.stock === null ? "No informado" : product.stock}
            </span>
          </div>

          <div className="flex justify-between text-xs gap-4">
            <span className="text-muted">Modelo</span>
            <span className="text-muted-2 font-medium text-right">{product.model || "—"}</span>
          </div>
        </div>
      </div>
    </div>
  );
}

function StoreDetail({ storeId, navigate }: Required<Props>) {
  const [store, setStore] = useState<StoreDetail | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [productSearch, setProductSearch] = useState("");

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
  }, [storeId]);

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
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando tienda...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn">{error}</p>
      </div>
    );
  }

  if (!store) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Tienda no encontrada</p>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Tiendas", onClick: () => navigate({ id: "stores" }) },
          { label: store.name },
        ]}
      />

      <div
        style={{
          background:
            "linear-gradient(#3b3f45, #3b3f45) padding-box, linear-gradient(135deg, #a3a3a3, #f472b6, #60a5fa, #a3a3a3) border-box",
          border: "1px solid transparent",
        }}
        className="mb-8 flex flex-col overflow-hidden rounded-2xl md:flex-row"
      >
        <div className="flex h-48 w-full shrink-0 items-center justify-center overflow-hidden bg-slate-300 p-4 md:h-auto md:min-h-[220px] md:w-64">
          {store.logo ? (
            <img
              src={store.logo}
              alt={`Imagen de ${store.name}`}
              className="h-full w-full object-contain"
            />
          ) : (
            <span className="text-5xl font-bold text-slate-700">
              {store.name.slice(0, 2).toUpperCase()}
            </span>
          )}
        </div>

        <div className="flex min-w-0 flex-1 items-start gap-4 p-6 flex-wrap">
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-1 flex-wrap">
              <h1 className="text-xl font-bold text-slate-50">{store.name}</h1>

              <Badge variant={store.reputation === "Excelente" ? "best" : "available"}>
                {store.reputation}
              </Badge>
            </div>

            <div className="flex items-center gap-3 flex-wrap">
              <Rating value={store.rating} count={store.reviewCount} />

              {store.website && (
                <a
                  href={store.website}
                  target="_blank"
                  rel="noreferrer"
                  className="text-xs font-semibold text-prime hover:underline"
                >
                  Visitar sitio
                </a>
              )}
            </div>

            <p className="text-sm text-slate-200 mt-2">{store.conditions}</p>
          </div>

          <div className="grid grid-cols-2 gap-3 text-center">
            <div style={{ background: "#50545A" }} className="rounded-xl px-4 py-3">
              <div className="price text-lg font-bold text-white">
                {store.productCount.toLocaleString("es-CL")}
              </div>
              <div className="text-xs text-slate-200">Productos</div>
            </div>

            <div style={{ background: "#50545A" }} className="rounded-xl px-4 py-3">
              <div className="text-lg font-bold text-slate-50">{store.dispatchTime}</div>
              <div className="text-xs text-slate-200">Despacho</div>
            </div>
          </div>
        </div>
      </div>

      <div className="flex items-center justify-between gap-4 mb-4 flex-wrap">
        <h2 className="text-lg font-bold text-text">Productos disponibles en {store.name}</h2>

        <div className="relative w-full sm:w-80">
          <input
            type="search"
            value={productSearch}
            onChange={(event) => setProductSearch(event.target.value)}
            placeholder="Buscar producto por nombre..."
            className="w-full rounded-xl border border-border bg-surface px-4 py-2.5 text-sm text-text outline-none focus:border-prime"
          />
        </div>
      </div>

      <div className="flex items-center justify-between mb-5">
        <p className="text-xs text-muted">
          {filteredProducts.length} {filteredProducts.length === 1 ? "producto" : "productos"}
        </p>
      </div>

      {filteredProducts.length === 0 ? (
        <div
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="rounded-2xl p-10 text-center mb-10"
        >
          <p className="text-text font-medium">No se encontraron productos</p>
          <p className="text-sm text-muted mt-1">Prueba con otro nombre de producto.</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-3 gap-5 mb-10">
          {filteredProducts.map((product) => (
            <StoreProductCard key={product.id} product={product} />
          ))}
        </div>
      )}

      <h2 className="text-lg font-bold text-text mb-4">Ubicaciones de {store.name}</h2>

      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {store.locations.map((location) => (
          <div
            key={location.id}
            style={{ background: "#111111", border: "1px solid #2A2A2A" }}
            className="rounded-2xl p-5"
          >
            <div className="text-sm font-semibold text-text">
              {location.address || "Direccion no disponible"}
            </div>

            <div className="text-sm text-muted mt-2">
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
    </div>
  );
}

export default function StoresPage(props: Props) {
  if (props.storeId) {
    return <StoreDetail {...props} storeId={props.storeId} />;
  }

  return <StoreList navigate={props.navigate} />;
}
