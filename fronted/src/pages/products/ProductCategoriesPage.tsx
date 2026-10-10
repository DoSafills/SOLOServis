import { useEffect, useMemo, useState } from "react";
import type { Page, Product } from "../../types";
import { getProducts } from "../../services/api/api";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
}

const fallbackCategoryImages = [
  "https://images.unsplash.com/photo-1511707171634-5f897ff02aa9?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1496181133206-80ce9b88a853?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1555041469-a586c61ea9bc?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1505740420928-5e560c06d30e?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1523275335684-37898b6baf30?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1542291026-7eec264c27ff?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1586023492125-27b2c045efd7?auto=format&fit=crop&w=320&q=80",
  "https://images.unsplash.com/photo-1583394838336-acd977736f90?auto=format&fit=crop&w=320&q=80",
];

const categoryColorStyles = [
  {
    card: "from-blue-950 via-slate-900 to-cyan-950 border-cyan-300/25 hover:border-cyan-200/70",
    accent: "from-cyan-300 to-blue-400",
    image: "from-cyan-400/30 to-blue-500/25 border-cyan-100/25",
    glow: "bg-cyan-400/15 group-hover:bg-cyan-300/25",
    title: "text-white",
    count: "border-cyan-200/20 bg-cyan-300/10 text-cyan-100",
    action: "border-cyan-100/20 bg-cyan-300/10 text-cyan-100 group-hover:bg-cyan-300/20",
  },
  {
    card: "from-fuchsia-950 via-slate-900 to-purple-950 border-fuchsia-300/25 hover:border-fuchsia-200/70",
    accent: "from-fuchsia-300 to-violet-400",
    image: "from-fuchsia-400/30 to-purple-500/25 border-fuchsia-100/25",
    glow: "bg-fuchsia-400/15 group-hover:bg-fuchsia-300/25",
    title: "text-white",
    count: "border-fuchsia-200/20 bg-fuchsia-300/10 text-fuchsia-100",
    action:
      "border-fuchsia-100/20 bg-fuchsia-300/10 text-fuchsia-100 group-hover:bg-fuchsia-300/20",
  },
  {
    card: "from-emerald-950 via-slate-900 to-teal-950 border-emerald-300/25 hover:border-emerald-200/70",
    accent: "from-emerald-300 to-teal-400",
    image: "from-emerald-400/30 to-teal-500/25 border-emerald-100/25",
    glow: "bg-emerald-400/15 group-hover:bg-emerald-300/25",
    title: "text-white",
    count: "border-emerald-200/20 bg-emerald-300/10 text-emerald-100",
    action:
      "border-emerald-100/20 bg-emerald-300/10 text-emerald-100 group-hover:bg-emerald-300/20",
  },
  {
    card: "from-amber-950 via-slate-900 to-rose-950 border-amber-300/25 hover:border-amber-200/70",
    accent: "from-amber-300 to-rose-400",
    image: "from-amber-400/30 to-rose-500/25 border-amber-100/25",
    glow: "bg-amber-400/15 group-hover:bg-amber-300/25",
    title: "text-white",
    count: "border-amber-200/20 bg-amber-300/10 text-amber-100",
    action: "border-amber-100/20 bg-amber-300/10 text-amber-100 group-hover:bg-amber-300/20",
  },
];

const categorySections = [
  {
    id: "home",
    title: "Hogar",
    description: "Electrodomésticos, muebles y productos para tu casa",
    icon: "🏠",
    card: "border-amber-200/20 bg-gradient-to-r from-amber-950/55 via-slate-950/70 to-rose-950/35",
    accent: "text-amber-200",
  },
  {
    id: "technology",
    title: "Tecnología",
    description: "TV, videojuegos, audio, computación y telefonía",
    icon: "⚡",
    card: "border-cyan-200/20 bg-gradient-to-r from-cyan-950/55 via-slate-950/70 to-violet-950/35",
    accent: "text-cyan-200",
  },
  {
    id: "other",
    title: "Otras categorías",
    description: "Deportes, fitness y nuevos tipos de productos",
    icon: "✨",
    card: "border-fuchsia-200/20 bg-gradient-to-r from-fuchsia-950/45 via-slate-950/70 to-indigo-950/35",
    accent: "text-fuchsia-200",
  },
] as const;

