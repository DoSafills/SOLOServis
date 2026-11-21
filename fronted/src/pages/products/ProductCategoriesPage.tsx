import { useEffect, useMemo, useState } from "react";
import type { Page, Product } from "../../types";
import { getAllProducts } from "../../services/api/api";
import { getCategories, type ApiProductCategory } from "../../services/api/products";
import { Breadcrumb, Skeleton, SkeletonGrid } from "../../components/common/ui";
import CategoryIcon, { type CategoryIconName } from "../../components/common/CategoryIcon";
import CategoryArtwork from "../../components/common/CategoryArtwork";
import { PRODUCT_CATEGORY_TREE } from "../../constants/categoryTree";

interface Props {
  navigate: (page: Page) => void;
}

const FALLBACK_SECTION_ID = "otras";

/** Descripciones de las secciones NV1; nombre e ícono salen del árbol NV. */
const SECTION_DESCRIPTIONS: Record<string, string> = {
  hogar: "Electrodomésticos, muebles y productos para tu casa",
  tecnologia: "TV, videojuegos, audio, computación y telefonía",
};

/** Sección que recibe los productos que no alcanzan ninguna NV3 del árbol. */
const FALLBACK_SECTION: { title: string; description: string; icon: CategoryIconName } = {
  title: "Otras categorías",
  description: "Deportes, fitness y nuevos tipos de productos",
  icon: "grid",
};

/**
 * Espejo del puente NV3 ↔ fila legacy de las migraciones 033–035: nombre
 * normalizado de la categoría legacy → slug de la NV3 que la absorbe.
 * Se usa para contar y filtrar mientras los productos no traigan
 * `categorySlug` (base sin el backfill de 033).
 */
const LEGACY_NAME_TO_NV3: Record<string, string> = {
  refrigeradores: "refrigeradores",
  lavadoras: "lavadoras",
  cocina: "cocina",
  climatizacion: "climatizacion",
  "limpieza del hogar": "limpieza-del-hogar",
  "cuidado personal": "cuidado-personal",
  "hogar y mueble": "muebles-y-decoracion",
  notebooks: "notebooks",
  "computadores de escritorio": "computadores-de-escritorio",
  "accesorios de computacion": "accesorios-de-computacion",
  televisores: "televisores",
  audio: "audio",
  celulares: "celulares",
  gaming: "gaming",
};

/** Palabras de la migración 033 (pasos 4–6) para dividir Computación en sus NV3. */
const COMPUTATION_ACCESSORY_WORDS = new Set([
  "mochila",
  "mochilas",
  "funda",
  "fundas",
  "maletin",
  "maletines",
  "soporte",
  "soportes",
  "base",
  "bases",
  "cargador",
  "cargadores",
  "adaptador",
  "adaptadores",
  "mouse",
  "teclado",
  "teclados",
  "monitor",
  "monitores",
  "tablet",
  "tablets",
  "ipad",
  "ipads",
]);
const COMPUTATION_NOTEBOOK_WORDS = new Set([
  "notebook",
  "notebooks",
  "laptop",
  "laptops",
  "macbook",
  "macbooks",
]);
const COMPUTATION_DESKTOP_WORDS = new Set([
  "imac",
  "imacs",
  "desktop",
  "desktops",
  "escritorio",
  "escritorios",
]);

/** Slugs NV3 declarados en el árbol; espejo de categories_nv3. */
const NV3_SLUGS = new Set(
  PRODUCT_CATEGORY_TREE.flatMap((section) =>
    section.children.flatMap((group) => group.children.map((leaf) => leaf.slug)),
  ),
);

/** Tarjeta NV3 (nivel 3): la categoría con la que se filtra la búsqueda. */
interface Nv3Card {
  slug: string;
  name: string;
  icon: CategoryIconName;
  count: number;
  /** Ids legacy de los productos de esta NV3, para el backend sin las NV tablas. */
  legacyIds: number[];
  /** true si algún producto ya trae este slug NV3 en `categorySlug`. */
  usesSlug: boolean;
}

/** Grupo NV2: agrupa las tarjetas NV3 dentro de una sección NV1. */
interface Nv2Group {
  slug: string;
  name: string;
  cards: Nv3Card[];
}

/** Sección NV1 (Hogar o Tecnología) con sus grupos y totales. */
interface CatalogSection {
  id: string;
  title: string;
  description: string;
  icon: CategoryIconName;
  groups: Nv2Group[];
  productCount: number;
  categoryCount: number;
}

