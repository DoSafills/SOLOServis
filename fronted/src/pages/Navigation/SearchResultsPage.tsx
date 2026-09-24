import { useEffect, useState, type ReactNode } from "react";
import type { Page, Product } from "../../types";
import { getProducts, type ApiProduct } from "../../Services/api/products";
import { products as mockProducts } from "../../data/mockData";
import ProductCard from "../../components/ProductCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/ui";

interface Props {
  query: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";
type ExcludedType = "cpus" | "graphics" | "notebooks" | "computers";

const EXCLUDED_TYPE_LABELS: Record<ExcludedType, string> = {
  cpus: "CPUs",
  graphics: "Gráficas",
  notebooks: "Notebooks",
  computers: "Computadores",
};

function CollapsibleFilterSection({
  title,
  defaultOpen = false,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <section className="border-b border-[#2A2A2A] pb-4 last:border-b-0 last:pb-0">
      <button
        type="button"
        aria-expanded={open}
        onClick={() => setOpen((current) => !current)}
        className="w-full flex items-center justify-between text-left text-xs font-semibold text-muted-2 uppercase tracking-widest"
      >
        <span>{title}</span>
        <svg
          width="14"
          height="14"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          style={{
            transition: "transform 0.15s ease",
            transform: open ? "rotate(180deg)" : "none",
          }}
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      {open && <div className="mt-3">{children}</div>}
    </section>
  );
}

export default function SearchResultsPage({
  query,
  navigate,
  favorites,
  compareList,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [sort, setSort] = useState<SortOption>("relevance");
  const [priceMin, setPriceMin] = useState(0);
  const [priceMax, setPriceMax] = useState(1000000);
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set());
  const [selectedLines, setSelectedLines] = useState<Set<string>>(new Set());
  const [weightMin, setWeightMin] = useState(0);
  const [weightMax, setWeightMax] = useState(5000);
  const [excludedTypes, setExcludedTypes] = useState<Set<ExcludedType>>(new Set());
  const [availableOnly, setAvailableOnly] = useState(false);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(1);
  const PER_PAGE = 6;
  const [apiProducts, setApiProducts] = useState<ApiProduct[]>([]);

  useEffect(() => {
    getProducts().then(setApiProducts).catch(console.error);
  }, []);

  const apiProductItems: Product[] = apiProducts.map((product) => ({
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    category: product.category,
    subcategory: "",
    image: "",
    images: [],
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.model ? { Modelo: product.model } : { Modelo: "" },
    offers: [],
    priceHistory: [],
    offerPriceHistory: [],
    tags: [],
  }));

  const apiProductIds = new Set(apiProductItems.map((product) => product.id));
  const products: Product[] = [
    ...apiProductItems,
    ...mockProducts.filter((product) => !apiProductIds.has(product.id)),
  ];

  const getProductPrice = (product: Product) =>
    product.offerPrice ??
    product.offers.reduce(
      (lowest, offer) => Math.min(lowest, offer.price),
      product.offers.length ? Number.POSITIVE_INFINITY : 0,
    );

  const getProductWeight = (product: Product) => {
    const weightEntry = Object.entries(product.specs).find(([key]) =>
      /peso|weight/i.test(key),
    );
    const weight = weightEntry?.[1].match(/[\d.]+/)?.[0];
    return weight ? Number(weight) * (/[kK][gG]/.test(weightEntry[1]) ? 1000 : 1) : 0;
  };

  const getProductSearchText = (product: Product) =>
    [product.name, product.category, product.subcategory, ...product.tags]
      .join(" ")
      .toLowerCase();

  const brands = [...new Set(products.map((p) => p.brand))].sort();
  const lines = [...new Set(products.map((p) => p.subcategory).filter(Boolean))].sort();

let filtered = products.filter((p) => {
  if (
    query &&
    !p.name.toLowerCase().includes(query.toLowerCase()) &&
    !p.brand.toLowerCase().includes(query.toLowerCase()) &&
    !p.category.toLowerCase().includes(query.toLowerCase())
  ) {
    return false;
  }

  if (selectedBrands.size > 0 && !selectedBrands.has(p.brand)) {
    return false;
  }

  if (selectedLines.size > 0 && !selectedLines.has(p.subcategory)) {
    return false;
  }

  const productPrice = getProductPrice(p);
  if (productPrice < priceMin || productPrice > priceMax) {
    return false;
  }

  if (availableOnly && !p.offers.some((offer) => offer.available)) {
    return false;
  }

  const productWeight = getProductWeight(p);
  if (productWeight < weightMin || productWeight > weightMax) {
    return false;
  }

  const searchText = getProductSearchText(p);
  const excludedTerms: Record<ExcludedType, string[]> = {
    cpus: ["cpu", "procesador", "processor"],
    graphics: ["gráfica", "grafica", "gpu", "rtx", "radeon", "rx ", "graphics"],
    notebooks: ["notebook", "laptop"],
    computers: ["computador", "desktop", "pc de escritorio", "all-in-one"],
  };

  if (
    [...excludedTypes].some((type) =>
      excludedTerms[type].some((term) => searchText.includes(term)),
    )
  ) {
    return false;
  }

  return true;
});

filtered = [...filtered].sort((a, b) => {
  if (sort === "price-asc") return getProductPrice(a) - getProductPrice(b);
  if (sort === "price-desc") return getProductPrice(b) - getProductPrice(a);
  if (sort === "rating") return b.rating - a.rating;
  return 0;
});

const paginated = filtered.slice((page - 1) * PER_PAGE, page * PER_PAGE);

const toggleBrand = (brand: string) => {
  setSelectedBrands((prev) => {
    const next = new Set(prev);

    if (next.has(brand)) {
      next.delete(brand);
    } else {
      next.add(brand);
    }

    return next;
  });

  setPage(1);
};

  const toggleLine = (line: string) => {
    setSelectedLines((prev) => {
      const next = new Set(prev);
      if (next.has(line)) next.delete(line);
      else next.add(line);
      return next;
    });
    setPage(1);
  };

  const toggleExcludedType = (type: ExcludedType) => {
    setExcludedTypes((prev) => {
      const next = new Set(prev);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
    setPage(1);
  };


  const renderFilters = () => (
    <div className="space-y-4">
      <CollapsibleFilterSection title="General" defaultOpen>
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={availableOnly}
            onChange={(e) => setAvailableOnly(e.target.checked)}
            className="accent-prime"
          />
          <span className="text-sm text-muted-2">Solo disponibles</span>
        </label>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title="Línea">
        <div className="space-y-2">
          {lines.map((line) => (
            <label key={line} className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={selectedLines.has(line)}
                onChange={() => toggleLine(line)}
                className="accent-prime"
              />
              <span className="text-sm text-muted-2">{line}</span>
            </label>
          ))}
        </div>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title="Precio (CLP)">
        <div className="space-y-3">
          <label className="block text-xs text-muted-2">
            Desde ${priceMin.toLocaleString("es-CL")}
            <input
              type="range"
              min="0"
              max="1000000"
              step="10000"
              value={priceMin}
              onChange={(e) => setPriceMin(Math.min(Number(e.target.value), priceMax))}
              className="w-full accent-prime"
            />
          </label>
          <label className="block text-xs text-muted-2">
            Hasta ${priceMax.toLocaleString("es-CL")}
            <input
              type="range"
              min="0"
              max="1000000"
              step="10000"
              value={priceMax}
              onChange={(e) => setPriceMax(Math.max(Number(e.target.value), priceMin))}
              className="w-full accent-prime"
            />
          </label>
        </div>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title="Peso">
        <div className="space-y-3">
          <label className="block text-xs text-muted-2">
            Desde {weightMin.toLocaleString("es-CL")} g
            <input
              type="range"
              min="0"
              max="5000"
              step="100"
              value={weightMin}
              onChange={(e) => setWeightMin(Math.min(Number(e.target.value), weightMax))}
              className="w-full accent-prime"
            />
          </label>
          <label className="block text-xs text-muted-2">
            Hasta {weightMax.toLocaleString("es-CL")} g
            <input
              type="range"
              min="0"
              max="5000"
              step="100"
              value={weightMax}
              onChange={(e) => setWeightMax(Math.max(Number(e.target.value), weightMin))}
              className="w-full accent-prime"
            />
          </label>
        </div>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title="Excluir productos">
        <div className="space-y-2">
          {(Object.keys(EXCLUDED_TYPE_LABELS) as ExcludedType[]).map((type) => (
            <label key={type} className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={excludedTypes.has(type)}
                onChange={() => toggleExcludedType(type)}
                className="accent-prime"
              />
              <span className="text-sm text-muted-2">{EXCLUDED_TYPE_LABELS[type]}</span>
            </label>
          ))}
        </div>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title="Marca">
        <div className="space-y-2">
          {brands.map((brand) => (
            <label key={brand} className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={selectedBrands.has(brand)}
                onChange={() => toggleBrand(brand)}
                className="accent-prime"
              />
              <span className="text-sm text-muted-2">{brand}</span>
            </label>
          ))}
        </div>
      </CollapsibleFilterSection>

      {/* Clear */}
      {(selectedBrands.size > 0 ||
        selectedLines.size > 0 ||
        excludedTypes.size > 0 ||
        priceMin > 0 ||
        priceMax < 1000000 ||
        weightMin > 0 ||
        weightMax < 5000 ||
        availableOnly) && (
        <button
          onClick={() => {
            setSelectedBrands(new Set());
            setSelectedLines(new Set());
            setExcludedTypes(new Set());
            setPriceMin(0);
            setPriceMax(1000000);
            setWeightMin(0);
            setWeightMax(5000);
            setAvailableOnly(false);
          }}
          className="text-xs text-prime hover:text-prime-dark transition-colors"
        >
          Limpiar filtros
        </button>
      )}
    </div>
  );

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Productos" },
          ...(query ? [{ label: `"${query}"` }] : []),
        ]}
      />

      <div className="flex items-start justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-text">
            {query ? `Resultados para "${query}"` : "Todos los productos"}
          </h1>
          <p className="text-sm text-muted mt-1">{filtered.length} productos encontrados</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {/* Mobile filter button */}
          <button
            onClick={() => setFiltersOpen(true)}
            style={{ background: "#111111", border: "1px solid #2A2A2A" }}
            className="lg:hidden flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm text-muted-2 hover:border-prime hover:text-prime transition-all"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <line x1="4" y1="6" x2="11" y2="6" />
              <line x1="8" y1="6" x2="8" y2="2" />
              <line x1="8" y1="10" x2="8" y2="6" />
              <line x1="4" y1="18" x2="11" y2="18" />
              <line x1="8" y1="22" x2="8" y2="18" />
              <line x1="8" y1="14" x2="8" y2="18" />
              <line x1="13" y1="12" x2="20" y2="12" />
              <line x1="16" y1="8" x2="16" y2="12" />
              <line x1="16" y1="16" x2="16" y2="12" />
            </svg>
            Filtros
          </button>

          {/* Sort */}
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as SortOption)}
            style={{
              background: "#111111",
              border: "1px solid #2A2A2A",
              color: "#94A3B8",
            }}
            className="px-3 py-2 rounded-xl text-sm focus:outline-none focus:border-prime transition-colors"
          >
            <option value="relevance">Relevancia</option>
            <option value="price-asc">Precio: menor a mayor</option>
            <option value="price-desc">Precio: mayor a menor</option>
            <option value="rating">Mejor valoración</option>
          </select>

          {/* Compare button */}
          {compareList.size >= 2 && (
            <button
              onClick={() =>
                navigate({
                  id: "product-comparison",
                  productIds: [...compareList],
                })
              }
              style={{ background: "#E8001B", color: "#0A0A0A" }}
              className="px-4 py-2 rounded-xl text-sm font-semibold hover:opacity-90 transition-opacity"
            >
              Comparar {compareList.size} productos
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-6">
        {/* Sidebar filters (desktop) */}
        <aside
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="hidden lg:block w-56 shrink-0 rounded-2xl p-5 self-start sticky top-24"
        >
          <h3 className="text-sm font-semibold text-text mb-5">Filtros</h3>
          {renderFilters()}
        </aside>

        {/* Mobile filter modal */}
        {filtersOpen && (
          <div className="fixed inset-0 z-50 lg:hidden">
            <div
              className="absolute inset-0 bg-black/60 backdrop-blur-sm"
              onClick={() => setFiltersOpen(false)}
            />
            <div
              style={{ background: "#111111", borderLeft: "1px solid #2A2A2A" }}
              className="absolute right-0 top-0 bottom-0 w-72 p-6 overflow-y-auto"
            >
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-sm font-semibold text-text">Filtros</h3>
                <button
                  onClick={() => setFiltersOpen(false)}
                  className="text-muted hover:text-text transition-colors"
                >
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                  >
                    <path d="M18 6 6 18" />
                    <path d="m6 6 12 12" />
                  </svg>
                </button>
              </div>
              {renderFilters()}
            </div>
          </div>
        )}

        {/* Results */}
        <div className="flex-1 min-w-0">
          {paginated.length === 0 ? (
            <EmptyState
              title="No encontramos resultados"
              description={`No hay productos que coincidan con "${query}". Intenta con otros términos o elimina algunos filtros.`}
              action={{
                label: "Modificar búsqueda",
                onClick: () => navigate({ id: "home" }),
              }}
            />
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
                {paginated.map((p) => (
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
              <Pagination
                page={page}
                total={filtered.length}
                perPage={PER_PAGE}
                onChange={setPage}
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