type CategorySectionId = (typeof categorySections)[number]["id"];

const normalizeCategoryName = (value: string): string =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase();

const getCategorySection = (name: string): CategorySectionId => {
  const normalized = normalizeCategoryName(name);
  if (
    /hogar|mueble|mesa|silla|sofa|cama|electrodom|refriger|lavado|lavadora|secadora|cocina|microonda|horno|aspirador/.test(
      normalized,
    )
  ) {
    return "home";
  }
  if (
    /tecnolog|electron|televisor|television|\btv\b|videojuego|computacion|computador|notebook|telefon|audio|gaming|consola|celular|smartphone/.test(
      normalized,
    )
  ) {
    return "technology";
  }
  return "other";
};

export default function ProductCategoriesPage({ navigate }: Props) {
  const [products, setProducts] = useState<Product[]>([]);
  const [expandedSections, setExpandedSections] = useState<Set<CategorySectionId>>(
    () => new Set(["home"]),
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    getProducts()
      .then((result) => {
        if (!cancelled) setProducts(result);
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "No se pudieron cargar las categorías");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const categoriesBySection = useMemo(() => {
    const groupedProducts = products.reduce((grouped, product) => {
      const name = product.category.trim();
      if (!name) return grouped;
      const items = grouped.get(name) ?? [];
      items.push(product);
      grouped.set(name, items);
      return grouped;
    }, new Map<string, Product[]>());
    const usedImages = new Set<string>();
    const categories = [...groupedProducts]
      .sort(([first], [second]) => first.localeCompare(second, "es"))
      .map(([name, items]) => {
        const productImages = items.flatMap((product) => [product.image, ...product.images]);
        const image = productImages.find((url) => url && !usedImages.has(url));
        const fallbackImage = fallbackCategoryImages.find((url) => !usedImages.has(url));
        const uniqueImage = image ?? fallbackImage ?? fallbackCategoryImages[0];
        usedImages.add(uniqueImage);

        return { name, count: items.length, image: uniqueImage };
      });

    return categorySections.map((section) => ({
      ...section,
      categories: categories.filter((category) => getCategorySection(category.name) === section.id),
    }));
  }, [products]);

  const totalProducts = products.length;

  const showAllProducts = () => navigate({ id: "search-products", query: "" });

  const toggleSection = (sectionId: CategorySectionId) => {
    setExpandedSections((previous) => {
      const next = new Set(previous);
      if (next.has(sectionId)) next.delete(sectionId);
      else next.add(sectionId);
      return next;
    });
  };

  return (
    <div className="max-w-7xl mx-auto px-4 py-8 sm:py-12">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Productos" },
        ]}
      />

      <section className="mt-6 mb-7 flex flex-col gap-4 border-b border-white/10 pb-7 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="mb-2 text-xs font-semibold uppercase tracking-[0.16em] text-prime">
            Catálogo
          </p>
          <h1 className="text-2xl font-bold text-white sm:text-3xl">Explora por categoría</h1>
          <p className="mt-2 text-sm text-slate-300">
            Compara precios y encuentra tu próxima oferta.
          </p>
        </div>
        <button
          onClick={showAllProducts}
          className="inline-flex items-center justify-center gap-2 self-start rounded-lg border border-white/25 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:border-white hover:bg-white/10 sm:self-auto"
        >
          Ver todas las ofertas
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M5 12h14m-6-6 6 6-6 6"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </section>

      {loading ? (
        <p className="py-8 text-center text-sm text-muted">Cargando categorías...</p>
      ) : error ? (
        <div className="py-8 text-center">
          <p className="mb-4 text-sm text-warn">{error}</p>
          <button onClick={showAllProducts} className="text-sm font-semibold text-prime">
            Ir a todos los productos
          </button>
        </div>
      ) : totalProducts === 0 ? (
        <div className="py-8 text-center">
          <p className="mb-4 text-sm text-muted">Todavía no hay categorías disponibles.</p>
          <button onClick={showAllProducts} className="text-sm font-semibold text-prime">
            Ver todos los productos
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {categoriesBySection.map((section) => {
            const isExpanded = expandedSections.has(section.id);
            const productCount = section.categories.reduce(
              (total, category) => total + category.count,
              0,
            );
            const panelId = `category-section-${section.id}`;

            return (
              <section
                key={section.id}
                className={`overflow-hidden rounded-2xl border ${section.card}`}
              >
                <button
                  type="button"
                  aria-expanded={isExpanded}
                  aria-controls={panelId}
                  onClick={() => toggleSection(section.id)}
                  className="flex w-full items-center gap-4 p-4 text-left transition hover:bg-white/[0.035] sm:p-5"
                >
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-slate-950/45 text-2xl shadow-inner">
                    {section.icon}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span className={`block text-lg font-extrabold sm:text-xl ${section.accent}`}>
                      {section.title}
                    </span>
                    <span className="mt-1 block text-xs text-slate-300 sm:text-sm">
                      {section.description}
                    </span>
                  </span>
                  <span className="hidden shrink-0 rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs font-bold text-slate-200 sm:block">
                    {section.categories.length}{" "}
                    {section.categories.length === 1 ? "categoría" : "categorías"} · {productCount}{" "}
                    {productCount === 1 ? "producto" : "productos"}
                  </span>
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden="true"
                    className={`shrink-0 text-slate-300 transition-transform ${isExpanded ? "rotate-180" : ""}`}
                  >
                    <path
                      d="m6 9 6 6 6-6"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </button>

                {isExpanded && (
                  <div id={panelId} className="border-t border-white/10 p-3 sm:p-5">
                    {section.categories.length === 0 ? (
                      <p className="px-2 py-3 text-sm text-slate-300">
                        Todavía no hay productos disponibles en esta sección.
                      </p>
                    ) : (
                      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                        {section.categories.map((category, index) => {
                          const colors = categoryColorStyles[index % categoryColorStyles.length];

                          return (
                            <button
                              key={category.name}
                              onClick={() =>
                                navigate({
                                  id: "search-products",
                                  query: "",
                                  category: category.name,
                                })
                              }
                              className={`group relative flex min-h-32 min-w-0 items-center gap-3 overflow-hidden rounded-2xl border bg-gradient-to-br ${colors.card} p-3.5 text-left shadow-lg shadow-slate-950/25 transition duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-violet-950/35 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300 sm:gap-4 sm:p-4`}
                            >
                              <span
                                aria-hidden="true"
                                className={`pointer-events-none absolute -right-8 -top-10 h-36 w-36 rounded-full blur-2xl transition ${colors.glow}`}
                              />
                              <span
                                aria-hidden="true"
                                className={`absolute inset-x-0 top-0 h-1 bg-gradient-to-r ${colors.accent} opacity-80`}
                              />
                              <span
                                className={`relative flex h-[4.5rem] w-[4.5rem] shrink-0 items-center justify-center rounded-2xl border bg-gradient-to-br ${colors.image} p-1.5 shadow-lg shadow-black/20 sm:h-20 sm:w-20`}
                              >
                                <img
                                  src={category.image}
                                  alt=""
                                  loading="lazy"
                                  className="h-full w-full rounded-xl object-contain drop-shadow-lg transition-transform duration-300 group-hover:scale-110"
                                />
                              </span>
                              <span className="relative flex min-w-0 flex-1 flex-col justify-center">
                                <span
                                  className={`line-clamp-2 text-base font-extrabold leading-snug sm:text-lg ${colors.title}`}
                                >
                                  {category.name}
                                </span>
                                <span
                                  className={`mt-2 inline-flex w-fit items-center rounded-full border px-2.5 py-1 text-xs font-bold ${colors.count}`}
                                >
                                  {category.count} {category.count === 1 ? "producto" : "productos"}
                                </span>
                                <span
                                  className={`mt-2.5 inline-flex w-fit items-center gap-1 rounded-lg border px-2.5 py-1 text-xs font-extrabold transition-colors ${colors.action}`}
                                >
                                  Explorar
                                  <svg
                                    width="14"
                                    height="14"
                                    viewBox="0 0 24 24"
                                    fill="none"
                                    aria-hidden="true"
                                    className="transition-transform group-hover:translate-x-1"
                                  >
                                    <path
                                      d="M5 12h14m-6-6 6 6-6 6"
                                      stroke="currentColor"
                                      strokeWidth="2"
                                      strokeLinecap="round"
                                      strokeLinejoin="round"
                                    />
                                  </svg>
                                </span>
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </div>
                )}
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