/** Tarjeta de respaldo: categoría legacy sin NV3 asignada. */
interface LegacyCard {
  id: number | null;
  name: string;
  count: number;
}

interface Catalog {
  sections: CatalogSection[];
  legacyCards: LegacyCard[];
}

/**
 * Normaliza un nombre para compararlo sin tildes, espacios sobrantes ni mayúsculas.
 * @param value Nombre original.
 * @returns El nombre normalizado.
 */
const normalizeName = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .trim()
    .toLocaleLowerCase("es");

/**
 * Clasifica un producto de la categoría legacy "Computación" en su NV3 con
 * las mismas palabras clave que la migración 033: accesorios primero, luego
 * notebooks y al final escritorio.
 * @param name Nombre del producto.
 * @returns El slug NV3, o null si no se puede clasificar.
 */
const classifyComputation = (name: string): string | null => {
  const normalized = normalizeName(name).replace(/[^a-z0-9]+/g, " ");
  const words = normalized.split(" ").filter(Boolean);
  if (words.some((word) => COMPUTATION_ACCESSORY_WORDS.has(word))) {
    return "accesorios-de-computacion";
  }
  if (words.some((word) => COMPUTATION_NOTEBOOK_WORDS.has(word))) return "notebooks";
  if (
    words.some((word) => COMPUTATION_DESKTOP_WORDS.has(word)) ||
    normalized.includes(" pc gamer ")
  ) {
    return "computadores-de-escritorio";
  }
  return null;
};

/**
 * Resuelve la NV3 de un producto: primero el `categorySlug` que entrega el
 * backend y, si la base todavía no tiene el backfill, el puente legacy de
 * las migraciones 033–035 subiendo por los padres de su categoría.
 * @param product Producto del catálogo.
 * @param categoriesById Categorías legacy indexadas por id.
 * @returns El slug NV3, o null si el producto no cae en ninguna.
 */
const resolveProductNv3 = (
  product: Product,
  categoriesById: Map<number, ApiProductCategory>,
): string | null => {
  if (product.categorySlug && NV3_SLUGS.has(product.categorySlug)) return product.categorySlug;
  let current =
    product.categoryId === undefined ? undefined : categoriesById.get(product.categoryId);
  const visited = new Set<number>();
  while (current && !visited.has(current.id)) {
    visited.add(current.id);
    const name = normalizeName(current.name);
    if (name === "computacion") return classifyComputation(product.name);
    const nv3 = LEGACY_NAME_TO_NV3[name];
    if (nv3) return nv3;
    current = current.parentId === null ? undefined : categoriesById.get(current.parentId);
  }
  return null;
};

interface CatalogCardProps {
  name: string;
  icon: CategoryIconName;
  count: number;
  onOpen?: () => void;
}

/**
 * Tarjeta del catálogo con la misma presentación para NV3 y para las
 * categorías de respaldo. Solo es interactiva cuando recibe `onOpen`;
 * sin productos se muestra atenuada y sin acción.
 * @param props.name Nombre de la categoría.
 * @param props.icon Ícono declarado en el árbol o inferido por nombre.
 * @param props.count Cantidad de productos.
 * @param props.onOpen Abre la búsqueda filtrada; ausente si no hay productos.
 * @returns La tarjeta lista para el grid.
 */
