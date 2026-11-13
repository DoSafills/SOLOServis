import { useEffect, useState } from "react";
import type { FormEvent } from "react";
import { UI_TEXT } from "../../constants/uiText";
import type { Page, Product, Service, StoreOffer } from "../../types";
import { PRODUCT_TYPES, type ProductTypeId } from "../../constants/productCatalog";
import type { SearchHistoryEntry } from "../../services/api/personalization";

import { getProducts, getServices } from "../../services/api/api";
import { getPopularProducts, type SearchProductResult } from "../../services/api/search";
import ProductCard from "../../components/products/ProductCard";
import ServiceCard from "../../components/services/ServiceCard";

const productCategories = PRODUCT_TYPES;

const serviceCategories = [
  { name: UI_TEXT.categories.internet, icon: "wifi", color: "#F472B6" },
  { name: UI_TEXT.categories.insurance, icon: "shield", color: "#FBBF24" },
  { name: UI_TEXT.categories.technicians, icon: "wrench", color: "#60A5FA" },
  { name: UI_TEXT.categories.education, icon: "book", color: "#34D399" },
];

const productCategoryIcons: Record<ProductTypeId, string> = {
  appliances: "refrigerator",
  technology: "bolt",
  computing: "laptop",
  phones: "phone",
  gaming: "gamepad",
  home: "house",
};

const featuredCategories = [
  ...productCategories.map((category) => ({
    ...category,
    icon: productCategoryIcons[category.id],
    kind: "product" as const,
  })),
  ...serviceCategories.map((category) => ({ ...category, kind: "service" as const })),
];

const categoryIconPaths: Record<string, string> = {
  refrigerator: "M6 3h12v18H6z M6 10h12 M9 7v1 M9 13v2",
  bolt: "M13 2 5 13h6l-1 9 9-12h-6l1-8Z",
  laptop: "M5 4h14v11H5z M2 20h20l-2-5H4z",
  phone: "M7 2.5h10v19H7z M11 18.5h2",
  gamepad:
    "M6.5 8h11a4.5 4.5 0 0 1 4.4 5.5l-.8 3.6a2 2 0 0 1-3.3 1l-2.6-2.1H8.8l-2.6 2.1a2 2 0 0 1-3.3-1l-.8-3.6A4.5 4.5 0 0 1 6.5 8Z M8 10.5v4 M6 12.5h4 M16 11h.01 M18 14h.01",
  house: "M3 10.5 12 3l9 7.5 M5 9v12h14V9 M10 21v-7h4v7",
  wifi: "M2 8.5a15 15 0 0 1 20 0 M5 12a10 10 0 0 1 14 0 M8.5 15.5a5 5 0 0 1 7 0",
  shield: "M12 22s8-4 8-11V5l-8-3-8 3v6c0 7 8 11 8 11Z",
  wrench:
    "M14.7 6.3a5 5 0 0 0-6.4 6.4L3 18l3 3 5.3-5.3a5 5 0 0 0 6.4-6.4L15 12l-3-3 2.7-2.7Z",
  book: "M4 19.5A2.5 2.5 0 0 1 6.5 17H20 M6.5 2H20v20H6.5A2.5 2.5 0 0 1 4 19.5v-15A2.5 2.5 0 0 1 6.5 2Z M8 6h8",
};

function CategoryIcon({ name }: { name: string }) {
  return (
    <svg
      aria-hidden="true"
      className="h-4 w-4"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.7"
      strokeLinecap="round"
      strokeLinejoin="round"
    >
      <path d={categoryIconPaths[name] ?? categoryIconPaths.house} />
      {name === "wifi" && <circle cx="12" cy="19" r="0.8" fill="currentColor" stroke="none" />}
    </svg>
  );
}

const POPULAR_PRODUCTS_AUTOPLAY_MS = 7000;
const MAX_RECENT_SEARCHES = 5;
const MAX_RECENT_SEARCH_LABEL_LENGTH = 24;

