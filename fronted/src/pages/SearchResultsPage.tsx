import { useEffect, useMemo, useState, type ChangeEvent, type ReactNode } from "react";
import type { Page, Product } from "../types";
import { getProducts, getMinPrice } from "../Services/api/frontend-src/api";
import ProductCard from "../components/ProductCard";
import { Breadcrumb, EmptyState, Pagination } from "../components/ui";

interface Props {
  query: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";
type PriceRange = { min: number; max: number };

const PER_PAGE = 6;
const ACCENT = "#E8001B";

/**
 * Filtrado, orden y paginación de productos.
 * Vive en el mismo archivo pero separado del render: solo depende de
 * `products` (no importa de dónde vengan) y de `query`.
 */
function useProductSearchFilters(products: Product[], query: string, perPage = PER_PAGE) {
  const [sort, setSort] = useState<SortOption>("relevance");
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set());
  const [availableOnly, setAvailableOnly] = useState(false);
  const [priceRange, setPriceRangeState] = useState<PriceRange | null>(null);
  const [page, setPage] = useState(1);

  const brands = useMemo(() => [...new Set(products.map((p) => p.brand))], [products]);

  const priceBounds = useMemo<PriceRange>(() => {
    if (products.length === 0) return { min: 0, max: 0 };
    const prices = products.map((p) => getMinPrice(p));
    return { min: Math.min(...prices), max: Math.max(...prices) };
  }, [products]);

  // Cuando llega el catálogo, el rango de precio arranca en los límites reales.
  useEffect(() => {
    if (products.length > 0 && priceRange === null) {
      setPriceRangeState(priceBounds);
    }
  }, [products, priceBounds, priceRange]);

  const activeRange = priceRange ?? priceBounds;

  const filtered = useMemo(() => {
    const result = products.filter((p) => {
      if (
        query &&
        !p.name.toLowerCase().includes(query.toLowerCase()) &&
        !p.brand.toLowerCase().includes(query.toLowerCase()) &&
        !p.category.toLowerCase().includes(query.toLowerCase())
      )
        return false;

      const min = getMinPrice(p);
      if (min < activeRange.min || min > activeRange.max) return false;
      if (selectedBrands.size > 0 && !selectedBrands.has(p.brand)) return false;
      if (availableOnly && p.offers.every((o) => !o.available)) return false;
      return true;
    });

    return [...result].sort((a, b) => {
      if (sort === "price-asc") return getMinPrice(a) - getMinPrice(b);
      if (sort === "price-desc") return getMinPrice(b) - getMinPrice(a);
      if (sort === "rating") return b.rating - a.rating;
      return 0;
    });
  }, [products, query, activeRange, selectedBrands, availableOnly, sort]);

  const paginated = useMemo(
    () => filtered.slice((page - 1) * perPage, page * perPage),
    [filtered, page, perPage],
  );

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

  const setPriceRange = (range: PriceRange) => {
    setPriceRangeState(range);
    setPage(1);
  };

  const hasActiveFilters =
    selectedBrands.size > 0 ||
    availableOnly ||
    activeRange.min > priceBounds.min ||
    activeRange.max < priceBounds.max;

  const clearFilters = () => {
    setSelectedBrands(new Set());
    setAvailableOnly(false);
    setPriceRangeState(priceBounds);
    setPage(1);
  };

  return {
    sort,
    setSort,
    priceBounds,
    priceRange: activeRange,
    setPriceRange,
    selectedBrands,
    toggleBrand,
    availableOnly,
    setAvailableOnly,
    page,
    setPage,
    perPage,
    brands,
    filtered,
    paginated,
    hasActiveFilters,
    clearFilters,
  };
}

/** Sección de filtro que se puede colapsar, para mantener el panel limpio. */
function CollapsibleFilterSection({
  title,
  defaultOpen = true,
  children,
}: {
  title: string;
  defaultOpen?: boolean;
  children: ReactNode;
}) {
  const [open, setOpen] = useState(defaultOpen);

  return (
    <div className="border-b border-[#2A2A2A] pb-4 last:border-b-0 last:pb-0">
      <button
        type="button"
        onClick={() => setOpen((o) => !o)}
        className="w-full flex items-center justify-between text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3"
      >
        <span>{title}</span>
        <svg
          width="12"
          height="12"
          viewBox="0 0 24 24"
          fill="none"
          stroke="currentColor"
          strokeWidth="2"
          style={{ transition: "transform 0.15s ease", transform: open ? "rotate(180deg)" : "none" }}
        >
          <path d="m6 9 6 6 6-6" />
        </svg>
      </button>
      {open && children}
    </div>
  );
}