function CatalogCard({ name, icon, count, onOpen }: CatalogCardProps) {
  return (
    <article
      role={onOpen ? "button" : undefined}
      tabIndex={onOpen ? 0 : undefined}
      aria-label={onOpen ? `Ver categoría ${name}` : `${name}: sin productos todavía`}
      onClick={onOpen}
      onKeyDown={(event) => {
        if (!onOpen) return;
        if (event.key === "Enter" || event.key === " ") {
          event.preventDefault();
          onOpen();
        }
      }}
      className={`group relative flex min-h-40 min-w-0 overflow-hidden rounded-[1.5rem] border bg-surface text-left shadow-lg shadow-black/20 transition-all duration-300 ${
        onOpen
          ? "cursor-pointer border-bd hover:-translate-y-1 hover:border-prime/50 hover:shadow-2xl hover:shadow-black/30 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime"
          : "border-dashed border-bd/70"
      }`}
    >
      <div className="theme-media-panel absolute inset-y-0 left-0 w-[58%]">
        <CategoryArtwork name={name} icon={icon} type="product" />
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
            "linear-gradient(135deg, var(--color-surface-3) 0%, var(--color-surface-2) 42%, var(--color-surface) 100%)",
          clipPath: "polygon(18% 0, 100% 0, 100% 100%, 0 100%)",
        }}
      />
      <div className="relative z-10 ml-[38%] flex w-[62%] min-w-0 flex-col items-start justify-center gap-2 p-3 pl-8 sm:gap-2.5 sm:p-4 sm:pl-10">
        <h3
          className={`line-clamp-2 text-sm font-bold leading-snug text-text transition-colors sm:text-base ${
            onOpen ? "group-hover:text-prime" : "opacity-80"
          }`}
        >
          {name}
        </h3>
        <span className="inline-flex items-center gap-1.5 rounded-full border border-bd bg-surface-2 px-2 py-1 text-[10px] font-medium text-muted">
          <span aria-hidden="true" className="h-1 w-1 rounded-full bg-prime" />
          {count === 0 ? "Sin productos" : `${count} ${count === 1 ? "producto" : "productos"}`}
        </span>
        {onOpen && (
          <span className="mt-0.5 inline-flex h-9 w-9 items-center justify-center self-end rounded-full bg-prime text-[#181714] shadow-lg shadow-black/20 transition-all group-hover:-translate-y-0.5 group-hover:bg-prime-dark group-hover:shadow-xl">
            <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" className="h-5 w-5">
              <path
                d="M4.5 10h11m-4.5-4.5L15.5 10 11 14.5"
                stroke="currentColor"
                strokeWidth="1.7"
                strokeLinecap="round"
                strokeLinejoin="round"
              />
            </svg>
          </span>
        )}
      </div>
    </article>
  );
}

/**
 * Página "Explora por categoría": muestra las categorías de nivel 3 del
 * árbol NV del backend (migraciones 032–035), agrupadas por su nivel 2 y
 * bajo Hogar y Tecnología. Los productos se cuentan por su `categorySlug`
 * NV3 y, si la base aún no tiene el backfill, por el puente legacy. Cada
 * tarjeta con productos filtra la búsqueda por el slug NV3 (o por los ids
 * legacy mientras el backend no resuelva esos slugs); los productos que no
 * alcanzan ninguna NV3 quedan en "Otras categorías".
 * @param props.navigate Cambia de página en la aplicación.
 * @returns La página de categorías de productos.
 */
