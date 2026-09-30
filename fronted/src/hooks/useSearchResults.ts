import { useEffect, useState } from "react";
import type { Product } from "../types";
import { getProducts, toProduct, type ApiProduct } from "../services/api/products";

export type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";
export type ProductType =
  | "cpus"
  | "graphics"
  | "notebooks"
  | "computers"
  | "smartphones"
  | "refrigeration"
  | "laundry"
  | "cooking"
  | "climate"
  | "smallAppliances";
export type MacroCategory =
  | "tecnologia"
  | "computacion"
  | "celulares"
  | "electrodomesticos"
  | "gaming"
  | "hogar";

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

export interface MacroCategoryFilterView {
  category: MacroCategory;
  label: string;
  count: number;
  selected: boolean;
  onToggle: () => void;
}

export interface RatingFilterView {
  min: number;
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
const WEIGHT_LIMIT = 150; // kg; los electrodomésticos grandes superan los 50 kg
const RATING_OPTIONS = [4, 3, 2];
const PROCESSOR_KEY = /procesador|processor|cpu/i;
const RAM_KEY = /ram|memoria/i;
const STORAGE_KEY = /almacenamiento|storage|disco/i;
const GRAPHICS_KEY = /gpu|gráfica|graphics|video/i;
const DISPLAY_KEY = /pantalla|display|screen/i;
const BATTERY_KEY = /batería|battery/i;
const CAMERA_KEY = /cámara|camera/i;

// Claves de specs para electrodomésticos
const CAPACITY_KEY = /capacidad|capacity/i;
const BTU_KEY = /btu|capacidad|capacity/i;
const POWER_KEY = /potencia|watt|power|consumo/i;
const ENERGY_EFFICIENCY_KEY = /eficiencia|clase energ|etiqueta energ|energy (class|efficiency|rating)/i;
const APPLIANCE_TECH_KEY = /tecnolog|inverter|no ?frost|inducci|wi-?fi|smart|conectividad/i;
const INSTALLATION_KEY = /instalaci|montaje|empotr/i;
const HEIGHT_KEY = /^alto|altura|height/i;
const WIDTH_KEY = /ancho|width/i;
const DEPTH_KEY = /fondo|profundidad|depth/i;
const COLOR_KEY = /color|acabado|finish/i;
const WARRANTY_KEY = /garant[ií]a|warranty/i;
const NOISE_KEY = /ruido|noise|decibel|\bdb\b/i;
const ENERGY_SOURCE_KEY = /fuente|tipo de energ|combustible/i;
const VOLTAGE_KEY = /voltaje|tensi[oó]n|voltage/i;
const AVAILABILITY_KEY = /disponibilidad|stock|retiro|env[ií]o|availability/i;

const PRODUCT_TYPE_LABELS: Record<ProductType, string> = {
  cpus: "CPUs",
  graphics: "Gráficas",
  notebooks: "Notebooks",
  computers: "Computadores",
  smartphones: "Smartphones",
  refrigeration: "Refrigeración",
  laundry: "Lavado",
  cooking: "Cocina",
  climate: "Climatización",
  smallAppliances: "Pequeños electrodomésticos",
};

const PRODUCT_TYPE_TERMS: Record<ProductType, string[]> = {
  cpus: ["cpu", "procesador", "procesadores", "processor"],
  graphics: ["gráfica", "grafica", "gpu", "rtx", "radeon", "rx ", "graphics"],
  notebooks: ["notebook", "laptop"],
  computers: ["computador", "computadora", "computadores", "desktop", "pc de escritorio", "all-in-one", "torre"],
  smartphones: ["smartphone", "celular", "teléfono", "telefono", "móvil", "movil"],
  refrigeration: ["refrigerador", "refrigeradora", "refri", "nevera", "frigobar", "freezer", "congelador", "frigorífico", "frigorifico"],
  laundry: ["lavadora", "secadora", "lavasecadora", "lavavajillas", "lavaplatos"],
  cooking: ["horno", "microondas", "encimera", "cocina a gas", "cocina eléctrica", "cocina electrica", "cocina de inducción", "cocina de induccion", "campana", "extractor"],
  climate: ["aire acondicionado", "climatizador", "calefactor", "estufa", "ventilador", "deshumidificador", "split", "purificador de aire"],
  smallAppliances: ["licuadora", "cafetera", "batidora", "tostadora", "hervidor", "freidora", "aspiradora", "plancha", "sandwichera", "exprimidor", "procesadora", "multicooker", "robot de cocina"],
};

const MACRO_CATEGORY_DEFINITIONS: Array<{
  id: MacroCategory;
  label: string;
  pattern: RegExp;
}> = [
  {
    id: "tecnologia",
    label: "Tecnología",
    pattern:
      /tecnolog[ií]a|technology|electr[oó]nic|electronic|digital|tablet|smart\s?watch|wearable|aud[ií]fono|headphone|monitor|router|impresora|c[aá]mara|proyector|celular|smartphone|notebook|laptop|computador|desktop|procesador|gr[aá]fica|gpu|consola|gaming|gamer|videojuego|playstation|xbox|nintendo/i,
  },
  {
    id: "computacion",
    label: "Computación",
    pattern:
      /computaci[oó]n|computador|computadora|desktop|notebook|laptop|pc\b|procesador|processor|cpu|tarjeta gr[aá]fica|gpu|motherboard|placa madre/i,
  },
  {
    id: "celulares",
    label: "Celulares",
    pattern: /celular|smartphone|tel[eé]fono m[oó]vil|m[oó]vil|iphone|android/i,
  },
  {
    id: "electrodomesticos",
    label: "Electrodomésticos",
    pattern:
      /electrodom[eé]stic|refrigerador|\brefri\b|nevera|frigobar|lavadora|secadora|lavasecadora|microondas|horno|encimera|campana|aspiradora|cafetera|licuadora|batidora|tostadora|hervidor|freidora|plancha|lavavajillas|freezer|congelador|aire acondicionado|climatizador|calefactor|ventilador|deshumidificador/i,
  },
  {
    id: "gaming",
    label: "Gaming",
    pattern: /gaming|gamer|consola|videojuego|playstation|xbox|nintendo|joystick|gamepad/i,
  },
  {
    id: "hogar",
    label: "Hogar",
    pattern:
      /hogar|home|mueble|decoraci[oó]n|decoraci|cocina|dormitorio|living|cama|silla|mesa|sof[aá]|l[aá]mpara|iluminaci[oó]n|textil|vajilla|menaje|jard[ií]n/i,
  },
];

const MACRO_CATEGORY_PRODUCT_TYPES: Record<MacroCategory, ProductType[]> = {
  tecnologia: ["cpus", "graphics", "notebooks", "computers", "smartphones"],
  computacion: ["cpus", "graphics", "notebooks", "computers"],
  celulares: ["smartphones"],
  electrodomesticos: ["refrigeration", "laundry", "cooking", "climate", "smallAppliances"],
  gaming: ["graphics"],
  hogar: [],
};

// Filtros técnicos, físicos y de servicio compartidos por todos los electrodomésticos.
// Solo cambia la definición de capacidad (litros, kg, BTU) según el tipo.
const applianceFilters = (capacity: AdvancedFilterDefinition): AdvancedFilterDefinition[] => [
  capacity,
  { id: "power", title: "Potencia / consumo", keyPattern: POWER_KEY },
  { id: "energy-efficiency", title: "Eficiencia energética", keyPattern: ENERGY_EFFICIENCY_KEY },
  { id: "technology", title: "Tecnología", keyPattern: APPLIANCE_TECH_KEY },
  { id: "installation", title: "Tipo de instalación", keyPattern: INSTALLATION_KEY },
  { id: "height", title: "Alto", keyPattern: HEIGHT_KEY },
  { id: "width", title: "Ancho", keyPattern: WIDTH_KEY },
  { id: "depth", title: "Fondo", keyPattern: DEPTH_KEY },
  { id: "color", title: "Color y acabado", keyPattern: COLOR_KEY },
  { id: "warranty", title: "Garantía", keyPattern: WARRANTY_KEY },
  { id: "noise", title: "Nivel de ruido (dB)", keyPattern: NOISE_KEY },
  { id: "energy-source", title: "Fuente de energía", keyPattern: ENERGY_SOURCE_KEY },
  { id: "voltage", title: "Voltaje", keyPattern: VOLTAGE_KEY },
  { id: "availability", title: "Disponibilidad", keyPattern: AVAILABILITY_KEY },
];

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
  refrigeration: applianceFilters({ id: "capacity", title: "Capacidad (litros)", keyPattern: CAPACITY_KEY }),
  laundry: applianceFilters({ id: "capacity", title: "Capacidad (kg)", keyPattern: CAPACITY_KEY }),
  cooking: applianceFilters({ id: "capacity", title: "Capacidad (litros)", keyPattern: CAPACITY_KEY }),
  climate: applianceFilters({ id: "capacity", title: "Capacidad (BTU)", keyPattern: BTU_KEY }),
  smallAppliances: applianceFilters({ id: "capacity", title: "Capacidad", keyPattern: CAPACITY_KEY }),
};

