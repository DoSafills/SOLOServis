import { useState, type ReactNode } from "react";
import type { Page } from "../../types";
import ProductCard from "../../components/ProductCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/ui";
import {
  PRODUCT_TYPE_LABELS,
  useSearchResults,
  type AdvancedFilterGroupView,
  type ProductType,
  type SortOption,
} from "./useSearchResults";

interface Props {
  query: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

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

function SearchableAdvancedFilter({
  title,
  count,
  options,
  selectedValues,
  getOptionCount,
  onToggle,
}: {
  title: string;
  count: number;
  options: string[];
  selectedValues: Set<string>;
  getOptionCount: (option: string) => number;
  onToggle: (option: string) => void;
}) {
  const [search, setSearch] = useState("");
  const visibleOptions = options.filter((option) =>
    option.toLowerCase().includes(search.trim().toLowerCase()),
  );

  return (
    <CollapsibleFilterSection title={`${title} (${count})`} defaultOpen>
      <div className="space-y-2">
        <input
          type="search"
          value={search}
          onChange={(event) => setSearch(event.target.value)}
          placeholder={`Buscar ${title.toLowerCase()}...`}
          aria-label={`Buscar ${title.toLowerCase()}`}
          disabled={options.length === 0}
          className="w-full rounded-lg border border-[#2A2A2A] bg-[#0A0A0A] px-3 py-2 text-sm text-text placeholder:text-muted-2 focus:border-prime focus:outline-none disabled:opacity-50"
        />
        {visibleOptions.length === 0 ? (
          <p className="text-xs text-muted-2">
            {options.length === 0
              ? "No hay especificaciones disponibles para esta categoría."
              : "No hay coincidencias."}
          </p>
        ) : (
          <div className="max-h-40 space-y-2 overflow-y-auto">
            {visibleOptions.map((option) => (
              <label key={option} className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={selectedValues.has(option)}
                  onChange={() => onToggle(option)}
                  className="accent-prime"
                />
                <span className="flex flex-1 items-center justify-between gap-2 break-words text-sm text-muted-2">
                  <span>{option}</span>
                  <span className="shrink-0 text-xs">{getOptionCount(option)}</span>
                </span>
              </label>
            ))}
          </div>
        )}
      </div>
    </CollapsibleFilterSection>
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
  const {
    sort,
    setSort,
    priceMin,
    setPriceMin,
    priceMax,
    setPriceMax,
    weightMin,
    setWeightMin,
    weightMax,
    setWeightMax,
    selectedBrands,
    selectedTypes,
    availableOnly,
    setAvailableOnly,
    brands,
    brandCounts,
    typeCounts,
    productTypeCount,
    priceCount,
    priceLimit,
    weightCount,
    availableCount,
    brandCount,
    selectedFilterCount,
    filtered,
    paginated,
    page,
    setPage,
    PER_PAGE,
    toggleBrand,
    toggleProductType,
    advancedFilterGroups,
    clearFilters,
    shouldShowClearFilters,
  } = useSearchResults(query);
  const [filterPanelExpanded, setFilterPanelExpanded] = useState(false);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const renderAdvancedFilters = (group: AdvancedFilterGroupView) => (
    <CollapsibleFilterSection
      key={group.type}
      title={`${group.title}: filtros avanzados`}
      defaultOpen
    >
      <div className="space-y-4">
        {group.filters.map((filter) => (
          <SearchableAdvancedFilter
            key={filter.id}
            title={filter.title}
            count={filter.count}
            options={filter.options}
            selectedValues={filter.selectedValues}
            getOptionCount={filter.getOptionCount}
            onToggle={filter.onToggle}
          />
        ))}
      </div>
    </CollapsibleFilterSection>
  );


  const renderFilters = () => (
    <div className="space-y-4">
      <CollapsibleFilterSection title={`Precio (CLP) (${priceCount})`}>
        <div className="space-y-3">
          <label className="block text-xs text-muted-2">
            Desde ${priceMin.toLocaleString("es-CL")}
            <input
              type="range"
              min="0"
              max={priceLimit}
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
              max={priceLimit}
              step="10000"
              value={priceMax}
              onChange={(e) => setPriceMax(Math.max(Number(e.target.value), priceMin))}
              className="w-full accent-prime"
            />
          </label>
        </div>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title={`Peso (kg) (${weightCount})`}>
        <div className="space-y-3">
          <label className="block text-xs text-muted-2">
            Desde {weightMin.toLocaleString("es-CL")} kg
            <input
              type="range"
              min="0"
              max="50"
              step="0.5"
              value={weightMin}
              onChange={(e) => setWeightMin(Math.min(Number(e.target.value), weightMax))}
              className="w-full accent-prime"
            />
          </label>
          <label className="block text-xs text-muted-2">
            Hasta {weightMax.toLocaleString("es-CL")} kg
            <input
              type="range"
              min="0"
              max="50"
              step="0.5"
              value={weightMax}
              onChange={(e) => setWeightMax(Math.max(Number(e.target.value), weightMin))}
              className="w-full accent-prime"
            />
          </label>
        </div>
      </CollapsibleFilterSection>

      <CollapsibleFilterSection title={`General (${availableCount})`} defaultOpen>
        <div className="space-y-4">
          <label className="flex items-center gap-2 cursor-pointer">
            <input
              type="checkbox"
              checked={availableOnly}
              onChange={(e) => setAvailableOnly(e.target.checked)}
              className="accent-prime"
            />
            <span className="flex flex-1 items-center justify-between text-sm text-muted-2">
              Solo disponibles
              <span className="text-xs">{availableCount}</span>
            </span>
          </label>
          <div className="space-y-2">
            <p className="text-xs font-semibold uppercase tracking-widest text-muted-2">
              Tipo de producto ({productTypeCount})
            </p>
            {(Object.keys(PRODUCT_TYPE_LABELS) as ProductType[]).map((type) => (
              <label key={type} className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={selectedTypes.has(type)}
                  onChange={() => toggleProductType(type)}
                  className="accent-prime"
                />
                <span className="flex flex-1 items-center justify-between text-sm text-muted-2">
                  {PRODUCT_TYPE_LABELS[type]}
                  <span className="text-xs">{typeCounts[type]}</span>
                </span>
              </label>
            ))}
          </div>
        </div>
      </CollapsibleFilterSection>

      {advancedFilterGroups.map((group) => renderAdvancedFilters(group))}

      <CollapsibleFilterSection title={`Marca (${brandCount})`}>
        <div className="space-y-2">
          {brands.map((brand) => (
            <label key={brand} className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={selectedBrands.has(brand)}
                onChange={() => toggleBrand(brand)}
                className="accent-prime"
              />
              <span className="flex flex-1 items-center justify-between text-sm text-muted-2">
                {brand}
                <span className="text-xs">{brandCounts.get(brand) ?? 0}</span>
              </span>
            </label>
          ))}
        </div>
      </CollapsibleFilterSection>

      {/* Clear */}
      {shouldShowClearFilters && (
        <button
          onClick={clearFilters}
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
            <span className="rounded-full bg-[#2A2A2A] px-2 py-0.5 text-xs text-muted-2">
              {selectedFilterCount}
            </span>
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
          <button
            type="button"
            aria-expanded={filterPanelExpanded}
            onClick={() => setFilterPanelExpanded((current) => !current)}
            className="w-full flex items-center justify-between text-left text-sm font-semibold text-text"
          >
            <span className="flex items-center gap-2">
              Filtros
              <span className="rounded-full bg-[#2A2A2A] px-2 py-0.5 text-xs text-muted-2">
                {selectedFilterCount}
              </span>
            </span>
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              style={{
                transition: "transform 0.15s ease",
                transform: filterPanelExpanded ? "rotate(180deg)" : "none",
              }}
            >
              <path d="m6 9 6 6 6-6" />
            </svg>
          </button>
          {filterPanelExpanded && <div className="mt-5">{renderFilters()}</div>}
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
                <button
                  type="button"
                  aria-expanded={filterPanelExpanded}
                  onClick={() => setFilterPanelExpanded((current) => !current)}
                  className="flex items-center gap-2 text-sm font-semibold text-text"
                >
                  <span className="flex items-center gap-2">
                    Filtros
                    <span className="rounded-full bg-[#2A2A2A] px-2 py-0.5 text-xs text-muted-2">
                      {selectedFilterCount}
                    </span>
                  </span>
                  <svg
                    width="14"
                    height="14"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    style={{
                      transition: "transform 0.15s ease",
                      transform: filterPanelExpanded ? "rotate(180deg)" : "none",
                    }}
                  >
                    <path d="m6 9 6 6 6-6" />
                  </svg>
                </button>
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
              {filterPanelExpanded && renderFilters()}
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