export default function ProductCategoriesPage({ navigate }: Props) {
  const [products, setProducts] = useState<Product[]>([]);
  const [categories, setCategories] = useState<ApiProductCategory[]>([]);
  const [expandedSections, setExpandedSections] = useState<Set<string>>(() => new Set(["hogar"]));
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    Promise.all([getAllProducts(), getCategories()])
      .then(([productList, categoryList]) => {
        if (!cancelled) {
          setProducts(productList);
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

  const catalog = useMemo<Catalog>(() => {
    const categoriesById = new Map(categories.map((category) => [category.id, category]));

    const statsByNv3 = new Map<
      string,
      { count: number; legacyIds: Set<number>; usesSlug: boolean }
    >();
    const legacyById = new Map<number, LegacyCard>();
    let unclassified: LegacyCard | null = null;

    for (const product of products) {
      const nv3 = resolveProductNv3(product, categoriesById);
      if (nv3) {
        const stats = statsByNv3.get(nv3) ?? { count: 0, legacyIds: new Set(), usesSlug: false };
        stats.count += 1;
        if (product.categoryId !== undefined) stats.legacyIds.add(product.categoryId);
        if (product.categorySlug === nv3) stats.usesSlug = true;
        statsByNv3.set(nv3, stats);
        continue;
      }

      const name =
        categoriesById.get(product.categoryId ?? -1)?.name ||
        product.subcategory.trim() ||
        product.category.trim() ||
        "Otros productos";
      if (product.categoryId === undefined) {
        unclassified = { id: null, name, count: (unclassified?.count ?? 0) + 1 };
        continue;
      }
      const existing = legacyById.get(product.categoryId);
      legacyById.set(product.categoryId, {
        id: product.categoryId,
        name: existing?.name ?? name,
        count: (existing?.count ?? 0) + 1,
      });
    }

    const sections = PRODUCT_CATEGORY_TREE.flatMap<CatalogSection>((section) => {
      const description = SECTION_DESCRIPTIONS[section.slug];
      if (!description) return [];
      const groups = section.children.map<Nv2Group>((group) => ({
        slug: group.slug,
        name: group.name,
        cards: group.children.map<Nv3Card>((leaf) => {
          const stats = statsByNv3.get(leaf.slug);
          return {
            slug: leaf.slug,
            name: leaf.name,
            icon: leaf.icon,
            count: stats?.count ?? 0,
            legacyIds: [...(stats?.legacyIds ?? [])],
            usesSlug: stats?.usesSlug ?? false,
          };
        }),
      }));
      const cards = groups.flatMap((group) => group.cards);
      return [
        {
          id: section.slug,
          title: section.name,
          description,
          icon: section.icon,
          groups,
          productCount: cards.reduce((total, card) => total + card.count, 0),
          categoryCount: cards.length,
        },
      ];
    });

    const legacyCards = [...legacyById.values()].sort((first, second) =>
      first.name.localeCompare(second.name, "es"),
    );
    if (unclassified) legacyCards.push(unclassified);

    return { sections, legacyCards };
  }, [categories, products]);

  const totalProducts = products.length;

  /** Abre la búsqueda con todos los productos, sin filtros. */
  const showAllProducts = () => navigate({ id: "search-products", query: "" });

  /**
   * Abre la búsqueda filtrada por una categoría NV3. Usa el slug NV3 cuando
   * el backend ya lo resuelve para esos productos y, si no, los ids legacy
   * equivalentes, que todas las versiones de la Search API aceptan.
   * @param card Tarjeta NV3 seleccionada.
   */
  const openNv3Card = (card: Nv3Card) => {
    if (card.count === 0) return;
    navigate({
      id: "search-products",
      query: "",
      ...(card.usesSlug || card.legacyIds.length === 0
        ? { category: card.slug }
        : { categoryIds: card.legacyIds }),
      categoryContext: card.name,
      resetFilters: true,
    });
  };

  /**
   * Abre la búsqueda con todas las NV3 de una sección.
   * @param section Sección cuyo botón "Todo …" se pulsó.
   */
  const openSection = (section: CatalogSection) => {
    if (section.productCount === 0) return;
    const categoryIds = [
      ...new Set(section.groups.flatMap((group) => group.cards.flatMap((card) => card.legacyIds))),
    ];
    navigate({
      id: "search-products",
      query: "",
      ...(categoryIds.length > 0 ? { categoryIds } : { category: section.id }),
      categoryContext: section.title,
      resetFilters: true,
    });
  };

  /**
   * Abre la búsqueda por el id de una categoría legacy sin NV3.
   * @param card Tarjeta de "Otras categorías".
   */
  const openLegacyCard = (card: LegacyCard) => {
    if (card.id === null) return;
    navigate({
      id: "search-products",
      query: "",
      categoryId: card.id,
      categoryContext: card.name,
      resetFilters: true,
    });
  };

  /**
   * Abre o cierra una sección de la página.
   * @param sectionId Sección a alternar.
   */
  const toggleSection = (sectionId: string) => {
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
        <div role="status" aria-label="Cargando categorías" className="space-y-4">
          {[0, 1].map((section) => (
            <div key={section} className="rounded-xl border border-bd bg-surface p-4">
              <Skeleton className="mb-3 h-12 w-full" />
              <Skeleton className="mb-4 h-8 w-36" />
              <SkeletonGrid
                count={3}
                className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3"
              />
            </div>
          ))}
        </div>
      ) : error ? (
        <div className="py-8 text-center">
          <p className="mb-4 text-sm text-warn">{error}</p>
          <button onClick={showAllProducts} className="text-sm font-semibold text-prime">
            Ir a todos los productos
          </button>
        </div>
      ) : totalProducts === 0 ? (
        <div className="py-8 text-center">
          <p className="mb-4 text-sm text-muted">
            Todavía no hay productos disponibles para mostrar por categoría.
          </p>
          <button onClick={showAllProducts} className="text-sm font-semibold text-prime">
            Ver todos los productos
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {catalog.sections.map((section) => {
            const isExpanded = expandedSections.has(section.id);
            const panelId = `category-section-${section.id}`;

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
                        {section.categoryCount}{" "}
                        {section.categoryCount === 1 ? "categoría" : "categorías"} ·{" "}
                        {section.productCount}{" "}
                        {section.productCount === 1 ? "producto" : "productos"}
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
                    {section.productCount > 0 && (
                      <div className="pb-4 pl-[4.25rem] pr-4 sm:pb-5 sm:pl-[4.5rem] sm:pr-5">
                        <button
                          type="button"
                          onClick={() => openSection(section)}
                          className="inline-flex items-center gap-1 rounded-lg border border-prime/40 bg-prime-muted px-3 py-2 text-xs font-semibold text-prime transition hover:border-prime hover:bg-prime/20 sm:px-3.5"
                          aria-label={`Ver todo ${section.title}`}
                        >
                          Todo {section.title}
                          <span aria-hidden="true">→</span>
                        </button>
                      </div>
                    )}
                  </div>

                  {isExpanded && (
                    <div id={panelId} className="space-y-4 border-t border-bd p-3 sm:p-4">
                      {section.groups.map((group) => (
                        <div key={group.slug}>
                          <h3 className="mb-2 px-1 text-[10px] font-semibold uppercase tracking-[0.16em] text-muted">
                            {group.name}
                          </h3>
                          <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                            {group.cards.map((card) => (
                              <CatalogCard
                                key={card.slug}
                                name={card.name}
                                icon={card.icon}
                                count={card.count}
                                onOpen={card.count > 0 ? () => openNv3Card(card) : undefined}
                              />
                            ))}
                          </div>
                        </div>
                      ))}
                    </div>
                  )}
                </div>
              </section>
            );
          })}

          <section
            className={`category-outline relative isolate overflow-hidden rounded-xl bg-surface-3 p-px ${
              expandedSections.has(FALLBACK_SECTION_ID) ? "category-outline-active" : ""
            }`}
          >
            <div className="relative z-10 overflow-hidden rounded-[11px] bg-surface">
              <button
                type="button"
                onClick={() => toggleSection(FALLBACK_SECTION_ID)}
                aria-expanded={expandedSections.has(FALLBACK_SECTION_ID)}
                aria-controls={`category-section-${FALLBACK_SECTION_ID}`}
                aria-label={`${expandedSections.has(FALLBACK_SECTION_ID) ? "Ocultar" : "Ver"} subcategorías de ${FALLBACK_SECTION.title}`}
                className="flex min-w-0 w-full items-center gap-3 px-4 pt-4 pb-2 text-left sm:gap-4 sm:px-5 sm:pt-5"
              >
                <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-prime">
                  <CategoryIcon name={FALLBACK_SECTION.icon} />
                </span>
                <span className="min-w-0 flex-1">
                  <span className="block text-base font-semibold text-text sm:text-lg">
                    {FALLBACK_SECTION.title}
                  </span>
                  <span className="mt-1 block text-xs text-muted sm:text-sm">
                    {FALLBACK_SECTION.description}
                  </span>
                </span>
                <span className="hidden shrink-0 text-xs text-muted sm:block">
                  {catalog.legacyCards.length}{" "}
                  {catalog.legacyCards.length === 1 ? "categoría" : "categorías"} ·{" "}
                  {catalog.legacyCards.reduce((total, card) => total + card.count, 0)}{" "}
                  {catalog.legacyCards.reduce((total, card) => total + card.count, 0) === 1
                    ? "producto"
                    : "productos"}
                </span>
                <svg
                  width="20"
                  height="20"
                  viewBox="0 0 24 24"
                  fill="none"
                  aria-hidden="true"
                  className={`shrink-0 text-muted transition-transform ${
                    expandedSections.has(FALLBACK_SECTION_ID) ? "rotate-180" : ""
                  }`}
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

              {expandedSections.has(FALLBACK_SECTION_ID) && (
                <div
                  id={`category-section-${FALLBACK_SECTION_ID}`}
                  className="border-t border-bd p-3 sm:p-4"
                >
                  {catalog.legacyCards.length === 0 ? (
                    <p className="px-2 py-3 text-sm text-muted">
                      Todavía no hay productos disponibles en esta sección.
                    </p>
                  ) : (
                    <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                      {catalog.legacyCards.map((card) => (
                        <CatalogCard
                          key={card.id ?? card.name}
                          name={card.name}
                          icon="grid"
                          count={card.count}
                          onOpen={card.id === null ? undefined : () => openLegacyCard(card)}
                        />
                      ))}
                    </div>
                  )}
                </div>
              )}
            </div>
          </section>
        </div>
      )}
    </div>
  );
}
