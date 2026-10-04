import { useEffect, useState } from "react";
import type { Page, Product, Service } from "../../types";
import { getProducts, getServices } from "../../services/api/api";
import { formatPrice, getMinPrice } from "../../services/utils/productUtils";
import { Badge, Breadcrumb, FavoriteButton, Rating } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  authenticated: boolean;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
}

export default function FavoritesPage({
  navigate,
  favorites,
  authenticated,
  onToggleFavorite,
}: Props) {
  const [products, setProducts] = useState<Product[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [loadError, setLoadError] = useState<string | null>(null);

  useEffect(() => {
    if (!authenticated) return;
    let cancelled = false;

    async function loadFavorites() {
      const [productsResult, servicesResult] = await Promise.allSettled([
        getProducts(),
        getServices(),
      ]);
      if (cancelled) return;

      const errors: string[] = [];
      if (productsResult.status === "fulfilled") setProducts(productsResult.value);
      else errors.push("No se pudieron cargar los productos favoritos.");

      if (servicesResult.status === "fulfilled") setServices(servicesResult.value);
      else errors.push("No se pudieron cargar los servicios favoritos.");

      setLoadError(errors.length > 0 ? errors.join(" ") : null);
      setLoading(false);
    }

    void loadFavorites();
    return () => {
      cancelled = true;
    };
  }, [authenticated]);

  if (!authenticated) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        <Breadcrumb
          items={[
            { label: "Inicio", onClick: () => navigate({ id: "home" }) },
            { label: "Favoritos" },
          ]}
        />
        <div className="mx-auto flex max-w-lg flex-col items-center gap-4 py-24 text-center">
          <h1 className="text-2xl font-black text-text">Inicia sesión para ver tus favoritos</h1>
          <p className="max-w-sm text-sm text-muted">
            La navegación y comparación son libres. Guarda y consulta tus productos favoritos al
            iniciar sesión o crear una cuenta.
          </p>
          <button
            onClick={() => navigate({ id: "user" })}
            className="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-5 py-3 text-sm font-bold text-white"
          >
            Ir a Mi cuenta
          </button>
        </div>
      </div>
    );
  }

  const favProducts = products.filter((product) => favorites.has(product.id));
  const favServices = services.filter((service) => favorites.has(service.id));

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando favoritos...</p>
      </div>
    );
  }

  if (favorites.size === 0) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-8">
        <Breadcrumb
          items={[
            { label: "Inicio", onClick: () => navigate({ id: "home" }) },
            { label: "Favoritos" },
          ]}
        />

        {loadError && (
          <p
            role="alert"
            className="mb-4 rounded-xl border border-rose-300/30 bg-rose-950/40 p-3 text-sm text-rose-200"
          >
            {loadError}
          </p>
        )}

        <div className="flex flex-col items-center justify-center py-24 gap-4 text-center">
          <div className="w-16 h-16 rounded-2xl bg-surface-2 flex items-center justify-center mb-2">
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="#64748B"
              strokeWidth="1.5"
            >
              <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
            </svg>
          </div>

          <h3 className="text-lg font-semibold text-text">No tienes favoritos aún</h3>

          <p className="text-sm text-muted max-w-xs">
            Guarda productos y servicios para seguir sus precios y recibir alertas de bajadas.
          </p>

          <button
            onClick={() => navigate({ id: "home" })}
            style={{ background: "#E8001B", color: "#0A0A0A" }}
            className="mt-2 px-5 py-2 rounded-xl text-sm font-semibold"
          >
            Explorar productos
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Favoritos" },
        ]}
      />

      <h1 className="text-2xl font-bold text-text mb-1">Mis favoritos</h1>
      <p className="text-sm text-muted mb-8">{favorites.size} items guardados</p>
      {loadError && (
        <p
          role="alert"
          className="mb-6 rounded-xl border border-rose-300/30 bg-rose-950/40 p-3 text-sm text-rose-200"
        >
          {loadError}
        </p>
      )}

      {favProducts.length > 0 && (
        <section className="mb-10">
          <h2 className="text-base font-semibold text-muted-2 uppercase tracking-widest text-xs mb-4">
            Productos
          </h2>

          <div className="space-y-3">
            {favProducts.map((product) => {
              const minPrice = getMinPrice(product);
              const prevPrice = Math.round((minPrice * 1.08) / 1000) * 1000;
              const diff = prevPrice - minPrice;

              return (
                <div
                  key={product.id}
                  style={{
                    background: "#111111",
                    border: "1px solid #2A2A2A",
                  }}
                  className="rounded-2xl p-3 flex items-center gap-3 hover:border-prime transition-all group"
                >
                  <div
                    className="w-16 h-14 rounded-xl overflow-hidden shrink-0 cursor-pointer bg-white"
                    onClick={() =>
                      navigate({
                        id: "product-detail",
                        productId: product.id,
                      })
                    }
                  >
                    <img
                      src={product.image}
                      alt={product.name}
                      className="w-full h-full object-contain p-1"
                    />
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="text-xs text-prime font-semibold">{product.brand}</div>

                    <div
                      className="text-sm font-semibold text-text truncate cursor-pointer hover:text-prime transition-colors"
                      onClick={() =>
                        navigate({
                          id: "product-detail",
                          productId: product.id,
                        })
                      }
                    >
                      {product.name}
                    </div>

                    <Rating value={product.rating} />
                  </div>

                  <div className="text-right shrink-0">
                    <div className="price text-lg font-bold text-prime">
                      {formatPrice(minPrice)}
                    </div>

                    {diff > 0 && (
                      <div className="flex items-center gap-1 text-xs text-success font-semibold justify-end">
                        <svg
                          width="10"
                          height="10"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="3"
                        >
                          <path d="m18 15-6-6-6 6" />
                        </svg>
                        Bajó {formatPrice(diff)}
                      </div>
                    )}

                    <div className="text-xs text-muted line-through">{formatPrice(prevPrice)}</div>
                  </div>

                  <div className="flex flex-col items-center gap-2 shrink-0">
                    <FavoriteButton active={true} onClick={() => onToggleFavorite(product.id)} />

                    <Badge
                      variant={
                        product.offers.some((offer) => offer.available)
                          ? "available"
                          : "unavailable"
                      }
                    >
                      {product.offers.some((offer) => offer.available) ? "Disponible" : "Agotado"}
                    </Badge>
                  </div>
                </div>
              );
            })}
          </div>
        </section>
      )}

      {favServices.length > 0 && (
        <section>
          <h2 className="text-base font-semibold text-muted-2 uppercase tracking-widest text-xs mb-4">
            Servicios
          </h2>

          <div className="space-y-3">
            {favServices.map((service) => {
              const prevPrice = Math.round((service.monthlyPrice * 1.06) / 100) * 100;
              const diff = prevPrice - service.monthlyPrice;

              return (
                <div
                  key={service.id}
                  style={{
                    background: "#111111",
                    border: "1px solid #2A2A2A",
                  }}
                  className="rounded-2xl p-4 flex items-center gap-4 hover:border-prime transition-all group"
                >
                  <div
                    className="w-20 h-16 rounded-xl overflow-hidden shrink-0 cursor-pointer"
                    onClick={() =>
                      navigate({
                        id: "service-detail",
                        serviceId: service.id,
                      })
                    }
                  >
                    <img
                      src={service.image}
                      alt={service.name}
                      className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
                    />
                  </div>

                  <div className="flex-1 min-w-0">
                    <div className="text-xs text-prime font-semibold">{service.provider}</div>

                    <div
                      className="text-sm font-semibold text-text truncate cursor-pointer hover:text-prime transition-colors"
                      onClick={() =>
                        navigate({
                          id: "service-detail",
                          serviceId: service.id,
                        })
                      }
                    >
                      {service.name}
                    </div>

                    <Badge variant="available">{service.category}</Badge>
                  </div>

                  <div className="text-right shrink-0">
                    <div className="price text-lg font-bold text-prime">
                      {formatPrice(service.monthlyPrice)}
                      <span className="text-xs text-muted font-normal">/mes</span>
                    </div>

                    {diff > 0 && (
                      <div className="flex items-center gap-1 text-xs text-success font-semibold justify-end">
                        ↑ Bajó {formatPrice(diff)}
                      </div>
                    )}
                  </div>

                  <FavoriteButton
                    active={true}
                    onClick={() => onToggleFavorite(service.id, "service")}
                  />
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
