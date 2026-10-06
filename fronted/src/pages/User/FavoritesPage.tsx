import type { Page } from "../../types";
import { getProducts } from "../../services/api/products";
import { getServices } from "../../services/api/services";
import { formatPrice, getMinPrice } from "../../services/utils/productUtils";
import { useFetch } from "../../hooks/useFetch";
import {
  Badge,
  Breadcrumb,
  EmptyState,
  FavoriteButton,
  PageMessage,
  Rating,
} from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  onToggleFavorite: (id: string) => void;
}

export default function FavoritesPage({ navigate, favorites, onToggleFavorite }: Props) {
  const { data: products = [], loading: loadingProducts } = useFetch("products", () =>
    getProducts(),
  );
  const { data: services = [], loading: loadingServices } = useFetch("services", getServices);

  const favProducts = products.filter((product) => favorites.has(product.id));
  const favServices = services.filter((service) => favorites.has(service.id));

  if (loadingProducts || loadingServices) {
    return <PageMessage>Cargando favoritos...</PageMessage>;
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

        <EmptyState
          title="No tienes favoritos aún"
          description="Guarda productos y servicios para seguir sus precios y recibir alertas de bajadas."
          action={{ label: "Explorar productos", onClick: () => navigate({ id: "home" }) }}
          icon={
            <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
          }
        />
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
                  className="rounded-2xl p-4 flex items-center gap-4 hover:border-prime transition-all group"
                >
                  <div
                    className="w-20 h-16 rounded-xl overflow-hidden shrink-0 cursor-pointer"
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
                      className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
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

                  <FavoriteButton active={true} onClick={() => onToggleFavorite(service.id)} />
                </div>
              );
            })}
          </div>
        </section>
      )}
    </div>
  );
}