interface Props {
  navigate: (page: Page) => void;
  authenticated: boolean;
  searchHistory: SearchHistoryEntry[];
  favorites: Set<string>;
  compareList: Set<string>;
  productCompareList: Set<string>;
  cartProductIds: ReadonlySet<string>;
  cartServiceIds: ReadonlySet<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
  onAddServiceToCart: (service: Service) => Promise<boolean>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string) => void;
  onToggleProductCompare: (id: string, category: string) => void;
}

export default function HomePage({
  navigate,
  authenticated,
  searchHistory,
  favorites,
  compareList,
  productCompareList,
  cartProductIds,
  cartServiceIds,
  onAddToCart,
  onAddServiceToCart,
  onToggleFavorite,
  onToggleCompare,
  onToggleProductCompare,
}: Props) {
  const [query, setQuery] = useState("");
  const recentSearches = authenticated
    ? searchHistory
        .filter(
          (entry) =>
          entry.query.trim().length > 0 &&
            (entry.type === "product" || entry.type === "service"),
        )
        .slice(0, MAX_RECENT_SEARCHES)
    : [];

  const [products, setProducts] = useState<Product[]>([]);
  const [services, setServices] = useState<Service[]>([]);
  const [popularProducts, setPopularProducts] = useState<SearchProductResult[]>([]);
  const [popularProductIndex, setPopularProductIndex] = useState(0);
  const [popularCarouselPaused, setPopularCarouselPaused] = useState(false);
  const [failedPopularImageId, setFailedPopularImageId] = useState<string | null>(null);
  const [popularProductsError, setPopularProductsError] = useState<string | null>(null);
  const [popularProductsAttempt, setPopularProductsAttempt] = useState(0);
  const [popularProductsLoadedAttempt, setPopularProductsLoadedAttempt] = useState<number | null>(
    null,
  );
  const [productsLoadedAttempt, setProductsLoadedAttempt] = useState<number | null>(null);
  const [servicesLoadedAttempt, setServicesLoadedAttempt] = useState<number | null>(null);
  const [productsError, setProductsError] = useState<string | null>(null);
  const [servicesError, setServicesError] = useState<string | null>(null);
  const [productsAttempt, setProductsAttempt] = useState(0);
  const [servicesAttempt, setServicesAttempt] = useState(0);
  const [searchError, setSearchError] = useState<string | null>(null);
  const popularProductsLoading = popularProductsLoadedAttempt !== popularProductsAttempt;
  const productsLoading = productsLoadedAttempt !== productsAttempt;
  const servicesLoading = servicesLoadedAttempt !== servicesAttempt;

  useEffect(() => {
    let active = true;
    getProducts()
      .then((result) => {
        if (active) setProducts(result);
      })
      .catch((error: unknown) => {
        if (active)
          setProductsError(error instanceof Error ? error.message : "Product API no disponible.");
      })
      .finally(() => {
        if (active) setProductsLoadedAttempt(productsAttempt);
      });
    return () => {
      active = false;
    };
  }, [productsAttempt]);

  useEffect(() => {
    let active = true;
    getServices()
      .then((result) => {
        if (active) setServices(result);
      })
      .catch((error: unknown) => {
        if (active)
          setServicesError(error instanceof Error ? error.message : "Services API no disponible.");
      })
      .finally(() => {
        if (active) setServicesLoadedAttempt(servicesAttempt);
      });
    return () => {
      active = false;
    };
  }, [servicesAttempt]);

  useEffect(() => {
    let active = true;
    getPopularProducts()
      .then((result) => {
        if (active) {
          setPopularProducts(result);
          setPopularProductIndex(0);
        }
      })
      .catch((error: unknown) => {
        if (active)
          setPopularProductsError(
            error instanceof Error ? error.message : "No se pudieron cargar los productos.",
          );
      })
      .finally(() => {
        if (active) setPopularProductsLoadedAttempt(popularProductsAttempt);
      });
    return () => {
      active = false;
    };
  }, [popularProductsAttempt]);

  useEffect(() => {
    if (popularProducts.length < 2 || popularCarouselPaused) return;
    const timer = window.setTimeout(() => {
      setPopularProductIndex((index) => (index + 1) % popularProducts.length);
    }, POPULAR_PRODUCTS_AUTOPLAY_MS);
    return () => window.clearTimeout(timer);
  }, [popularCarouselPaused, popularProductIndex, popularProducts.length]);

  const featuredProducts = products.slice(0, 4);

  const handleSearch = (e: FormEvent) => {
    e.preventDefault();
    const trimmedQuery = query.trim();
    if (!trimmedQuery) {
      setSearchError("Escribe un producto para iniciar la búsqueda.");
      return;
    }
    if (trimmedQuery.length > 200) {
      setSearchError("La búsqueda no puede superar los 200 caracteres.");
      return;
    }
    if (trimmedQuery.split(/\s+/).length > 5) {
      setSearchError("La búsqueda admite un máximo de 5 palabras.");
      return;
    }
    setSearchError(null);
    navigate({ id: "search-products", query: trimmedQuery });
  };

  const featuredServices = services.slice(0, 4);
  const activePopularProduct = popularProducts[popularProductIndex] ?? null;
  const changePopularProduct = (direction: -1 | 1) => {
    if (popularProducts.length < 2) return;
    setPopularProductIndex(
      (index) => (index + direction + popularProducts.length) % popularProducts.length,
    );
  };

  return (
    <div>
      {/* Hero */}
      <section
        style={{
          background:
            "radial-gradient(circle at top, rgba(124, 58, 237, 0.28), transparent 32%), radial-gradient(circle at 20% 20%, rgba(45, 212, 191, 0.12), transparent 28%), linear-gradient(135deg, #0b1020 0%, #0f172a 42%, #111827 100%)",
          borderBottom: "1px solid rgba(148, 163, 184, 0.2)",
        }}
        className="relative overflow-hidden py-20 px-4"
      >
        <div className="absolute inset-0 opacity-30" aria-hidden="true">
          <div className="absolute -top-20 left-1/2 h-72 w-72 -translate-x-1/2 rounded-full bg-gradient-to-r from-[#7c3aed]/35 via-[#ff6b6b]/20 to-[#22d3ee]/20 blur-3xl" />
        </div>

        <div className="relative max-w-4xl mx-auto text-center">
          <h1 className="text-3xl md:text-5xl font-black text-white mb-4 leading-[0.95] tracking-[-0.05em]">
            Compara precios,
            <br />
            <span className="bg-gradient-to-r from-[#ff8a65] via-[#ff6b6b] to-[#8b5cf6] bg-clip-text text-transparent">
              elige mejor.
            </span>
          </h1>
          <p className="text-base text-slate-300 max-w-xl mx-auto mb-10 leading-relaxed">
            Busca productos y servicios entre cientos de tiendas y proveedores. Compara, ahorra y
            elige con confianza.
          </p>

          {/* Search */}
          <form onSubmit={handleSearch} className="flex gap-2 max-w-2xl mx-auto">
            <div className="relative flex-1">
              <svg
                className="absolute left-4 top-1/2 -translate-y-1/2 text-slate-400"
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <circle cx="11" cy="11" r="8" />
                <path d="m21 21-4.35-4.35" />
              </svg>
              <input
                value={query}
                onChange={(e) => {
                  setQuery(e.target.value);
                  setSearchError(null);
                }}
                placeholder="¿Qué producto estás buscando?"
                maxLength={200}
                aria-invalid={Boolean(searchError)}
                aria-describedby={searchError ? "home-search-error" : undefined}
                style={{
                  background: "rgba(15, 23, 42, 0.8)",
                  border: "1px solid rgba(148, 163, 184, 0.25)",
                }}
                className="w-full pl-11 pr-4 py-3.5 rounded-2xl text-sm text-white placeholder-slate-400 focus:outline-none focus:border-[#8b5cf6] transition-colors duration-200 shadow-[0_12px_30px_rgba(15,23,42,0.4)]"
              />
            </div>
            <button
              type="submit"
              style={{
                background: "linear-gradient(135deg, #ff7a59 0%, #ff5f7b 32%, #8b5cf6 100%)",
                color: "#fff",
                boxShadow: "0 12px 30px rgba(139, 92, 246, 0.35)",
              }}
              className="px-6 py-3.5 rounded-2xl font-semibold text-sm hover:scale-[1.01] transition-all prime-glow whitespace-nowrap"
            >
              Buscar
            </button>
          </form>
          {searchError && (
            <p id="home-search-error" role="alert" className="mt-2 text-sm text-rose-300">
              {searchError}
            </p>
          )}

          {recentSearches.length > 0 && (
            <div className="mt-5 flex flex-wrap items-center justify-center gap-2">
              <span className="w-full text-center text-xs text-slate-400">
                Tus búsquedas recientes
              </span>
              {recentSearches.map((entry) => {
                const recentQuery = entry.query.trim();
                const label =
                  recentQuery.length > MAX_RECENT_SEARCH_LABEL_LENGTH
                    ? `${recentQuery.slice(0, MAX_RECENT_SEARCH_LABEL_LENGTH - 1).trimEnd()}…`
                    : recentQuery;

                return (
                  <button
                    key={entry.id}
                    type="button"
                    title={recentQuery}
                    aria-label={`Buscar ${entry.type === "service" ? "servicios" : "productos"}: ${recentQuery}`}
                    onClick={() =>
                      navigate({
                        id: entry.type === "service" ? "search-services" : "search-products",
                        query: recentQuery,
                      })
                    }
                    style={{
                      background: "rgba(15, 23, 42, 0.7)",
                      border: "1px solid rgba(148, 163, 184, 0.2)",
                      color: "#cbd5e1",
                    }}
                    className="px-3 py-1.5 rounded-full text-xs hover:border-[#8b5cf6] hover:text-white transition-all"
                  >
                    {label}
                  </button>
                );
              })}
            </div>
          )}
        </div>
      </section>

      <section
        aria-labelledby="popular-products-title"
        style={{
          background: "linear-gradient(180deg, #0b1020 0%, #101827 100%)",
          borderBottom: "1px solid rgba(148, 163, 184, 0.2)",
        }}
        className="px-4 py-8"
      >
        <div className="max-w-7xl mx-auto">
          <div className="mb-5 flex flex-wrap items-center justify-between gap-4">
            <div>
              <h2 id="popular-products-title" className="text-xl font-bold text-white">
                Descubre estos productos destacados
              </h2>
              <p className="mt-1 text-sm text-slate-400">
                Productos que están marcando tendencia
              </p>
            </div>
          </div>

          {popularProductsLoading && (
            <p className="py-4 text-sm text-slate-400" role="status">
              Cargando productos más buscados...
            </p>
          )}
          {popularProductsError && (
            <div className="rounded-xl border border-rose-300/20 bg-rose-950/30 p-4 text-sm text-rose-200" role="alert">
              No se pudieron cargar los productos más buscados: {popularProductsError}
              <button
                type="button"
                onClick={() => {
                  setPopularProductsError(null);
                  setPopularProductsAttempt((attempt) => attempt + 1);
                }}
                className="ml-3 rounded-md font-semibold underline underline-offset-4 transition-colors hover:text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-rose-200"
              >
                Reintentar
              </button>
            </div>
          )}
          {!popularProductsLoading && !popularProductsError && popularProducts.length === 0 && (
            <p className="py-4 text-sm text-slate-400">
              Aún no hay búsquedas suficientes para mostrar productos.
            </p>
          )}
          {!popularProductsLoading && !popularProductsError && activePopularProduct && (
            <div
              className="mx-auto max-w-6xl"
              onMouseEnter={() => setPopularCarouselPaused(true)}
              onMouseLeave={() => setPopularCarouselPaused(false)}
              onFocus={() => setPopularCarouselPaused(true)}
              onBlur={(event) => {
                const nextTarget = event.relatedTarget;
                if (!(nextTarget instanceof Node) || !event.currentTarget.contains(nextTarget)) {
                  setPopularCarouselPaused(false);
                }
              }}
            >
              <article
                role="group"
                aria-roledescription="diapositiva"
                aria-label={`Producto ${popularProductIndex + 1} de ${popularProducts.length}: ${activePopularProduct.name}`}
                className="relative isolate aspect-[4/5] w-full overflow-hidden rounded-[2rem] border border-white/10 bg-slate-950 shadow-2xl shadow-black/30 ring-1 ring-white/5 sm:aspect-[16/9] sm:min-h-[340px] lg:min-h-[400px]"
              >
                <div aria-hidden="true" className="absolute inset-0 bg-slate-800">
                  <span className="absolute inset-0 flex items-center justify-center text-5xl">
                    🛍️
                  </span>
                  {activePopularProduct.imageUrl &&
                  failedPopularImageId !== activePopularProduct.id ? (
                    <img
                      src={activePopularProduct.imageUrl}
                      alt=""
                      loading="lazy"
                      onError={() => setFailedPopularImageId(activePopularProduct.id)}
                      className="absolute inset-0 h-full w-full object-cover object-center md:w-[72%]"
                    />
                  ) : null}
                  <div className="absolute inset-0 bg-gradient-to-tr from-slate-950/20 via-transparent to-slate-950/10" />
                </div>
                <div
                  aria-hidden="true"
                  className="absolute inset-x-0 bottom-0 h-[60%] md:hidden"
                  style={{
                    background: "#0f172a",
                    clipPath: "polygon(0 14%, 100% 0, 100% 100%, 0 100%)",
                  }}
                />
                <div
                  aria-hidden="true"
                  className="absolute inset-y-0 right-0 hidden w-[60%] md:block"
                  style={{
                    background: "#0f172a",
                    clipPath: "polygon(18% 0, 100% 0, 100% 100%, 0 100%)",
                  }}
                />
                <div className="absolute inset-x-14 bottom-8 z-10 flex flex-col items-center gap-3 text-center sm:inset-x-16 md:inset-x-auto md:right-16 md:top-1/2 md:bottom-auto md:w-[36%] md:-translate-y-1/2 md:items-start md:text-left lg:right-20">
                  <span className="inline-flex items-center gap-2 rounded-full border border-violet-200/20 bg-violet-300/10 px-3 py-1 text-[10px] font-bold uppercase tracking-[0.16em] text-violet-100 backdrop-blur-sm">
                    <span className="h-1.5 w-1.5 rounded-full bg-cyan-300 shadow-[0_0_10px_rgba(103,232,249,0.8)]" />
                    Más buscado
                  </span>
                  <p className="text-xs font-medium uppercase tracking-[0.14em] text-cyan-200/90">
                    {activePopularProduct.brand} · {activePopularProduct.category}
                  </p>
                  <h3 className="line-clamp-3 text-xl font-bold leading-tight text-white sm:text-2xl md:text-3xl">
                    {activePopularProduct.name}
                  </h3>
                  <button
                    type="button"
                    onClick={() =>
                      navigate({ id: "product-detail", productId: activePopularProduct.id })
                    }
                    className="mt-2 inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#ff7a59] to-[#8b5cf6] px-6 py-3 text-sm font-semibold text-white shadow-lg shadow-violet-950/30 transition-all duration-200 hover:-translate-y-0.5 hover:shadow-xl hover:shadow-violet-950/40 active:translate-y-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white"
                  >
                    Ver producto
                    <svg
                      aria-hidden="true"
                      viewBox="0 0 20 20"
                      fill="none"
                      className="h-4 w-4"
                    >
                      <path
                        d="M4.5 10h11m-4.5-4.5L15.5 10 11 14.5"
                        stroke="currentColor"
                        strokeWidth="1.7"
                        strokeLinecap="round"
                        strokeLinejoin="round"
                      />
                    </svg>
                  </button>
                </div>
                {popularProducts.length > 1 && (
                  <>
                    <button
                      type="button"
                      aria-label="Ver producto anterior"
                      onClick={() => changePopularProduct(-1)}
                      className="absolute left-3 top-1/2 z-20 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full border border-white/20 bg-slate-950/65 text-white shadow-lg shadow-black/20 backdrop-blur-md transition-all duration-200 hover:scale-105 hover:border-violet-200/70 hover:bg-violet-600/80 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white sm:left-5 sm:h-12 sm:w-12"
                    >
                      <span aria-hidden="true" className="text-xl leading-none">
                        ←
                      </span>
                    </button>
                    <button
                      type="button"
                      aria-label="Ver siguiente producto"
                      onClick={() => changePopularProduct(1)}
                      className="absolute right-3 top-1/2 z-20 flex h-11 w-11 -translate-y-1/2 items-center justify-center rounded-full border border-white/20 bg-slate-950/65 text-white shadow-lg shadow-black/20 backdrop-blur-md transition-all duration-200 hover:scale-105 hover:border-violet-200/70 hover:bg-violet-600/80 active:scale-95 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-white sm:right-5 sm:h-12 sm:w-12"
                    >
                      <span aria-hidden="true" className="text-xl leading-none">
                        →
                      </span>
                    </button>
                  </>
                )}
              </article>
              {popularProducts.length > 1 && (
                <div className="mt-5 flex items-center justify-center gap-3">
                  <div
                    role="group"
                    aria-label="Seleccionar producto destacado"
                    className="flex items-center gap-1.5"
                  >
                    {popularProducts.map((product, index) => (
                      <button
                        key={product.id}
                        type="button"
                        aria-label={`Mostrar producto ${index + 1}: ${product.name}`}
                        aria-pressed={index === popularProductIndex}
                        onClick={() => setPopularProductIndex(index)}
                        className={`h-2.5 rounded-full transition-all duration-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-violet-200 ${
                          index === popularProductIndex
                            ? "w-7 bg-violet-300 shadow-[0_0_12px_rgba(196,181,253,0.55)]"
                            : "w-2.5 bg-slate-600 hover:bg-slate-300"
                        }`}
                      />
                    ))}
                  </div>
                  <span className="min-w-[3.5rem] text-xs font-medium tabular-nums text-slate-400">
                    {String(popularProductIndex + 1).padStart(2, "0")}
                    <span className="px-1 text-slate-600">/</span>
                    {String(popularProducts.length).padStart(2, "0")}
                  </span>
                </div>
              )}
            </div>
          )}
        </div>
      </section>

      <div className="max-w-7xl mx-auto px-4">
        <div className="flex flex-wrap justify-center gap-2 pt-8 pb-6">
          {featuredCategories.map((cat) => (
            <button
              key={`${cat.kind}-${cat.name}`}
              onClick={() =>
                cat.kind === "service"
                  ? navigate({ id: "search-services", query: cat.name })
                  : navigate({ id: "search-products", query: "", productGroup: cat.id })
              }
              style={{
                background: "rgba(15, 23, 42, 0.72)",
                border: "1px solid rgba(148, 163, 184, 0.25)",
                boxShadow: "0 10px 25px rgba(15, 23, 42, 0.2)",
              }}
              className="group inline-flex items-center gap-2 rounded-full px-3 py-2 transition-all duration-200 hover:-translate-y-0.5 hover:border-[#a78bfa]"
            >
              <span
                className="flex h-7 w-7 items-center justify-center rounded-full"
                style={{
                  background: `${cat.color}20`,
                  color: cat.color,
                  boxShadow: `inset 0 0 0 1px ${cat.color}40`,
                }}
              >
                <CategoryIcon name={cat.icon} />
              </span>
              <span className="text-[11px] font-semibold text-slate-100 group-hover:text-white">
                {cat.name}
              </span>
            </button>
          ))}
        </div>

        {/* Featured products */}
        <section className="pb-12">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Productos destacados</h2>
              <p className="text-xs text-muted mt-1">Explora nuestra selección de productos</p>
            </div>
            <button
              onClick={() => navigate({ id: "search-products", query: "" })}
              className="text-sm font-medium transition-colors bg-gradient-to-r from-[#ff7a59] to-[#8b5cf6] bg-clip-text text-transparent"
            >
              Ver todos →
            </button>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            {productsError && (
              <div
                className="col-span-full rounded-xl border border-rose-300/20 bg-rose-950/30 p-4 text-sm text-rose-200"
                role="alert"
              >
                Product API: {productsError}
                <button
                  type="button"
                  onClick={() => {
                    setProducts([]);
                    setProductsError(null);
                    setProductsAttempt((attempt) => attempt + 1);
                  }}
                  className="ml-3 font-semibold underline"
                >
                  Reintentar
                </button>
              </div>
            )}
            {productsLoading && (
              <p className="col-span-full py-4 text-sm text-muted" role="status">
                Cargando productos...
              </p>
            )}
            {!productsLoading && !productsError && featuredProducts.length === 0 && (
              <p className="col-span-full py-4 text-sm text-muted">
                Product API no devolvió productos destacados.
              </p>
            )}
            {featuredProducts.map((p) => (
              <ProductCard
                key={p.id}
                product={p}
                navigate={navigate}
                isFavorite={favorites.has(p.id)}
                isComparing={productCompareList.has(p.id)}
                cartProductIds={cartProductIds}
                onAddToCart={onAddToCart}
                onToggleFavorite={onToggleFavorite}
                onToggleCompare={onToggleProductCompare}
              />
            ))}
          </div>
        </section>

        {/* Featured services */}
        <section className="pb-16">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Servicios populares</h2>
              <p className="text-xs text-muted mt-1">Encuentra el mejor plan para ti</p>
            </div>
            <button
              onClick={() => navigate({ id: "search-services", query: "" })}
              className="text-sm font-medium transition-colors bg-gradient-to-r from-[#22d3ee] to-[#8b5cf6] bg-clip-text text-transparent"
            >
              Ver todos →
            </button>
          </div>
          <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
            {servicesError && (
              <div
                className="col-span-full rounded-xl border border-rose-300/20 bg-rose-950/30 p-4 text-sm text-rose-200"
                role="alert"
              >
                Services API: {servicesError}
                <button
                  type="button"
                  onClick={() => {
                    setServices([]);
                    setServicesError(null);
                    setServicesAttempt((attempt) => attempt + 1);
                  }}
                  className="ml-3 font-semibold underline"
                >
                  Reintentar
                </button>
              </div>
            )}
            {servicesLoading && (
              <p className="col-span-full py-4 text-sm text-muted" role="status">
                Cargando servicios...
              </p>
            )}
            {!servicesLoading && !servicesError && featuredServices.length === 0 && (
              <p className="col-span-full py-4 text-sm text-muted">
                Services API no devolvió servicios populares.
              </p>
            )}
            {featuredServices.map((s) => (
              <ServiceCard
                key={s.id}
                service={s}
                navigate={navigate}
                isFavorite={favorites.has(s.id)}
                isComparing={compareList.has(s.id)}
                isInCart={cartServiceIds.has(s.id)}
                onToggleFavorite={onToggleFavorite}
                onToggleCompare={onToggleCompare}
                onAddToCart={onAddServiceToCart}
              />
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}
