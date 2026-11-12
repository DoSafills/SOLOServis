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

  const breadcrumb = (
    <Breadcrumb
      items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Favoritos" }]}
    />
  );

  if (!authenticated) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        {breadcrumb}
        <section className="mx-auto flex max-w-lg flex-col items-center rounded-2xl border border-slate-700/80 bg-slate-900/60 px-6 py-16 text-center">
          <span className="mb-5 flex h-12 w-12 items-center justify-center rounded-full border border-slate-700 bg-slate-950/60 text-slate-400">
            <HeartIcon />
          </span>
          <h1 className="text-xl font-semibold text-white">Inicia sesión para ver tus favoritos</h1>
          <p className="mt-2 max-w-sm text-sm leading-relaxed text-slate-400">
            Guarda productos y servicios para tenerlos a mano y seguir sus precios.
          </p>
          <button
            type="button"
            onClick={() => navigate({ id: "user" })}
            className="mt-6 rounded-lg bg-cyan-300 px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-200"
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
      <div className="mx-auto max-w-7xl px-4 py-8">
        {breadcrumb}
        <div className="mb-6">
          <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-cyan-300/80">
            Tu colección
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-white">Favoritos</h1>
        </div>
        <div className="grid gap-4 sm:grid-cols-2 2xl:grid-cols-3">
          {[0, 1, 2].map((item) => (
            <div
              key={item}
              className="h-36 animate-pulse rounded-xl border border-slate-800 bg-slate-900/50"
            />
          ))}
        </div>
        <p role="status" className="mt-4 text-center text-sm text-slate-400">
          Cargando favoritos...
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      {breadcrumb}

      <header className="mb-8 flex flex-wrap items-end justify-between gap-4 border-b border-slate-800 pb-5">
        <div>
          <p className="text-[10px] font-semibold uppercase tracking-[0.2em] text-cyan-300/80">
            Tu colección
          </p>
          <h1 className="mt-1 text-2xl font-semibold tracking-tight text-white">Favoritos</h1>
          <p className="mt-1 text-sm text-slate-400">
            {visibleCount} {visibleCount === 1 ? "elemento guardado" : "elementos guardados"}
          </p>
        </div>
        <button
          type="button"
          onClick={() => navigate({ id: "home" })}
          className="rounded-lg border border-slate-700 px-3.5 py-2 text-sm font-medium text-slate-300 transition hover:border-slate-500 hover:text-white"
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
        <section className="flex flex-col items-center rounded-2xl border border-slate-700/80 bg-slate-900/40 px-6 py-16 text-center">
          <span className="mb-5 flex h-12 w-12 items-center justify-center rounded-full border border-slate-700 bg-slate-950/60 text-slate-400">
            <HeartIcon />
          </span>
          <h2 className="text-lg font-semibold text-white">Aún no tienes favoritos</h2>
          <p className="mt-2 max-w-sm text-sm leading-relaxed text-slate-400">
            Guarda productos o servicios que quieras revisar más tarde.
          </p>
          <button
            type="button"
            onClick={() => navigate({ id: "home" })}
            className="mt-6 rounded-lg bg-cyan-300 px-4 py-2.5 text-sm font-semibold text-slate-950 transition hover:bg-cyan-200"
          >
            Explorar productos
          </button>
        </section>
      ) : visibleCount === 0 ? (
        <section className="rounded-xl border border-slate-700/80 bg-slate-900/40 px-5 py-8 text-center">
          <h2 className="text-base font-semibold text-white">
            No se pudieron encontrar los elementos guardados
          </h2>
          <p className="mt-2 text-sm text-slate-400">
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
                      className="group flex min-w-0 gap-4 overflow-hidden rounded-2xl border border-slate-700/75 bg-gradient-to-br from-slate-900/95 via-slate-900/75 to-indigo-950/30 p-4 shadow-lg shadow-slate-950/15 transition duration-200 hover:-translate-y-0.5 hover:border-cyan-300/30 hover:shadow-xl hover:shadow-cyan-950/10"
                    >
                      <button
                        type="button"
                        onClick={() => navigate({ id: "product-detail", productId: product.id })}
                        aria-label={`Ver ${product.name}`}
                        className="flex h-24 w-24 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-white/[0.06] bg-slate-950/65 p-2.5 shadow-inner focus-visible:outline-2 focus-visible:outline-cyan-300"
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
                            <p className="truncate text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-400">
                              {product.brand}
                            </p>
                            <button
                              type="button"
                              onClick={() =>
                                navigate({ id: "product-detail", productId: product.id })
                              }
                              className="mt-1 line-clamp-2 text-left text-sm font-semibold leading-snug text-slate-100 transition-colors hover:text-cyan-200"
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

                        <div className="mt-auto flex flex-wrap items-end justify-between gap-2 border-t border-slate-700/70 pt-3">
                          <div className="min-w-0">
                            {minOffer ? (
                              <>
                                <span className="block text-[10px] font-medium text-slate-500">
                                  Mejor precio
                                </span>
                                <span className="price text-lg font-semibold leading-tight text-white">
                                  {formatPrice(minOffer.price)}
                                </span>
                                <span className="mt-0.5 block max-w-36 truncate text-[10px] text-slate-400">
                                  {minOffer.storeName}
                                </span>
                              </>
                            ) : (
                              <span className="text-xs text-slate-500">Sin oferta disponible</span>
                            )}
                          </div>
                          <span
                            className={`mb-0.5 rounded-full border px-2.5 py-1 text-[10px] font-medium ${
                              available
                                ? "border-emerald-400/20 bg-emerald-400/[0.07] text-emerald-300"
                                : "border-slate-700 bg-slate-950/40 text-slate-400"
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
                    className="group flex min-w-0 gap-4 overflow-hidden rounded-2xl border border-slate-700/75 bg-gradient-to-br from-slate-900/95 via-slate-900/75 to-violet-950/25 p-4 shadow-lg shadow-slate-950/15 transition duration-200 hover:-translate-y-0.5 hover:border-violet-300/30 hover:shadow-xl hover:shadow-violet-950/10"
                  >
                    <button
                      type="button"
                      onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
                      aria-label={`Ver ${service.name}`}
                      className="flex h-24 w-24 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-white/[0.06] bg-slate-950/65 p-2.5 shadow-inner focus-visible:outline-2 focus-visible:outline-cyan-300"
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
                          <p className="truncate text-[10px] font-semibold uppercase tracking-[0.14em] text-slate-400">
                            {service.provider}
                          </p>
                          <button
                            type="button"
                            onClick={() =>
                              navigate({ id: "service-detail", serviceId: service.id })
                            }
                            className="mt-1 line-clamp-2 text-left text-sm font-semibold leading-snug text-slate-100 transition-colors hover:text-cyan-200"
                          >
                            {service.name}
                          </button>
                        </div>
                        <FavoriteButton
                          active
                          onClick={() => onToggleFavorite(service.id, "service")}
                        />
                      </div>

                      <div className="mt-auto flex flex-wrap items-end justify-between gap-2 border-t border-slate-700/70 pt-3">
                        <div>
                          <span className="block text-[10px] font-medium text-slate-500">
                            {service.category}
                          </span>
                          <span className="price text-lg font-semibold leading-tight text-white">
                            {formatPrice(service.monthlyPrice)}
                          </span>
                          <span className="ml-1 text-[10px] text-slate-400">
                            {getBillingPeriodText(service.billingPeriod)}
                          </span>
                        </div>
                        <button
                          type="button"
                          onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
                          className="mb-1 inline-flex items-center gap-1 text-xs font-medium text-cyan-300/80 transition-colors hover:text-cyan-200"
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
    <div className="mb-3 flex items-center justify-between border-b border-slate-800 pb-2">
      <h2 className="text-sm font-semibold text-slate-100">{title}</h2>
      <span className="text-xs text-slate-500">{count}</span>
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
