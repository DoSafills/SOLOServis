import {
  useEffect,
  useState,
  type ReactNode,
} from "react";
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
type ProductType = "cpus" | "graphics" | "notebooks" | "computers" | "smartphones";
type FilterFacet = string;
interface AdvancedFilterDefinition {
  id: string;
  title: string;
  keyPattern: RegExp;
}

const PRICE_LIMIT = 40000000;
const PROCESSOR_KEY = /procesador|processor|cpu/i;
const RAM_KEY = /ram|memoria/i;
const STORAGE_KEY = /almacenamiento|storage|disco/i;
const GRAPHICS_KEY = /gpu|gráfica|graphics|video/i;
const DISPLAY_KEY = /pantalla|display|screen/i;
const BATTERY_KEY = /batería|battery/i;
const CAMERA_KEY = /cámara|camera/i;

const PRODUCT_TYPE_LABELS: Record<ProductType, string> = {
  cpus: "CPUs",
  graphics: "Gráficas",
  notebooks: "Notebooks",
  computers: "Computadores",
  smartphones: "Smartphones",
};

const PRODUCT_TYPE_TERMS: Record<ProductType, string[]> = {
  cpus: ["cpu", "procesador", "procesadores", "processor"],
  graphics: ["gráfica", "grafica", "gpu", "rtx", "radeon", "rx ", "graphics"],
  notebooks: ["notebook", "laptop"],
  computers: ["computador", "computadora", "computadores", "desktop", "pc de escritorio", "all-in-one", "torre"],
  smartphones: ["smartphone", "celular", "teléfono", "telefono", "móvil", "movil"],
};

