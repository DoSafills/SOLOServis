import { useEffect, useState } from "react";
import type { Page } from "../../types";
import { getStores, type ApiStore } from "../../Services/api/stores";
import { getOffers, type ApiOffer } from "../../Services/api/offers";
import { getProducts, type ApiProduct } from "../../Services/api/products";
import { mapProduct } from "../../Services/api/catalog";
import { Badge, Breadcrumb, Rating } from "../../components/ui";
import ProductCard from "../../components/ProductCard";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
  storeId?: string;
}

function storeInitials(name: string): string {
  const words = name.trim().split(/\s+/);
  if (words.length > 1) {
    return (words[0][0] + words[1][0]).toUpperCase();
  }
  return name.slice(0, 2).toUpperCase();
}

function bareUrl(url: string): string {
  return url.replace(/^https?:\/\//, "");
}


function StoreList({
  stores,
  offers,
  navigate,
}: {
  stores: ApiStore[];
  offers: ApiOffer[];
  navigate: (page: Page) => void;
}) {
  const productCountByStore = new Map<number, number>();
  for (const offer of offers) {
    const seen = new Set<string>();
    for (const o of offers) {
      if (o.storeId === offer.storeId) seen.add(o.productPublicId);
    }
    productCountByStore.set(offer.storeId, seen.size);
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Tiendas" }]}
      />

      <h1 className="text-2xl font-bold text-text mb-2">Tiendas y proveedores</h1>
      <p className="text-sm text-muted mb-8">
        Directorio de tiendas comparadas en nuestra plataforma
      </p>

      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {stores.map((store) => (
          <button
            key={store.id}
            onClick={() => navigate({ id: "store-detail", storeId: String(store.id) })}
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
                {storeInitials(store.name)}
              </div>
              <div>
                <div className="text-sm font-bold text-text group-hover:text-prime transition-colors">
                  {store.name}
                </div>
                <Badge
                  variant={
                    store.reputation === "Excelente" || store.reputation === "Muy buena"
                      ? "best"
                      : "available"
                  }
                >
                  {store.reputation}
                </Badge>
              </div>
            </div>

            <Rating value={store.rating} />

            <div className="mt-4 space-y-1.5">
              <div className="flex justify-between text-xs">
                <span className="text-muted">Productos</span>
                <span className="text-muted-2 font-semibold">
                  {(productCountByStore.get(store.id) ?? 0).toLocaleString("es-CL")}
                </span>
              </div>
              <div className="flex justify-between text-xs">
                <span className="text-muted">Despacho</span>
                <span className="text-muted-2 font-semibold">{store.shippingInformation}</span>
              </div>
              <div className="flex justify-between text-xs">
                <span className="text-muted">Sitio</span>
                <span className="text-prime font-semibold">{bareUrl(store.websiteUrl)}</span>
              </div>
            </div>
          </button>
        ))}
      </div>
    </div>
  );
}

function StoreDetail({
  storeId,
  stores,
  offers,
  products,
  navigate,
  favorites,
  compareList,
  onToggleFavorite,
  onToggleCompare,
}: Required<Props> & { stores: ApiStore[]; offers: ApiOffer[]; products: ApiProduct[] }) {
  const store = stores.find((s) => String(s.id) === storeId);
  if (!store) return null;

  const storeOffers = offers.filter((o) => String(o.storeId) === storeId);
  const storeOffersByProduct = new Map<string, ApiOffer[]>();
  for (const offer of storeOffers) {
    const list = storeOffersByProduct.get(offer.productPublicId) ?? [];
    list.push(offer);
    storeOffersByProduct.set(offer.productPublicId, list);
  }
  const storeProducts = products
    .filter((p) => storeOffersByProduct.has(p.id))
    .map((p) => mapProduct(p, storeOffersByProduct.get(p.id) ?? []));

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
            style={{
              background: "rgba(232,0,27,0.12)",
              border: "1px solid rgba(232,0,27,0.3)",
            }}
            className="w-16 h-16 rounded-2xl flex items-center justify-center text-xl font-bold text-prime shrink-0"
          >
            {storeInitials(store.name)}
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-1 flex-wrap">
              <h1 className="text-xl font-bold text-text">{store.name}</h1>
              <Badge
                variant={
                  store.reputation === "Excelente" || store.reputation === "Muy buena"
                    ? "best"
                    : "available"
                }
              >
                {store.reputation}
              </Badge>
            </div>
            <Rating value={store.rating} />
            <p className="text-sm text-muted mt-2">{store.generalConditions}</p>
          </div>
          <div className="grid grid-cols-2 gap-3 text-center">
            <div style={{ background: "#1A1A1A" }} className="rounded-xl px-4 py-3">
              <div className="price text-lg font-bold text-prime">
                {storeProducts.length.toLocaleString("es-CL")}
              </div>
              <div className="text-xs text-muted">Productos</div>
            </div>
            <div style={{ background: "#1A1A1A" }} className="rounded-xl px-4 py-3">
              <div className="text-lg font-bold text-text">{store.shippingInformation}</div>
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
  const [stores, setStores] = useState<ApiStore[]>([]);
  const [offers, setOffers] = useState<ApiOffer[]>([]);
  const [products, setProducts] = useState<ApiProduct[]>([]);

  useEffect(() => {
    getStores().then(setStores).catch(console.error);
    getOffers().then(setOffers).catch(console.error);
    getProducts().then(setProducts).catch(console.error);
  }, []);

  if (props.storeId) {
    return (
      <StoreDetail
        {...props}
        storeId={props.storeId}
        stores={stores}
        offers={offers}
        products={products}
        favorites={props.favorites}
        compareList={props.compareList}
        onToggleFavorite={props.onToggleFavorite}
        onToggleCompare={props.onToggleCompare}
      />
    );
  }
  return <StoreList stores={stores} offers={offers} navigate={props.navigate} />;
}
