import { useEffect, useState } from "react";
import type { Page, Product, Service } from "../../types";
import { getProducts, getServices } from "../../services/api/api";
import { formatPrice, getBillingPeriodText, getMinOffer } from "../../services/utils/productUtils";
import { Breadcrumb, FavoriteButton, Rating } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  authenticated: boolean;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  embedded?: boolean;
}

export default function FavoritesPage({
  navigate,
  favorites,
  authenticated,
  onToggleFavorite,
  embedded = false,
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

  const breadcrumb = !embedded && (
    <Breadcrumb
      items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Favoritos" }]}
    />
  );

  if (!authenticated) {
    return (
      <div className={embedded ? "w-full" : "mx-auto max-w-7xl px-4 py-8"}>
        {breadcrumb}
        <section className="mx-auto flex max-w-lg flex-col items-center rounded-2xl border border-bd bg-surface px-6 py-16 text-center">
          <span className="mb-5 flex h-12 w-12 items-center justify-center rounded-full border border-bd bg-surface-2 text-prime">
            <HeartIcon />
          </span>
          <h1 className="text-xl font-semibold text-text">Inicia sesión para ver tus favoritos</h1>
          <p className="mt-2 max-w-sm text-sm leading-relaxed text-muted">
            Guarda productos y servicios para tenerlos a mano y seguir sus precios.
          </p>
          <button
            type="button"
            onClick={() => navigate({ id: "user" })}
            className="mt-6 rounded-lg bg-prime px-4 py-2.5 text-sm font-semibold text-[#181714] transition hover:bg-prime-dark"
          >
            Ir a Mi cuenta
          </button>
        </section>
      </div>
    );
  }

  const favProducts = products.filter((product) => favorites.has(product.id));
  const favServices = services.filter((service) => favorites.has(service.id));
  const visibleCount = favProducts.length + favServices.length;

  if (loading) {
    return (
      <div className={embedded ? "w-full" : "mx-auto max-w-7xl px-4 py-8"}>
        {breadcrumb}
        <div className="mb-6">
          <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-prime">
            Tu colección
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-text">Favoritos</h1>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-3">
          {[0, 1, 2].map((item) => (
            <div key={item} className="h-36 animate-pulse rounded-xl border border-bd bg-surface" />
          ))}
        </div>
        <p role="status" className="mt-4 text-center text-sm text-muted">
          Cargando favoritos...
        </p>
      </div>
    );
  }

  return (
    <div className={embedded ? "w-full" : "mx-auto max-w-7xl px-4 py-8"}>
      {breadcrumb}

      <header className="mb-8 flex flex-wrap items-end justify-between gap-4 border-b border-bd pb-5">
        <div>
          <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-prime">
            Tu colección
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-text">Favoritos</h1>
          <p className="mt-1 text-sm text-muted">
            {visibleCount} {visibleCount === 1 ? "elemento guardado" : "elementos guardados"}
          </p>
        </div>
        <button
          type="button"
          onClick={() => navigate({ id: "home" })}
          className="rounded-lg border border-bd px-3.5 py-2 text-sm font-medium text-muted transition hover:border-prime hover:text-text"
        >
          Explorar catálogo
        </button>
      </header>

      {loadError && (
        <p
          role="alert"
          className="mb-6 rounded-xl border border-rose-300/20 bg-rose-950/30 px-4 py-3 text-sm text-rose-200"
        >
          {loadError}
        </p>
      )}

      {favorites.size === 0 ? (
        <section className="flex flex-col items-center rounded-2xl border border-bd bg-surface px-6 py-16 text-center">
          <span className="mb-5 flex h-12 w-12 items-center justify-center rounded-full border border-bd bg-surface-2 text-prime">
            <HeartIcon />
          </span>
          <h2 className="text-lg font-semibold text-text">Aún no tienes favoritos</h2>
          <p className="mt-2 max-w-sm text-sm leading-relaxed text-muted">
            Guarda productos o servicios que quieras revisar más tarde.
          </p>
          <button
            type="button"
            onClick={() => navigate({ id: "home" })}
            className="mt-6 rounded-lg bg-prime px-4 py-2.5 text-sm font-semibold text-[#181714] transition hover:bg-prime-dark"
          >
            Explorar productos
          </button>
        </section>
      ) : visibleCount === 0 ? (
        <section className="rounded-xl border border-bd bg-surface px-5 py-8 text-center">
          <h2 className="text-base font-semibold text-text">
            No se pudieron encontrar los elementos guardados
          </h2>
          <p className="mt-2 text-sm text-muted">
            {loadError
              ? "Revisa la conexión con las APIs e inténtalo nuevamente."
              : "Es posible que ya no estén disponibles en el catálogo."}
          </p>
        </section>
      ) : (
        <div className="space-y-10">
          {favProducts.length > 0 && (
            <section>
              <SectionHeading title="Productos" count={favProducts.length} />
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {favProducts.map((product) => {
                  const minOffer = getMinOffer(product);
                  const available = product.offers.some((offer) => offer.available);

                  return (
                    <article
                      key={product.id}
                      className="group flex min-w-0 gap-4 overflow-hidden rounded-2xl border border-bd bg-gradient-to-br from-surface to-surface-2 p-4 shadow-lg shadow-black/15 transition duration-200 hover:-translate-y-0.5 hover:border-prime/60 hover:shadow-xl hover:shadow-black/20"
                    >
                      <button
                        type="button"
                        onClick={() => navigate({ id: "product-detail", productId: product.id })}
                        aria-label={`Ver ${product.name}`}
                        className="flex h-24 w-24 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-bd bg-surface-2 p-2.5 shadow-inner focus-visible:outline-2 focus-visible:outline-prime"
                      >
                        {product.image ? (
                          <img
                            src={product.image}
                            alt=""
                            loading="lazy"
                            className="h-full w-full object-contain transition-transform group-hover:scale-105"
                          />
                        ) : (
                          <span className="text-[10px] text-slate-500">Sin imagen</span>
                        )}
                      </button>

                      <div className="flex min-h-24 min-w-0 flex-1 flex-col">
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0">
                            <p className="truncate text-[10px] font-semibold uppercase tracking-[0.14em] text-muted">
                              {product.brand}
                            </p>
                            <button
                              type="button"
                              onClick={() =>
                                navigate({ id: "product-detail", productId: product.id })
                              }
                              className="mt-1 line-clamp-2 text-left text-sm font-semibold leading-snug text-text transition-colors hover:text-prime"
                            >
                              {product.name}
                            </button>
                          </div>
                          <FavoriteButton
                            active
                            onClick={() => onToggleFavorite(product.id, "product")}
                          />
                        </div>

                        <div className="mt-2">
                          <Rating value={product.rating} count={product.reviewCount} />
                        </div>

                        <div className="mt-auto flex flex-wrap items-end justify-between gap-2 border-t border-bd pt-3">
                          <div className="min-w-0">
                            {minOffer ? (
                              <>
                                <span className="block text-[10px] font-medium text-muted">
                                  Mejor precio
                                </span>
                                <span className="price text-lg font-semibold leading-tight text-prime">
                                  {formatPrice(minOffer.price)}
                                </span>
                                <span className="mt-0.5 block max-w-36 truncate text-[10px] text-muted">
                                  {minOffer.storeName}
                                </span>
                              </>
                            ) : (
                              <span className="text-xs text-muted">Sin oferta disponible</span>
                            )}
                          </div>
                          <span
                            className={`mb-0.5 rounded-full border px-2.5 py-1 text-[10px] font-medium ${
                              available
                                ? "border-emerald-400/20 bg-emerald-400/[0.07] text-emerald-300"
                                : "border-bd bg-surface-2 text-muted"
                            }`}
                          >
                            {available ? "Disponible" : "Agotado"}
                          </span>
                        </div>
                      </div>
                    </article>
                  );
                })}
              </div>
            </section>
          )}

          {favServices.length > 0 && (
            <section>
              <SectionHeading title="Servicios" count={favServices.length} />
              <div className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-3">
                {favServices.map((service) => (
                  <article
                    key={service.id}
                    className="group flex min-w-0 gap-4 overflow-hidden rounded-2xl border border-bd bg-gradient-to-br from-surface to-surface-2 p-4 shadow-lg shadow-black/15 transition duration-200 hover:-translate-y-0.5 hover:border-prime/60 hover:shadow-xl hover:shadow-black/20"
                  >
                    <button
                      type="button"
                      onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
                      aria-label={`Ver ${service.name}`}
                      className="flex h-24 w-24 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-bd bg-surface-2 p-2.5 shadow-inner focus-visible:outline-2 focus-visible:outline-prime"
                    >
                      {service.image ? (
                        <img
                          src={service.image}
                          alt=""
                          loading="lazy"
                          className="h-full w-full object-contain transition-transform group-hover:scale-105"
                        />
                      ) : (
                        <span className="text-[10px] text-slate-500">Sin imagen</span>
                      )}
                    </button>

                    <div className="flex min-h-24 min-w-0 flex-1 flex-col">
                      <div className="flex items-start justify-between gap-2">
                        <div className="min-w-0">
                          <p className="truncate text-[10px] font-semibold uppercase tracking-[0.14em] text-muted">
                            {service.provider}
                          </p>
                          <button
                            type="button"
                            onClick={() =>
                              navigate({ id: "service-detail", serviceId: service.id })
                            }
                            className="mt-1 line-clamp-2 text-left text-sm font-semibold leading-snug text-text transition-colors hover:text-prime"
                          >
                            {service.name}
                          </button>
                        </div>
                        <FavoriteButton
                          active
                          onClick={() => onToggleFavorite(service.id, "service")}
                        />
                      </div>

                      <div className="mt-auto flex flex-wrap items-end justify-between gap-2 border-t border-bd pt-3">
                        <div>
                          <span className="block text-[10px] font-medium text-muted">
                            {service.category}
                          </span>
                          <span className="price text-lg font-semibold leading-tight text-prime">
                            {formatPrice(service.monthlyPrice)}
                          </span>
                          <span className="ml-1 text-[10px] text-muted">
                            {getBillingPeriodText(service.billingPeriod)}
                          </span>
                        </div>
                        <button
                          type="button"
                          onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
                          className="mb-1 inline-flex items-center gap-1 text-xs font-medium text-prime transition-colors hover:text-text"
                        >
                          Ver servicio
                          <span aria-hidden="true">→</span>
                        </button>
                      </div>
                    </div>
                  </article>
                ))}
              </div>
            </section>
          )}
        </div>
      )}
    </div>
  );
}

function SectionHeading({ title, count }: { title: string; count: number }) {
  return (
    <div className="mb-3 flex items-center justify-between border-b border-bd pb-2">
      <h2 className="text-sm font-semibold text-text">{title}</h2>
      <span className="text-xs text-muted">{count}</span>
    </div>
  );
}

function HeartIcon() {
  return (
    <svg
      width="20"
      height="20"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      aria-hidden="true"
    >
      <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
    </svg>
  );
}