export function useSearchResults(query: string) {
  const [apiProducts, setApiProducts] = useState<ApiProduct[]>([]);
  const [sort, setSort] = useState<SortOption>("relevance");
  const [priceMin, setPriceMin] = useState(0);
  const [priceMax, setPriceMax] = useState(PRICE_LIMIT);
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set());
  const [weightMin, setWeightMin] = useState(0);
  const [weightMax, setWeightMax] = useState(WEIGHT_LIMIT);
  const [minRating, setMinRating] = useState(0);
  const [onlyDiscount, setOnlyDiscount] = useState(false);
  const [selectedTypes, setSelectedTypes] = useState<Set<ProductType>>(new Set());
  const [selectedMacroCategories, setSelectedMacroCategories] = useState<Set<MacroCategory>>(new Set());
  const [selectedAdvancedFilters, setSelectedAdvancedFilters] = useState<Record<string, Set<string>>>({});
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
  const hasDiscount = (product: Product) =>
    product.offerPrice != null &&
    (product.offers.length === 0 || product.offers.some((offer) => offer.price > product.offerPrice!));
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
    [product.name, product.category, product.subcategory, product.description, ...product.tags]
      .join(" ")
      .toLowerCase();
  const matchesProductType = (product: Product, type: ProductType) =>
    PRODUCT_TYPE_TERMS[type].some((term) => getProductSearchText(product).includes(term));
  const matchesMacroCategory = (product: Product, category: MacroCategory) => {
    const definition = MACRO_CATEGORY_DEFINITIONS.find((item) => item.id === category);
    return definition?.pattern.test(getProductSearchText(product)) ?? false;
  };
  const getAdvancedOptions = (type: ProductType, keyPattern: RegExp) =>
    [...new Set(
      products
        .filter((product) => matchesProductType(product, type))
        .map((product) => getSpecValue(product, keyPattern))
        .filter((value): value is string => Boolean(value)),
    )].sort();

  const brands = [...new Set(products.map((product) => product.brand))].sort();
  const visibleProductTypes = [...new Set(
    [...selectedMacroCategories].flatMap((category) => MACRO_CATEGORY_PRODUCT_TYPES[category]),
  )];

  const matchesProduct = (product: Product, excludedFacet?: string) => {
    const normalizedQuery = query.trim().toLowerCase();
    if (
      normalizedQuery &&
      !product.name.toLowerCase().includes(normalizedQuery) &&
      !product.brand.toLowerCase().includes(normalizedQuery) &&
      !product.category.toLowerCase().includes(normalizedQuery)
    ) return false;
    if (excludedFacet !== "brand" && selectedBrands.size && !selectedBrands.has(product.brand)) return false;
    if (
      excludedFacet !== "macro-category" &&
      selectedMacroCategories.size > 0 &&
      ![...selectedMacroCategories].some((category) => matchesMacroCategory(product, category))
    ) return false;

    if (excludedFacet !== "rating" && minRating > 0 && product.rating < minRating) return false;
    if (excludedFacet !== "discount" && onlyDiscount && !hasDiscount(product)) return false;

    const price = getProductPrice(product);
    if (excludedFacet !== "price" && (price < priceMin || price > priceMax)) return false;

    const weight = getProductWeight(product);
    if (
      excludedFacet !== "weight" &&
      (weight === null ? weightMin > 0 || weightMax < WEIGHT_LIMIT : weight < weightMin || weight > weightMax)
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
    visibleProductTypes.some((type) => matchesProductType(product, type)),
  );
  const macroCategoryCount = countForFacet("macro-category", (product) =>
    MACRO_CATEGORY_DEFINITIONS.some(({ id }) => matchesMacroCategory(product, id)),
  );
  const macroCategoryFilters: MacroCategoryFilterView[] = MACRO_CATEGORY_DEFINITIONS.map(({ id, label }) => ({
    category: id,
    label,
    count: countForFacet("macro-category", (product) => matchesMacroCategory(product, id)),
    selected: selectedMacroCategories.has(id),
    onToggle: () => toggleMacroCategory(id),
  }));
  const brandCounts = new Map(
    brands.map((brand) => [brand, countForFacet("brand", (product) => product.brand === brand)]),
  );
  const brandCount = countForFacet("brand");
  const priceCount = countForFacet("price");
  const weightCount = countForFacet("weight");
  const discountCount = countForFacet("discount", hasDiscount);
  const ratingFilters: RatingFilterView[] = RATING_OPTIONS.map((min) => ({
    min,
    label: `${min} estrellas o más`,
    count: countForFacet("rating", (product) => product.rating >= min),
    selected: minRating === min,
    onToggle: () => toggleRating(min),
  }));
  const selectedFilterCount =
    Number(minRating > 0) + Number(onlyDiscount) + selectedBrands.size + selectedTypes.size + selectedMacroCategories.size +
    Object.values(selectedAdvancedFilters).reduce((total, values) => total + values.size, 0) +
    Number(priceMin > 0 || priceMax < PRICE_LIMIT) +
    Number(weightMin > 0 || weightMax < WEIGHT_LIMIT);

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
  const toggleRating = (min: number) => {
    setMinRating((previous) => (previous === min ? 0 : min));
    setPage(1);
  };
  const toggleDiscount = () => {
    setOnlyDiscount((previous) => !previous);
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
  const toggleMacroCategory = (category: MacroCategory) => {
    const nextCategories = new Set(selectedMacroCategories);
    if (nextCategories.has(category)) nextCategories.delete(category);
    else nextCategories.add(category);

    const availableTypes = new Set(
      [...nextCategories].flatMap((selectedCategory) => MACRO_CATEGORY_PRODUCT_TYPES[selectedCategory]),
    );
    const nextTypes = new Set([...selectedTypes].filter((type) => availableTypes.has(type)));

    setSelectedMacroCategories(nextCategories);
    setSelectedTypes(nextTypes);
    setSelectedAdvancedFilters((previous) =>
      Object.fromEntries(
        Object.entries(previous).filter(([key]) => nextTypes.has(key.split(":")[0] as ProductType)),
      ),
    );
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
  const advancedFilterGroups: AdvancedFilterGroupView[] = [...selectedTypes]
    .filter((type) => visibleProductTypes.includes(type))
    .map((type) => ({
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
    setSelectedMacroCategories(new Set());
    setSelectedAdvancedFilters({});
    setMinRating(0);
    setOnlyDiscount(false);
    setPriceMin(0);
    setPriceMax(PRICE_LIMIT);
    setWeightMin(0);
    setWeightMax(WEIGHT_LIMIT);
    setPage(1);
  };
  const shouldShowClearFilters =
    minRating > 0 || onlyDiscount || selectedBrands.size > 0 || selectedTypes.size > 0 || selectedMacroCategories.size > 0 ||
    Object.values(selectedAdvancedFilters).some((values) => values.size > 0) ||
    priceMin > 0 || priceMax < PRICE_LIMIT || weightMin > 0 || weightMax < WEIGHT_LIMIT;

  const brandFilters: BrandFilterView[] = brands.map((brand) => ({
    brand,
    count: brandCounts.get(brand) ?? 0,
    selected: selectedBrands.has(brand),
    onToggle: () => toggleBrand(brand),
  }));
  const typeFilters: ProductTypeFilterView[] = visibleProductTypes.map((type) => ({
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
    weightLimit: WEIGHT_LIMIT,
    minRating,
    ratingFilters,
    onlyDiscount,
    discountCount,
    toggleDiscount,
    updateWeightMin,
    updateWeightMax,
    selectedFilterCount,
    productTypeCount,
    typeFilters,
    macroCategoryCount,
    macroCategoryFilters,
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