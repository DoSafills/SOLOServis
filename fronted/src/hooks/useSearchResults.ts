import { useEffect, useState } from "react";
import type { Product } from "../types";
import { getProducts, toProduct, type ApiProduct } from "../services/api/products";

export type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";
export type ProductType = "cpus" | "graphics" | "notebooks" | "computers" | "smartphones";

interface AdvancedFilterDefinition {
  id: string;
  title: string;
  keyPattern: RegExp;
}

export interface AdvancedFilterView {
  id: string;
  title: string;
  count: number;
  options: string[];
  selectedValues: Set<string>;
  getOptionCount: (option: string) => number;
  onToggle: (option: string) => void;
}

export interface AdvancedFilterGroupView {
  type: ProductType;
  title: string;
  filters: AdvancedFilterView[];
}

export interface ProductTypeFilterView {
  type: ProductType;
  label: string;
  count: number;
  selected: boolean;
  onToggle: () => void;
}

export interface BrandFilterView {
  brand: string;
  count: number;
  selected: boolean;
  onToggle: () => void;
}

const PER_PAGE = 6;
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
    { id: "gpu-cores", title: "Núcleos de procesamiento", keyPattern: /núcleos cuda|cuda cores|stream processors/i },
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

export function useSearchResults(query: string) {
  const [apiProducts, setApiProducts] = useState<ApiProduct[]>([]);
  const [sort, setSort] = useState<SortOption>("relevance");
  const [priceMin, setPriceMin] = useState(0);
  const [priceMax, setPriceMax] = useState(PRICE_LIMIT);
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set());
  const [weightMin, setWeightMin] = useState(0);
  const [weightMax, setWeightMax] = useState(50);
  const [selectedTypes, setSelectedTypes] = useState<Set<ProductType>>(new Set());
  const [selectedAdvancedFilters, setSelectedAdvancedFilters] = useState<Record<string, Set<string>>>({});
  const [availableOnly, setAvailableOnly] = useState(false);
  const [page, setPage] = useState(1);

  useEffect(() => {
    let cancelled = false;

    getProducts()
      .then((products) => {
        if (!cancelled) setApiProducts(products);
      })
      .catch(console.error);

    return () => {
      cancelled = true;
    };
  }, []);

  useEffect(() => {
    setPage(1);
  }, [query]);

  const products: Product[] = apiProducts.map(toProduct);
  const getProductPrice = (product: Product) =>
    product.offerPrice ??
    product.offers.reduce(
      (lowest, offer) => Math.min(lowest, offer.price),
      product.offers.length ? Number.POSITIVE_INFINITY : 0,
    );
  const getSpecValue = (product: Product, keyPattern: RegExp) =>
    Object.entries(product.specs).find(([key]) => keyPattern.test(key))?.[1]?.trim();
  const getProductWeight = (product: Product) => {
    const weightEntry = Object.entries(product.specs).find(([key]) => /peso|weight/i.test(key));
    const weight = weightEntry?.[1].match(/(\d+(?:[.,]\d+)?)\s*(kg|g)?/i);
    if (!weight) return null;
    const value = Number(weight[1].replace(",", "."));
    return weight[2]?.toLowerCase() === "g" ? value / 1000 : value;
  };
  const getProductSearchText = (product: Product) =>
    [product.name, product.category, product.subcategory, ...product.tags].join(" ").toLowerCase();
  const matchesProductType = (product: Product, type: ProductType) =>
    PRODUCT_TYPE_TERMS[type].some((term) => getProductSearchText(product).includes(term));
  const getAdvancedOptions = (type: ProductType, keyPattern: RegExp) =>
    [...new Set(
      products
        .filter((product) => matchesProductType(product, type))
        .map((product) => getSpecValue(product, keyPattern))
        .filter((value): value is string => Boolean(value)),
    )].sort();

  const brands = [...new Set(products.map((product) => product.brand))].sort();

  const matchesProduct = (product: Product, excludedFacet?: string) => {
    const normalizedQuery = query.trim().toLowerCase();
    if (
      normalizedQuery &&
      !product.name.toLowerCase().includes(normalizedQuery) &&
      !product.brand.toLowerCase().includes(normalizedQuery) &&
      !product.category.toLowerCase().includes(normalizedQuery)
    ) return false;
    if (excludedFacet !== "brand" && selectedBrands.size && !selectedBrands.has(product.brand)) return false;

    const price = getProductPrice(product);
    if (excludedFacet !== "price" && (price < priceMin || price > priceMax)) return false;
    if (excludedFacet !== "availability" && availableOnly && !product.offers.some((offer) => offer.available)) return false;

    const weight = getProductWeight(product);
    if (
      excludedFacet !== "weight" &&
      (weight === null ? weightMin > 0 || weightMax < 50 : weight < weightMin || weight > weightMax)
    ) return false;

    if (excludedFacet !== "type" && selectedTypes.size > 0) {
      const matchesSelectedType = [...selectedTypes].some((type) => {
        if (!matchesProductType(product, type)) return false;
        return ADVANCED_FILTERS[type].every((definition) => {
          const values = selectedAdvancedFilters[`${type}:${definition.id}`];
          return !values?.size || excludedFacet === `${type}:${definition.id}` ||
            values.has(getSpecValue(product, definition.keyPattern) ?? "");
        });
      });
      if (!matchesSelectedType) return false;
    }
    return true;
  };

  const countForFacet = (facet: string, matchesOption: (product: Product) => boolean = () => true) =>
    products.filter((product) => matchesProduct(product, facet) && matchesOption(product)).length;
  const typeCounts = Object.fromEntries(
    (Object.keys(PRODUCT_TYPE_LABELS) as ProductType[]).map((type) => [
      type,
      countForFacet("type", (product) => matchesProductType(product, type)),
    ]),
  ) as Record<ProductType, number>;
  const productTypeCount = countForFacet("type", (product) =>
    (Object.keys(PRODUCT_TYPE_LABELS) as ProductType[]).some((type) => matchesProductType(product, type)),
  );
  const brandCounts = new Map(
    brands.map((brand) => [brand, countForFacet("brand", (product) => product.brand === brand)]),
  );
  const brandCount = countForFacet("brand");
  const priceCount = countForFacet("price");
  const weightCount = countForFacet("weight");
  const availableCount = countForFacet("availability", (product) =>
    product.offers.some((offer) => offer.available),
  );
  const selectedFilterCount =
    selectedBrands.size + selectedTypes.size +
    Object.values(selectedAdvancedFilters).reduce((total, values) => total + values.size, 0) +
    Number(availableOnly) + Number(priceMin > 0 || priceMax < PRICE_LIMIT) +
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
    setSelectedBrands((previous) => {
      const next = new Set(previous);
      if (next.has(brand)) next.delete(brand);
      else next.add(brand);
      return next;
    });
    setPage(1);
  };
  const toggleProductType = (type: ProductType) => {
    if (selectedTypes.has(type)) {
      setSelectedAdvancedFilters((previous) =>
        Object.fromEntries(Object.entries(previous).filter(([key]) => !key.startsWith(`${type}:`))),
      );
    }
    setSelectedTypes((previous) => {
      const next = new Set(previous);
      if (next.has(type)) next.delete(type);
      else next.add(type);
      return next;
    });
    setPage(1);
  };
  const toggleAdvancedFilter = (type: ProductType, id: string, value: string) => {
    const filterId = `${type}:${id}`;
    setSelectedAdvancedFilters((previous) => {
      const values = new Set(previous[filterId] ?? []);
      if (values.has(value)) values.delete(value);
      else values.add(value);
      return { ...previous, [filterId]: values };
    });
    setPage(1);
  };
  const updatePriceMin = (value: number) => {
    setPriceMin(Math.min(value, priceMax));
    setPage(1);
  };
  const updatePriceMax = (value: number) => {
    setPriceMax(Math.max(value, priceMin));
    setPage(1);
  };
  const updateWeightMin = (value: number) => {
    setWeightMin(Math.min(value, weightMax));
    setPage(1);
  };
  const updateWeightMax = (value: number) => {
    setWeightMax(Math.max(value, weightMin));
    setPage(1);
  };
  const updateAvailability = (value: boolean) => {
    setAvailableOnly(value);
    setPage(1);
  };

  const advancedFilterGroups: AdvancedFilterGroupView[] = [...selectedTypes].map((type) => ({
    type,
    title: PRODUCT_TYPE_LABELS[type],
    filters: ADVANCED_FILTERS[type].map((definition) => {
      const filterId = `${type}:${definition.id}`;
      const options = getAdvancedOptions(type, definition.keyPattern);
      const getCount = (option?: string) => products.filter((product) =>
        matchesProduct(product, filterId) && matchesProductType(product, type) &&
        (option === undefined || getSpecValue(product, definition.keyPattern) === option),
      ).length;
      return {
        id: definition.id,
        title: definition.title,
        count: getCount(),
        options,
        selectedValues: selectedAdvancedFilters[filterId] ?? new Set<string>(),
        getOptionCount: (option: string) => getCount(option),
        onToggle: (option: string) => toggleAdvancedFilter(type, definition.id, option),
      };
    }),
  }));

  const clearFilters = () => {
    setSelectedBrands(new Set());
    setSelectedTypes(new Set());
    setSelectedAdvancedFilters({});
    setPriceMin(0);
    setPriceMax(PRICE_LIMIT);
    setWeightMin(0);
    setWeightMax(50);
    setAvailableOnly(false);
    setPage(1);
  };
  const shouldShowClearFilters =
    selectedBrands.size > 0 || selectedTypes.size > 0 ||
    Object.values(selectedAdvancedFilters).some((values) => values.size > 0) ||
    priceMin > 0 || priceMax < PRICE_LIMIT || weightMin > 0 || weightMax < 50 || availableOnly;

  const brandFilters: BrandFilterView[] = brands.map((brand) => ({
    brand,
    count: brandCounts.get(brand) ?? 0,
    selected: selectedBrands.has(brand),
    onToggle: () => toggleBrand(brand),
  }));
  const typeFilters: ProductTypeFilterView[] = (Object.keys(PRODUCT_TYPE_LABELS) as ProductType[]).map((type) => ({
    type,
    label: PRODUCT_TYPE_LABELS[type],
    count: typeCounts[type],
    selected: selectedTypes.has(type),
    onToggle: () => toggleProductType(type),
  }));

  return {
    sort,
    setSort,
    priceMin,
    priceMax,
    priceCount,
    priceLimit: PRICE_LIMIT,
    updatePriceMin,
    updatePriceMax,
    weightMin,
    weightMax,
    weightCount,
    updateWeightMin,
    updateWeightMax,
    availableOnly,
    availableCount,
    updateAvailability,
    selectedFilterCount,
    productTypeCount,
    typeFilters,
    advancedFilterGroups,
    brandCount,
    brandFilters,
    shouldShowClearFilters,
    clearFilters,
    filtered,
    paginated,
    page,
    setPage,
    perPage: PER_PAGE,
  };
}