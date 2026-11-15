import { useEffect, useMemo, useState } from "react";
import type { Page, Product } from "../../types";
import { getAllProducts } from "../../services/api/api";
import { getCategories, type ApiProductCategory } from "../../services/api/products";
import { Breadcrumb } from "../../components/common/ui";
import CategoryIcon from "../../components/common/CategoryIcon";
import {
  CATEGORY_SECTION_ICONS,
  getProductCategoryGroup,
  getProductSubtype,
  getProductSubtypeSearchQuery,
} from "../../constants/productCatalog";

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

const categorySections = [
  {
    id: "home",
    title: "Hogar",
    description: "Electrodomésticos, muebles y productos para tu casa",
    icon: CATEGORY_SECTION_ICONS.home,
  },
  {
    id: "technology",
    title: "Tecnología",
    description: "TV, videojuegos, audio, computación y telefonía",
    icon: CATEGORY_SECTION_ICONS.technology,
  },
  {
    id: "other",
    title: "Otras categorías",
    description: "Deportes, fitness y nuevos tipos de productos",
    icon: CATEGORY_SECTION_ICONS.other,
  },
] as const;

type CategorySectionId = (typeof categorySections)[number]["id"];

export default function ProductCategoriesPage({ navigate }: Props) {
  const [products, setProducts] = useState<Product[]>([]);
  const [categories, setCategories] = useState<ApiProductCategory[]>([]);
  const [expandedSections, setExpandedSections] = useState<Set<CategorySectionId>>(
    () => new Set(["home"]),
  );
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    Promise.all([getAllProducts(), getCategories()])
      .then(([result, categoryList]) => {
        if (!cancelled) {
          setProducts(result);
          setCategories(categoryList);
        }
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

  const { categoriesBySection, rootProductCounts } = useMemo(() => {
    const categoriesById = new Map(categories.map((category) => [category.id, category]));
    const childrenByParent = new Map<number, ApiProductCategory[]>();
    for (const category of categories) {
      if (category.parentId === null) continue;
      const children = childrenByParent.get(category.parentId) ?? [];
      children.push(category);
      childrenByParent.set(category.parentId, children);
    }

    const getDescendantIds = (categoryId: number) => {
      const descendants = new Set([categoryId]);
      const pending = [categoryId];
      while (pending.length) {
        const parentId = pending.pop();
        if (parentId === undefined) continue;
        for (const child of childrenByParent.get(parentId) ?? []) {
          if (!descendants.has(child.id)) {
            descendants.add(child.id);
            pending.push(child.id);
          }
        }
      }
      return descendants;
    };

    const rootCategoryFor = (category: ApiProductCategory) => {
      let current = category;
      while (current.parentId !== null) {
        const parent = categoriesById.get(current.parentId);
        if (!parent) break;
        current = parent;
      }
      return current;
    };

    const productsByCategory = new Map(
      categories.map((category) => {
        const descendants = getDescendantIds(category.id);
        return [
          category.id,
          products.filter(
            (product) => product.categoryId !== undefined && descendants.has(product.categoryId),
          ),
        ] as const;
      }),
    );
    const rootCounts = new Map(
      categories
        .filter((category) => category.parentId === null)
        .map(
          (category) => [category.id, productsByCategory.get(category.id)?.length ?? 0] as const,
        ),
    );

    const usedImages = new Set<string>();
    const makeCard = (
      name: string,
      items: Product[],
      root: ApiProductCategory,
      categoryId: number,
      query = "",
    ) => {
      const productImages = items.flatMap((product) => [product.image, ...product.images]);
      const image = productImages.find((url) => url && !usedImages.has(url));
      const fallbackImage = fallbackCategoryImages.find((url) => !usedImages.has(url));
      const uniqueImage = image ?? fallbackImage ?? fallbackCategoryImages[0];
      usedImages.add(uniqueImage);
      return {
        name,
        rootName: root.name,
        count: items.length,
        image: uniqueImage,
        categoryId,
        query,
        sectionId: getProductCategoryGroup(root.name),
      };
    };

    const categoryCards = categories
      .filter((category) => category.parentId !== null)
      .flatMap((category) => {
        const matchingProducts = productsByCategory.get(category.id) ?? [];
        const childHasProducts = (childrenByParent.get(category.id) ?? []).some(
          (child) => (productsByCategory.get(child.id)?.length ?? 0) > 0,
        );
        if (matchingProducts.length === 0 || childHasProducts) return [];
        return [makeCard(category.name, matchingProducts, rootCategoryFor(category), category.id)];
      });

    const directProductsBySubtype = new Map<string, Product[]>();
    for (const product of products) {
      const category = product.categoryId ? categoriesById.get(product.categoryId) : undefined;
      if (!category) continue;
      const hasProductChild = (childrenByParent.get(category.id) ?? []).some(
        (child) => (productsByCategory.get(child.id)?.length ?? 0) > 0,
      );
      if (category.parentId !== null && !hasProductChild) continue;
      const subtype = getProductSubtype(product);
      const key = `${category.id}:${subtype}`;
      const items = directProductsBySubtype.get(key) ?? [];
      items.push(product);
      directProductsBySubtype.set(key, items);
    }

    const inferredCards = [...directProductsBySubtype.entries()].flatMap(([key, items]) => {
      const [rootId, ...subtypeParts] = key.split(":");
      const root = categoriesById.get(Number(rootId));
      if (!root) return [];
      const subtype = subtypeParts.join(":");
      return [makeCard(subtype, items, root, root.id, getProductSubtypeSearchQuery(subtype))];
    });
    const displayCards = [...categoryCards, ...inferredCards].sort((first, second) =>
      first.name.localeCompare(second.name, "es"),
    );

    return {
      categoriesBySection: categorySections.map((section) => ({
        ...section,
        categories: displayCards.filter((category) => category.sectionId === section.id),
        rootCategories: categories.filter(
          (category) =>
            category.parentId === null && getProductCategoryGroup(category.name) === section.id,
        ),
      })),
      rootProductCounts: rootCounts,
    };
  }, [categories, products]);

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

      <section className="mt-6 mb-7 flex flex-col gap-4 border-b border-bd pb-6 sm:flex-row sm:items-end sm:justify-between">
        <div className="min-w-0">
          <p className="mb-2 text-[10px] font-semibold uppercase tracking-[0.2em] text-prime">
            Catálogo
          </p>
          <h1 className="text-xl font-semibold tracking-tight text-text sm:text-2xl">
            Explora por categoría
          </h1>
          <p className="mt-1.5 text-sm text-muted">
            Compara precios y encuentra tu próxima oferta.
          </p>
        </div>
        <div className="flex flex-wrap items-center gap-3 self-start sm:self-auto">
          <button
            onClick={showAllProducts}
            className="inline-flex items-center justify-center gap-2 rounded-lg bg-prime px-3.5 py-2 text-xs font-semibold text-[#181714] transition hover:bg-prime-dark"
          >
            Ver ofertas
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
        </div>
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
            const panelId = `category-section-${section.id}`;
            const sectionRoots = section.rootCategories.filter(
              (category) => (rootProductCounts.get(category.id) ?? 0) > 0,
            );
            const productCount = sectionRoots.reduce(
              (total, category) => total + (rootProductCounts.get(category.id) ?? 0),
              0,
            );

            return (
              <section
                key={section.id}
                className={`category-outline relative isolate overflow-hidden rounded-xl bg-surface-3 p-px ${
                  isExpanded ? "category-outline-active" : ""
                }`}
              >
                <div className="relative z-10 overflow-hidden rounded-[11px] bg-surface">
                  <div>
                    <button
                      type="button"
                      onClick={() => toggleSection(section.id)}
                      aria-expanded={isExpanded}
                      aria-controls={panelId}
                      aria-label={`${isExpanded ? "Ocultar" : "Ver"} subcategorías de ${section.title}`}
                      className="flex min-w-0 w-full items-center gap-3 px-4 pt-4 pb-2 text-left sm:gap-4 sm:px-5 sm:pt-5"
                    >
                      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-prime">
                        <CategoryIcon name={section.icon} />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block text-base font-semibold text-text sm:text-lg">
                          {section.title}
                        </span>
                        <span className="mt-1 block text-xs text-muted sm:text-sm">
                          {section.description}
                        </span>
                      </span>
                      <span className="hidden shrink-0 text-xs text-muted sm:block">
                        {section.categories.length}{" "}
                        {section.categories.length === 1 ? "categoría" : "categorías"} ·{" "}
                        {productCount} {productCount === 1 ? "producto" : "productos"}
                      </span>
                      <svg
                        width="20"
                        height="20"
                        viewBox="0 0 24 24"
                        fill="none"
                        aria-hidden="true"
                        className={`shrink-0 text-muted transition-transform ${isExpanded ? "rotate-180" : ""}`}
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
                    <div className="pb-4 pl-[4.25rem] pr-4 sm:pb-5 sm:pl-[4.5rem] sm:pr-5">
                      <button
                        type="button"
                        onClick={() =>
                          navigate({
                            id: "search-products",
                            query: "",
                            productGroup: section.id,
                          })
                        }
                        className="inline-flex items-center gap-1 rounded-lg border border-prime/40 bg-prime-muted px-3 py-2 text-xs font-semibold text-prime transition hover:border-prime hover:bg-prime/20 sm:px-3.5"
                        aria-label={`Ver todo ${section.title}`}
                      >
                        Todo {section.title}
                        <span aria-hidden="true">→</span>
                      </button>
                    </div>
                  </div>

                  {isExpanded && (
                    <div id={panelId} className="border-t border-bd p-3 sm:p-4">
                      {section.categories.length === 0 ? (
                        <p className="px-2 py-3 text-sm text-muted">
                          Todavía no hay productos disponibles en esta sección.
                        </p>
                      ) : (
                        <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                          {section.categories.map((category) => {
                            const openCategory = () =>
                              navigate({
                                id: "search-products",
                                query: category.query,
                                categoryContext: category.name,
                                categoryId: category.categoryId,
                              });

                            return (
                              <article
                                key={`${category.categoryId ?? category.query}:${category.name}`}
                                role="button"
                                tabIndex={0}
                                aria-label={`Ver categoría ${category.name}`}
                                onClick={openCategory}
                                onKeyDown={(event) => {
                                  if (event.key === "Enter" || event.key === " ") {
                                    event.preventDefault();
                                    openCategory();
                                  }
                                }}
                                className="group relative flex min-h-40 min-w-0 cursor-pointer overflow-hidden rounded-[1.5rem] border border-bd bg-surface text-left shadow-lg shadow-black/20 transition-all duration-300 hover:-translate-y-1 hover:border-prime/50 hover:shadow-2xl hover:shadow-black/30 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime"
                              >
                                <div className="absolute inset-y-0 left-0 w-[58%] bg-slate-950/60">
                                  <img
                                    src={category.image}
                                    alt=""
                                    loading="lazy"
                                    className="h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-[1.03]"
                                  />
                                  <span
                                    aria-hidden="true"
                                    className="pointer-events-none absolute inset-0 bg-gradient-to-r from-transparent via-transparent to-slate-950/25"
                                  />
                                </div>
                                <div
                                  aria-hidden="true"
                                  className="absolute inset-y-0 right-0 w-[66%]"
                                  style={{
                                    background:
                                      "linear-gradient(135deg, #343B40 0%, #22282C 42%, #191D20 100%)",
                                    clipPath: "polygon(18% 0, 100% 0, 100% 100%, 0 100%)",
                                  }}
                                />
                                <div className="relative z-10 ml-[38%] flex w-[62%] min-w-0 flex-col items-start justify-center gap-2 p-3 pl-8 sm:gap-2.5 sm:p-4 sm:pl-10">
                                  <h3 className="line-clamp-2 text-sm font-bold leading-snug text-text transition-colors group-hover:text-prime sm:text-base">
                                    {category.name}
                                  </h3>
                                  <span className="inline-flex items-center gap-1.5 rounded-full border border-bd bg-surface-2 px-2 py-1 text-[10px] font-medium text-muted">
                                    <span
                                      aria-hidden="true"
                                      className="h-1 w-1 rounded-full bg-prime"
                                    />
                                    {category.count}{" "}
                                    {category.count === 1 ? "producto" : "productos"}
                                  </span>
                                  <span className="mt-0.5 inline-flex h-9 w-9 items-center justify-center self-end rounded-full bg-prime text-[#181714] shadow-lg shadow-black/20 transition-all group-hover:-translate-y-0.5 group-hover:bg-prime-dark group-hover:shadow-xl">
                                    <svg
                                      aria-hidden="true"
                                      viewBox="0 0 20 20"
                                      fill="none"
                                      className="h-5 w-5"
                                    >
                                      <path
                                        d="M4.5 10h11m-4.5-4.5L15.5 10 11 14.5"
                                        stroke="currentColor"
                                        strokeWidth="1.7"
                                        strokeLinecap="round"
                                        strokeLinejoin="round"
                                      />
                                    </svg>
                                  </span>
                                </div>
                              </article>
                            );
                          })}
                        </div>
                      )}
                    </div>
                  )}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