const ADVANCED_FILTERS: Record<ProductType, AdvancedFilterDefinition[]> = {
  cpus: [
    { id: "socket", title: "Socket", keyPattern: /socket/i },
    { id: "cores", title: "Núcleos", keyPattern: /núcleos|cores/i },
    { id: "threads", title: "Hilos", keyPattern: /hilos|threads/i },
    { id: "frequency", title: "Frecuencia", keyPattern: /frecuencia|clock/i },
    { id: "tdp", title: "Consumo (TDP)", keyPattern: /tdp|consumo/i },
  ],
  graphics: [
    { id: "vram", title: "VRAM", keyPattern: /vram/i },
    {
      id: "gpu-cores",
      title: "Núcleos de procesamiento",
      keyPattern: /núcleos cuda|cuda cores|stream processors/i,
    },
    { id: "memory-bus", title: "Bus de memoria", keyPattern: /bus de memoria/i },
  ],
  notebooks: [
    { id: "processor", title: "Procesador", keyPattern: PROCESSOR_KEY },
    { id: "ram", title: "RAM", keyPattern: RAM_KEY },
    { id: "storage", title: "Almacenamiento", keyPattern: STORAGE_KEY },
    { id: "graphics", title: "Gráficos", keyPattern: GRAPHICS_KEY },
    { id: "display", title: "Pantalla", keyPattern: DISPLAY_KEY },
    { id: "battery", title: "Batería", keyPattern: BATTERY_KEY },
  ],
  computers: [
    { id: "processor", title: "Procesador", keyPattern: PROCESSOR_KEY },
    { id: "ram", title: "RAM", keyPattern: RAM_KEY },
    { id: "storage", title: "Almacenamiento", keyPattern: STORAGE_KEY },
    { id: "graphics", title: "Gráficos", keyPattern: GRAPHICS_KEY },
  ],
  smartphones: [
    { id: "processor", title: "Procesador", keyPattern: PROCESSOR_KEY },
    { id: "ram", title: "RAM", keyPattern: RAM_KEY },
    { id: "storage", title: "Almacenamiento", keyPattern: STORAGE_KEY },
    { id: "display", title: "Pantalla", keyPattern: DISPLAY_KEY },
    { id: "camera", title: "Cámara", keyPattern: CAMERA_KEY },
    { id: "battery", title: "Batería", keyPattern: BATTERY_KEY },
  ],
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
  const [sort, setSort] = useState<SortOption>("relevance");
  const [priceMin, setPriceMin] = useState(0);
  const [priceMax, setPriceMax] = useState(PRICE_LIMIT);
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set());
  const [weightMin, setWeightMin] = useState(0);
  const [weightMax, setWeightMax] = useState(50);
  const [selectedTypes, setSelectedTypes] = useState<Set<ProductType>>(new Set());
  const [selectedAdvancedFilters, setSelectedAdvancedFilters] = useState<Record<string, Set<string>>>({});
  const [availableOnly, setAvailableOnly] = useState(false);
  const [filterPanelExpanded, setFilterPanelExpanded] = useState(false);
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
    subcategory: product.subcategory ?? "",
    image: "",
    images: [],
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.specs ?? (product.model ? { Modelo: product.model } : {}),
    offers: [],
    priceHistory: [],
    offerPriceHistory: [],
    tags: product.tags ?? [],
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

  const getSpecValue = (product: Product, keyPattern: RegExp) =>
    Object.entries(product.specs).find(([key]) => keyPattern.test(key))?.[1]?.trim();

  const getProductWeight = (product: Product) => {
    const weightEntry = Object.entries(product.specs).find(([key]) =>
      /peso|weight/i.test(key),
    );
    const weight = weightEntry?.[1].match(/(\d+(?:[.,]\d+)?)\s*(kg|g)?/i);
    if (!weight) return null;

    const value = Number(weight[1].replace(",", "."));
    return weight[2]?.toLowerCase() === "g" ? value / 1000 : value;
  };

  const getProductSearchText = (product: Product) =>
    [product.name, product.category, product.subcategory, ...product.tags]
      .join(" ")
      .toLowerCase();

  const matchesProductType = (product: Product, type: ProductType) => {
    const searchText = getProductSearchText(product);
    return PRODUCT_TYPE_TERMS[type].some((term) => searchText.includes(term));
  };

  const brands = [...new Set(products.map((p) => p.brand))].sort();
  const getAdvancedOptions = (type: ProductType, keyPattern: RegExp) =>
    [
      ...new Set(
        products
          .filter((product) => matchesProductType(product, type))
          .map((product) => getSpecValue(product, keyPattern))
          .filter((value): value is string => Boolean(value)),
      ),
    ].sort();

  const matchesProduct = (product: Product, excludedFacet?: FilterFacet) => {
    if (
      query &&
      !product.name.toLowerCase().includes(query.toLowerCase()) &&
      !product.brand.toLowerCase().includes(query.toLowerCase()) &&
      !product.category.toLowerCase().includes(query.toLowerCase())
    ) {
      return false;
    }

    if (
      excludedFacet !== "brand" &&
      selectedBrands.size > 0 &&
      !selectedBrands.has(product.brand)
    ) {
      return false;
    }

    const productPrice = getProductPrice(product);
    if (
      excludedFacet !== "price" &&
      (productPrice < priceMin || productPrice > priceMax)
    ) {
      return false;
    }

    if (
      excludedFacet !== "availability" &&
      availableOnly &&
      !product.offers.some((offer) => offer.available)
    ) {
      return false;
    }

    const productWeight = getProductWeight(product);
    if (
      excludedFacet !== "weight" &&
      (productWeight === null
        ? weightMin > 0 || weightMax < 50
        : productWeight < weightMin || productWeight > weightMax)
    ) {
      return false;
    }

    if (excludedFacet !== "type" && selectedTypes.size > 0) {
      const matchesSelectedType = [...selectedTypes].some((type) => {
        if (!matchesProductType(product, type)) return false;

        return ADVANCED_FILTERS[type].every((definition) => {
          const selectedValues = selectedAdvancedFilters[`${type}:${definition.id}`];
          return (
            !selectedValues?.size ||
            excludedFacet === `${type}:${definition.id}` ||
            selectedValues.has(getSpecValue(product, definition.keyPattern) ?? "")
          );
        });
      });

      if (!matchesSelectedType) return false;
    }

    return true;
  };

  const countForFacet = (
    facet: FilterFacet,
    matchesOption: (product: Product) => boolean = () => true,
  ) => products.filter((product) => matchesProduct(product, facet) && matchesOption(product)).length;

  const brandCounts = new Map(
    brands.map((brand) => [brand, countForFacet("brand", (product) => product.brand === brand)]),
  );
  const typeCounts = Object.fromEntries(
    (Object.keys(PRODUCT_TYPE_LABELS) as ProductType[]).map((type) => [
      type,
      countForFacet("type", (product) => {
        const searchText = getProductSearchText(product);
        return PRODUCT_TYPE_TERMS[type].some((term) => searchText.includes(term));
      }),
    ]),
  ) as Record<ProductType, number>;
  const productTypeCount = countForFacet("type", (product) => {
    const searchText = getProductSearchText(product);
    return Object.values(PRODUCT_TYPE_TERMS).some((terms) =>
      terms.some((term) => searchText.includes(term)),
    );
  });
  const priceCount = countForFacet("price");
  const weightCount = countForFacet("weight");
  const availableCount = countForFacet("availability", (product) =>
    product.offers.some((offer) => offer.available),
  );

  const selectedFilterCount =
    selectedBrands.size +
    selectedTypes.size +
    Object.values(selectedAdvancedFilters).reduce((total, values) => total + values.size, 0) +
    Number(availableOnly) +
    Number(priceMin > 0 || priceMax < PRICE_LIMIT) +
    Number(weightMin > 0 || weightMax < 50);

  const filtered = products.filter((product) => matchesProduct(product));
  filtered.sort((a, b) => {
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

  const toggleProductType = (type: ProductType) => {
    if (selectedTypes.has(type)) {
      setSelectedAdvancedFilters((previous) =>
        Object.fromEntries(
          Object.entries(previous).filter(([filterId]) => !filterId.startsWith(`${type}:`)),
        ),
      );
    }
    setSelectedTypes((prev) => {
      const next = new Set(prev);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
    setPage(1);
  };

  const toggleAdvancedFilter = (type: ProductType, definition: AdvancedFilterDefinition, value: string) => {
    const filterId = `${type}:${definition.id}`;
    setSelectedAdvancedFilters((previous) => {
      const values = new Set(previous[filterId] ?? []);
      if (values.has(value)) values.delete(value);
      else values.add(value);
      return { ...previous, [filterId]: values };
    });
    setPage(1);
  };

  const renderAdvancedFilters = (type: ProductType) => (
    <CollapsibleFilterSection key={type} title={`${PRODUCT_TYPE_LABELS[type]}: filtros avanzados`} defaultOpen>
      <div className="space-y-4">
        {ADVANCED_FILTERS[type].map((definition) => {
          const options = getAdvancedOptions(type, definition.keyPattern);
          const filterId = `${type}:${definition.id}`;
          const selectedValues = selectedAdvancedFilters[filterId] ?? new Set<string>();
          const getCount = (option?: string) =>
            products.filter(
              (product) =>
                matchesProduct(product, filterId) &&
                matchesProductType(product, type) &&
                (option === undefined ||
                  getSpecValue(product, definition.keyPattern) === option),
            ).length;

          return (
            <SearchableAdvancedFilter
              key={definition.id}
              title={definition.title}
              count={getCount()}
              options={options}
              selectedValues={selectedValues}
              getOptionCount={(option) => getCount(option)}
              onToggle={(option) => toggleAdvancedFilter(type, definition, option)}
            />
          );
        })}
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
              max={PRICE_LIMIT}
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
              max={PRICE_LIMIT}
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

      {[...selectedTypes].map((type) => renderAdvancedFilters(type))}

      <CollapsibleFilterSection title={`Marca (${countForFacet("brand")})`}>
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
      {(selectedBrands.size > 0 ||
        selectedTypes.size > 0 ||
        Object.values(selectedAdvancedFilters).some((values) => values.size > 0) ||
        priceMin > 0 ||
        priceMax < PRICE_LIMIT ||
        weightMin > 0 ||
        weightMax < 50 ||
        availableOnly) && (
        <button
          onClick={() => {
            setSelectedBrands(new Set());
            setSelectedTypes(new Set());
            setSelectedAdvancedFilters({});
            setPriceMin(0);
            setPriceMax(PRICE_LIMIT);
            setWeightMin(0);
            setWeightMax(50);
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
