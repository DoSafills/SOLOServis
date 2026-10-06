import { useEffect, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import { getProducts } from "../../services/api/api";
import ProductCard from "../../components/products/ProductCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/common/ui";
import {
  getProductSubtype,
  PRODUCT_TYPES,
  productMatchesSubtype,
  productMatchesType,
} from "../../constants/productCatalog";

interface Props {
  query: string;
  category?: string;
  productGroup?: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  productCompareList: Set<string>;
  cartProductIds: Set<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => void;
  onToggleFavorite: (id: string) => void;
  onToggleProductCompare: (id: string, category: string) => void;
}

type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";

interface ProductFilterState {
  priceMin: string;
  priceMax: string;
  selectedBrands: Set<string>;
  selectedSpecs: Record<string, Set<string>>;
  availableOnly: boolean;
}

const emptyFilters: ProductFilterState = {
  priceMin: "",
  priceMax: "",
  selectedBrands: new Set(),
  selectedSpecs: {},
  availableOnly: false,
};

const specFilterGroups = [
  {
    title: "Pantalla e imagen",
    pattern: /(pantalla|pulgad|resoluci[oó]n|panel|brillo|imagen|display|oled|led)/i,
  },
  {
    title: "Memoria y almacenamiento",
    pattern: /(ram|memoria|almacenamiento|ssd|disco|rom|vram)/i,
  },
  { title: "Capacidad y carga", pattern: /(capacidad|volumen|carga|soporte)/i },
  {
    title: "Peso y dimensiones",
    pattern: /(peso|masa|dimensi[oó]n|ancho|alto|largo|profundidad|di[aá]metro|superficie)/i,
  },
  { title: "Rendimiento", pattern: /(procesador|cpu|n[uú]cleos|cuda|velocidad|rpm|rendimiento)/i },
  {
    title: "Consumo y energía",
    pattern: /(consumo|energ[ií]a|eficiencia|potencia|tensi[oó]n|voltaje|watt|kwh)/i,
  },
  {
    title: "Conectividad y funciones",
    pattern: /(wifi|bluetooth|conect|sistema operativo|smart|asistente|voz|aplicaci[oó]n)/i,
  },
  { title: "Materiales y acabado", pattern: /(material|acabado|color)/i },
  { title: "Otros", pattern: /.*/ },
];

const normalizeFilterValue = (value: string): string => value.trim().toLocaleLowerCase();

const getProductPrice = (product: Product): number | null => {
  const prices = product.offers.map((offer) => offer.price).filter(Number.isFinite);
  return prices.length ? Math.min(...prices) : null;
};

const getSpecFilterGroups = (products: Product[]) => {
  const names = [...new Set(products.flatMap((product) => Object.keys(product.specs)))].sort(
    (first, second) => first.localeCompare(second, "es"),
  );
  const assigned = new Set<string>();

  return specFilterGroups
    .map(({ title, pattern }) => {
      const specs = names
        .filter((name) => {
          if (assigned.has(name) || !pattern.test(name)) return false;
          assigned.add(name);
          return true;
        })
        .map((name) => ({
          name,
          values: [
            ...new Set(
              products
                .map((product) => product.specs[name]?.trim())
                .filter((value): value is string => Boolean(value)),
            ),
          ].sort((first, second) => first.localeCompare(second, "es", { numeric: true })),
        }))
        .filter((spec) => spec.values.length > 0);

      return { title, specs };
    })
    .filter((group) => group.specs.length > 0);
};

export default function SearchResultsPage({
  query,
  category,
  productGroup,
  navigate,
  favorites,
  productCompareList,
  cartProductIds,
  onAddToCart,
  onToggleFavorite,
  onToggleProductCompare,
}: Props) {
  const [sort, setSort] = useState<SortOption>("relevance");
  const [activeProductGroup, setActiveProductGroup] = useState(productGroup ?? "all");
  const [activeSubtype, setActiveSubtype] = useState("all");
  const [subtypesByGroup, setSubtypesByGroup] = useState<Record<string, string>>({
    [productGroup ?? "all"]: "all",
  });
  const [filtersByGroup, setFiltersByGroup] = useState<Record<string, ProductFilterState>>({});
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(1);
  const PER_PAGE = 6;
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const activeFilterKey =
    activeSubtype === "all" ? activeProductGroup : `${activeProductGroup}:${activeSubtype}`;
  const activeFilters = filtersByGroup[activeFilterKey] ?? emptyFilters;
  const { priceMin, priceMax, selectedBrands, selectedSpecs, availableOnly } = activeFilters;

  const updateActiveFilters = (update: Partial<ProductFilterState>) => {
    setFiltersByGroup((previous) => ({
      ...previous,
      [activeFilterKey]: { ...(previous[activeFilterKey] ?? emptyFilters), ...update },
    }));
    setPage(1);
  };

  const updateActiveFiltersWith = (update: (current: ProductFilterState) => ProductFilterState) => {
    setFiltersByGroup((previous) => ({
      ...previous,
      [activeFilterKey]: update(previous[activeFilterKey] ?? emptyFilters),
    }));
    setPage(1);
  };

  useEffect(() => {
    let cancelled = false;

    async function loadProducts() {
      setLoading(true);
      setError(null);

      try {
        const result = await getProducts();
        if (!cancelled) {
          setProducts(result);
        }
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Error al cargar los productos");
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    }

    loadProducts();

    return () => {
      cancelled = true;
    };
  }, []);

  const departmentProducts = products.filter((product) => {
    if (category && product.category.toLocaleLowerCase() !== category.toLocaleLowerCase()) {
      return false;
    }
    if (!productMatchesType(product, activeProductGroup)) return false;

    return (
      !query ||
      product.name.toLowerCase().includes(query.toLowerCase()) ||
      product.brand.toLowerCase().includes(query.toLowerCase()) ||
      product.category.toLowerCase().includes(query.toLowerCase())
    );
  });
  const availableSubtypes = [...new Set(departmentProducts.map(getProductSubtype))].sort((a, b) =>
    a.localeCompare(b, "es"),
  );
  const matchingProducts = departmentProducts.filter(
    (product) => activeSubtype === "all" || productMatchesSubtype(product, activeSubtype),
  );

  const brands = [...new Set(matchingProducts.map((product) => product.brand))].sort((a, b) =>
    a.localeCompare(b, "es"),
  );
  const specGroups = getSpecFilterGroups(matchingProducts);
  const minimumPrice = priceMin ? Number(priceMin) : null;
  const maximumPrice = priceMax ? Number(priceMax) : null;

  let filtered = matchingProducts.filter((product) => {
    if (selectedBrands.size > 0 && !selectedBrands.has(product.brand)) return false;
    if (availableOnly && !product.offers.some((offer) => offer.available)) return false;

    const productPrice = getProductPrice(product);
    if (minimumPrice !== null && Number.isFinite(minimumPrice)) {
      if (productPrice === null || productPrice < minimumPrice) return false;
    }
    if (maximumPrice !== null && Number.isFinite(maximumPrice)) {
      if (productPrice === null || productPrice > maximumPrice) return false;
    }

    return Object.entries(selectedSpecs).every(([name, values]) => {
      if (values.size === 0) return true;
      const productValue = product.specs[name];
      return productValue ? values.has(normalizeFilterValue(productValue)) : false;
    });
  });

  filtered = [...filtered].sort((a, b) => {
    if (sort === "rating") return b.rating - a.rating;
    if (sort === "price-asc")
      return (getProductPrice(a) ?? Infinity) - (getProductPrice(b) ?? Infinity);
    if (sort === "price-desc")
      return (getProductPrice(b) ?? -Infinity) - (getProductPrice(a) ?? -Infinity);
    return 0;
  });

  const paginated = filtered.slice((page - 1) * PER_PAGE, page * PER_PAGE);

  const toggleBrand = (brand: string) => {
    updateActiveFiltersWith((current) => {
      const selectedBrands = new Set(current.selectedBrands);
      if (selectedBrands.has(brand)) selectedBrands.delete(brand);
      else selectedBrands.add(brand);
      return { ...current, selectedBrands };
    });
  };

  const toggleSpecValue = (name: string, value: string) => {
    const normalizedValue = normalizeFilterValue(value);
    updateActiveFiltersWith((current) => {
      const selectedSpecs = { ...current.selectedSpecs };
      const values = new Set(selectedSpecs[name] ?? []);

      if (values.has(normalizedValue)) values.delete(normalizedValue);
      else values.add(normalizedValue);

      if (values.size === 0) delete selectedSpecs[name];
      else selectedSpecs[name] = values;
      return { ...current, selectedSpecs };
    });
  };

  const clearFilters = () => {
    updateActiveFilters({ ...emptyFilters });
  };

  const renderFilters = () => (
    <div className="space-y-2">
      <details className="border-b border-slate-200 pb-2" open>
        <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
          Precio <span className="float-right text-slate-500">⌄</span>
        </summary>
        <div className="flex gap-2 pb-2 pt-1">
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Mínimo
            <input
              type="number"
              min="0"
              value={priceMin}
              onChange={(event) => {
                updateActiveFilters({ priceMin: event.target.value });
              }}
              placeholder="$0"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 placeholder:text-slate-500 focus:border-violet-500 focus:outline-none"
            />
          </label>
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Máximo
            <input
              type="number"
              min="0"
              value={priceMax}
              onChange={(event) => {
                updateActiveFilters({ priceMax: event.target.value });
              }}
              placeholder="Sin límite"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 placeholder:text-slate-500 focus:border-violet-500 focus:outline-none"
            />
          </label>
        </div>
      </details>

      <details className="border-b border-slate-200 pb-2">
        <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
          Marca <span className="float-right text-slate-500">⌄</span>
        </summary>
        <div className="max-h-48 space-y-2 overflow-y-auto pb-2 pt-1">
          {brands.map((brand) => (
            <label
              key={brand}
              className="flex cursor-pointer items-center gap-2 text-sm text-slate-700"
            >
              <input
                type="checkbox"
                checked={selectedBrands.has(brand)}
                onChange={() => toggleBrand(brand)}
                className="accent-prime"
              />
              <span>{brand}</span>
            </label>
          ))}
        </div>
      </details>

      <details className="border-b border-slate-200 pb-2">
        <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
          Disponibilidad <span className="float-right text-slate-500">⌄</span>
        </summary>
        <label className="flex cursor-pointer items-center gap-2 pb-2 pt-1 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={availableOnly}
            onChange={(event) => {
              updateActiveFilters({ availableOnly: event.target.checked });
            }}
            className="accent-prime"
          />
          Solo disponibles
        </label>
      </details>

      {specGroups.map((group) => (
        <details key={group.title} className="border-b border-slate-200 pb-2">
          <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
            {group.title} <span className="float-right text-slate-500">⌄</span>
          </summary>
          <div className="max-h-64 space-y-3 overflow-y-auto pb-2 pt-1">
            {group.specs.map((spec) => (
              <fieldset key={spec.name}>
                <legend className="mb-1.5 text-xs font-semibold text-slate-600">{spec.name}</legend>
                <div className="space-y-1.5">
                  {spec.values.map((value) => (
                    <label
                      key={value}
                      className="flex cursor-pointer items-start gap-2 text-xs text-slate-700"
                    >
                      <input
                        type="checkbox"
                        checked={
                          selectedSpecs[spec.name]?.has(normalizeFilterValue(value)) ?? false
                        }
                        onChange={() => toggleSpecValue(spec.name, value)}
                        className="mt-0.5 accent-prime"
                      />
                      <span>{value}</span>
                    </label>
                  ))}
                </div>
              </fieldset>
            ))}
          </div>
        </details>
      ))}

      {(selectedBrands.size > 0 ||
        Object.keys(selectedSpecs).length > 0 ||
        priceMin ||
        priceMax ||
        availableOnly) && (
        <button
          onClick={clearFilters}
          className="pt-2 text-xs font-semibold text-violet-700 hover:text-violet-900"
        >
          Limpiar filtros
        </button>
      )}
    </div>
  );

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando productos...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn mb-4">{error}</p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "#E8001B", color: "#0A0A0A" }}
          className="px-5 py-2 rounded-xl text-sm font-semibold"
        >
          Volver a productos
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Productos",
            onClick: () => navigate({ id: "product-categories" }),
          },
          ...(category ? [{ label: category }] : []),
          ...(query ? [{ label: `"${query}"` }] : []),
        ]}
      />

      {!category && (
        <nav
          aria-label="Tipos de productos"
          role="tablist"
          className="mb-6 flex gap-2 overflow-x-auto border-b border-white/10 pb-3"
        >
          {[{ id: "all", name: "Todos" }, ...PRODUCT_TYPES].map((type) => {
            const isActive = activeProductGroup === type.id;
            const selectedSubtype = subtypesByGroup[type.id] ?? "all";
            const groupFilterKey =
              selectedSubtype === "all" ? type.id : `${type.id}:${selectedSubtype}`;
            const groupFilters = filtersByGroup[groupFilterKey] ?? emptyFilters;
            const activeCount =
              groupFilters.selectedBrands.size +
              Object.values(groupFilters.selectedSpecs).reduce(
                (count, values) => count + values.size,
                0,
              ) +
              Number(Boolean(groupFilters.priceMin || groupFilters.priceMax)) +
              Number(groupFilters.availableOnly);

            return (
              <button
                key={type.id}
                type="button"
                role="tab"
                aria-selected={isActive}
                onClick={() => {
                  setActiveProductGroup(type.id);
                  const nextSubtype = subtypesByGroup[type.id] ?? "all";
                  setActiveSubtype(nextSubtype);
                  setPage(1);
                }}
                className={`inline-flex shrink-0 items-center gap-2 rounded-t-lg border-b-2 px-3 py-2 text-sm font-semibold transition-colors ${
                  isActive
                    ? "border-prime text-prime"
                    : "border-transparent text-muted-2 hover:text-text"
                }`}
              >
                {type.name}
                {activeCount > 0 && (
                  <span className="rounded-full bg-prime/15 px-1.5 py-0.5 text-[10px] text-prime">
                    {activeCount}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
      )}

      {!category && availableSubtypes.length > 0 && (
        <nav
          aria-label="Tipos específicos de productos"
          role="tablist"
          className="mb-6 flex gap-2 overflow-x-auto pb-1"
        >
          {["Todos los tipos", ...availableSubtypes].map((subtype, index) => {
            const subtypeId = index === 0 ? "all" : subtype;
            const isActive = activeSubtype === subtypeId;
            const subtypeFilterKey =
              subtypeId === "all" ? activeProductGroup : `${activeProductGroup}:${subtypeId}`;
            const subtypeFilters = filtersByGroup[subtypeFilterKey] ?? emptyFilters;
            const activeCount =
              subtypeFilters.selectedBrands.size +
              Object.values(subtypeFilters.selectedSpecs).reduce(
                (count, values) => count + values.size,
                0,
              ) +
              Number(Boolean(subtypeFilters.priceMin || subtypeFilters.priceMax)) +
              Number(subtypeFilters.availableOnly);

            return (
              <button
                key={subtypeId}
                type="button"
                role="tab"
                aria-selected={isActive}
                onClick={() => {
                  setActiveSubtype(subtypeId);
                  setSubtypesByGroup((previous) => ({
                    ...previous,
                    [activeProductGroup]: subtypeId,
                  }));
                  setPage(1);
                }}
                className={`inline-flex shrink-0 items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors ${
                  isActive
                    ? "border-prime bg-prime/10 text-prime"
                    : "border-white/10 text-muted-2 hover:border-white/30 hover:text-text"
                }`}
              >
                {subtype}
                {activeCount > 0 && <span className="text-[10px]">{activeCount}</span>}
              </button>
            );
          })}
        </nav>
      )}

      <div className="flex items-start justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-text">
            {category ||
              (activeSubtype !== "all"
                ? activeSubtype
                : activeProductGroup !== "all"
                  ? `${PRODUCT_TYPES.find((type) => type.id === activeProductGroup)?.name ?? "Productos"}${query ? `: ${query}` : ""}`
                  : query
                    ? `Resultados para "${query}"`
                    : "Todos los productos")}
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
          {productCompareList.size >= 2 && (
            <button
              onClick={() =>
                navigate({
                  id: "product-comparison",
                  productIds: [...productCompareList],
                })
              }
              style={{ background: "#E8001B", color: "#0A0A0A" }}
              className="px-4 py-2 rounded-xl text-sm font-semibold hover:opacity-90 transition-opacity"
            >
              Comparar {productCompareList.size} productos
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-6">
        {/* Sidebar filters (desktop) */}
        <aside className="hidden lg:block w-56 shrink-0 rounded-2xl border border-slate-200 bg-slate-50 p-5 self-start sticky top-24 shadow-sm">
          <h3 className="text-sm font-semibold text-slate-900 mb-5">Filtros</h3>
          {renderFilters()}
        </aside>

        {/* Mobile filter modal */}
        {filtersOpen && (
          <div className="fixed inset-0 z-50 lg:hidden">
            <div
              className="absolute inset-0 bg-black/60 backdrop-blur-sm"
              onClick={() => setFiltersOpen(false)}
            />
            <div className="absolute right-0 top-0 bottom-0 w-72 border-l border-slate-200 bg-slate-50 p-6 overflow-y-auto shadow-2xl">
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-sm font-semibold text-slate-900">Filtros</h3>
                <button
                  onClick={() => setFiltersOpen(false)}
                  className="text-slate-500 hover:text-slate-900 transition-colors"
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
                    isComparing={productCompareList.has(p.id)}
                    isInCart={cartProductIds.has(p.id)}
                    onAddToCart={onAddToCart}
                    onToggleFavorite={onToggleFavorite}
                    onToggleCompare={onToggleProductCompare}
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
