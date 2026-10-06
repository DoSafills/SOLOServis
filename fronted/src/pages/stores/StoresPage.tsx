import type { Page, Store } from "../../types";
import { getProducts } from "../../services/api/products";
import { getStoreById, getStores } from "../../services/api/stores";
import { useFetch } from "../../hooks/useFetch";
import { Badge, Breadcrumb, PageMessage, Rating } from "../../components/common/ui";
import ProductCard from "../../components/products/ProductCard";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
  storeId?: string;
}

/** Muestra el logo si es una URL; si no hay, las iniciales de la tienda. */
function StoreLogo({ store }: { store: Store }) {
  if (store.logo.startsWith("http")) {
    return (
      <img src={store.logo} alt={store.name} className="w-full h-full object-contain rounded-xl" />
    );
  }

  return <>{store.logo || store.name.slice(0, 2).toUpperCase()}</>;
}

function StoreList({ navigate }: { navigate: (page: Page) => void }) {
  const { data: stores = [], loading, error } = useFetch("stores", getStores);

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
          <p className="text-muted">Cargando tiendas…</p>
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
              {/* Logo */}
              <div className="flex items-center gap-3 mb-4">
                <div
                  style={{
                    background: "rgba(232,0,27,0.12)",
                    border: "1px solid rgba(232,0,27,0.25)",
                  }}
                  className="w-12 h-12 rounded-xl flex items-center justify-center text-sm font-bold text-prime"
                >
                  <StoreLogo store={store} />
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

function StoreDetail({
  storeId,
  navigate,
  favorites,
  compareList,
  onToggleFavorite,
  onToggleCompare,
}: Required<Props>) {
  // Los productos de la tienda son los que tienen alguna oferta en ella.
  const { data, loading, error } = useFetch(`store:${storeId}`, async () => {
    const [store, products] = await Promise.all([getStoreById(storeId), getProducts()]);

    return {
      store,
      storeProducts: products.filter((p) => p.offers.some((o) => o.storeId === storeId)),
    };
  });

  if (loading) return <PageMessage>Cargando tienda…</PageMessage>;

  if (error) return <PageMessage tone="warn">{error}</PageMessage>;

  if (!data?.store) return <PageMessage>Tienda no encontrada.</PageMessage>;

  const { store, storeProducts } = data;

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Tiendas", onClick: () => navigate({ id: "stores" }) },
          { label: store.name },
        ]}
      />

      {/* Store header */}
      <div
        style={{ background: "#111111", border: "1px solid #2A2A2A" }}
        className="rounded-2xl p-6 mb-8"
      >
        <div className="flex items-start gap-4 flex-wrap">
          <div
            style={{ background: "rgba(232,0,27,0.12)", border: "1px solid rgba(232,0,27,0.3)" }}
            className="w-16 h-16 rounded-2xl flex items-center justify-center text-xl font-bold text-prime shrink-0"
          >
            <StoreLogo store={store} />
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-1 flex-wrap">
              <h1 className="text-xl font-bold text-text">{store.name}</h1>
              <Badge variant={store.reputation === "Excelente" ? "best" : "available"}>
                {store.reputation}
              </Badge>
            </div>
            <Rating value={store.rating} count={store.reviewCount} />
            <p className="text-sm text-muted mt-2">{store.conditions}</p>
          </div>
          <div className="grid grid-cols-2 gap-3 text-center">
            <div style={{ background: "#1A1A1A" }} className="rounded-xl px-4 py-3">
              <div className="price text-lg font-bold text-prime">
                {store.productCount.toLocaleString("es-CL")}
              </div>
              <div className="text-xs text-muted">Productos</div>
            </div>
            <div style={{ background: "#1A1A1A" }} className="rounded-xl px-4 py-3">
              <div className="text-lg font-bold text-text">{store.dispatchTime}</div>
              <div className="text-xs text-muted">Despacho</div>
            </div>
          </div>
        </div>
      </div>

      {/* Products in store */}
      <h2 className="text-lg font-bold text-text mb-4">Productos disponibles en {store.name}</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {storeProducts.map((p) => (
          <ProductCard
            key={p.id}
            product={p}
            navigate={navigate}
            isFavorite={favorites.has(p.id)}
            isComparing={compareList.has(p.id)}
            onToggleFavorite={onToggleFavorite}
            onToggleCompare={onToggleCompare}
          />
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