/** Slider de rango doble (min/máx) hecho con dos <input type="range"> nativos superpuestos. */
function PriceRangeSlider({
  bounds,
  value,
  onChange,
}: {
  bounds: PriceRange;
  value: PriceRange;
  onChange: (range: PriceRange) => void;
}) {
  const span = Math.max(bounds.max - bounds.min, 1);
  const minPercent = ((value.min - bounds.min) / span) * 100;
  const maxPercent = ((value.max - bounds.min) / span) * 100;
  const sliderMax = bounds.max > bounds.min ? bounds.max : bounds.min + 1;

  const handleMinChange = (e: ChangeEvent<HTMLInputElement>) => {
    const next = Math.min(Number(e.target.value), value.max - 1);
    onChange({ min: next, max: value.max });
  };

  const handleMaxChange = (e: ChangeEvent<HTMLInputElement>) => {
    const next = Math.max(Number(e.target.value), value.min + 1);
    onChange({ min: value.min, max: next });
  };

  return (
    <div className="pt-1">
      <div className="flex justify-between text-xs text-muted-2 mb-4">
        <span>${value.min.toLocaleString("es-CL")}</span>
        <span>${value.max.toLocaleString("es-CL")}</span>
      </div>

      <div className="relative h-4">
        <div
          className="absolute top-1/2 left-0 right-0 h-1 rounded-full -translate-y-1/2"
          style={{ background: "#2A2A2A" }}
        />
        <div
          className="absolute top-1/2 h-1 rounded-full -translate-y-1/2"
          style={{ background: ACCENT, left: `${minPercent}%`, right: `${100 - maxPercent}%` }}
        />
        <input
          aria-label="Precio mínimo"
          className="price-slider-thumb absolute top-1/2 left-0 w-full -translate-y-1/2"
          max={sliderMax}
          min={bounds.min}
          onChange={handleMinChange}
          type="range"
          value={value.min}
        />
        <input
          aria-label="Precio máximo"
          className="price-slider-thumb absolute top-1/2 left-0 w-full -translate-y-1/2"
          max={sliderMax}
          min={bounds.min}
          onChange={handleMaxChange}
          type="range"
          value={value.max}
        />
      </div>

      <style>{`
        .price-slider-thumb {
          -webkit-appearance: none;
          appearance: none;
          background: transparent;
          pointer-events: none;
          margin: 0;
        }
        .price-slider-thumb::-webkit-slider-thumb {
          -webkit-appearance: none;
          appearance: none;
          pointer-events: auto;
          width: 16px;
          height: 16px;
          border-radius: 9999px;
          background: ${ACCENT};
          border: 2px solid #0A0A0A;
          cursor: pointer;
        }
        .price-slider-thumb::-moz-range-thumb {
          pointer-events: auto;
          width: 16px;
          height: 16px;
          border-radius: 9999px;
          background: ${ACCENT};
          border: 2px solid #0A0A0A;
          cursor: pointer;
        }
        .price-slider-thumb::-webkit-slider-runnable-track {
          -webkit-appearance: none;
          background: transparent;
          height: 4px;
        }
        .price-slider-thumb::-moz-range-track {
          background: transparent;
          height: 4px;
        }
      `}</style>
    </div>
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
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [filtersOpen, setFiltersOpen] = useState(false);

  // Trae el catálogo completo: los filtros de precio/marca/disponibilidad y el
  // listado de marcas se calculan en el cliente sobre todo el catálogo.
  useEffect(() => {
    let cancelled = false;

    async function loadProducts() {
      setLoading(true);
      setError(null);
      try {
        const result = await getProducts();
        if (cancelled) return;
        setProducts(result);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Error al cargar los productos");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadProducts();

    return () => {
      cancelled = true;
    };
  }, []);

  const {
    sort,
    setSort,
    priceBounds,
    priceRange,
    setPriceRange,
    selectedBrands,
    toggleBrand,
    availableOnly,
    setAvailableOnly,
    page,
    setPage,
    perPage,
    brands,
    filtered,
    paginated,
    hasActiveFilters,
    clearFilters,
  } = useProductSearchFilters(products, query);

  const renderFilters = () => (
    <div>
      <CollapsibleFilterSection title="Disponibilidad">
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

      <div className="h-4" />

      <CollapsibleFilterSection title="Precio (CLP)">
        <PriceRangeSlider bounds={priceBounds} value={priceRange} onChange={setPriceRange} />
      </CollapsibleFilterSection>

      <div className="h-4" />

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

      {hasActiveFilters && (
        <button
          onClick={clearFilters}
          className="mt-5 text-xs text-prime hover:text-prime-dark transition-colors"
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
          <p className="text-sm text-muted mt-1">
            {loading ? "Cargando…" : `${filtered.length} productos encontrados`}
          </p>
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
            style={{ background: "#111111", border: "1px solid #2A2A2A", color: "#94A3B8" }}
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
              onClick={() => navigate({ id: "product-comparison", productIds: [...compareList] })}
              style={{ background: ACCENT, color: "#0A0A0A" }}
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
          {error ? (
            <EmptyState
              title="No pudimos cargar los productos"
              description={error}
              action={{ label: "Volver al inicio", onClick: () => navigate({ id: "home" }) }}
            />
          ) : loading ? (
            <div className="text-center py-20">
              <p className="text-muted">Cargando productos…</p>
            </div>
          ) : paginated.length === 0 ? (
            <EmptyState
              title="No encontramos resultados"
              description={`No hay productos que coincidan con "${query}". Intenta con otros términos o elimina algunos filtros.`}
              action={{ label: "Modificar búsqueda", onClick: () => navigate({ id: "home" }) }}
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
              <Pagination page={page} total={filtered.length} perPage={perPage} onChange={setPage} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}