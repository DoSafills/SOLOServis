import { useEffect, useMemo, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import { getProductById } from "../../services/api/api";
import { getCategories, type ApiProductCategory } from "../../services/api/products";
import { getDynamicCategories, type ProductCategory } from "../../services/api/categories";
import {
  getSearchFacetsAcrossCategories,
  getSearchFacets,
  searchProductsAcrossCategories,
  searchProducts,
  type SearchFacets,
  type SearchProductsParams,
} from "../../services/api/search";
import ProductCard from "../../components/products/ProductCard";
import {
  Breadcrumb,
  EmptyState,
  Pagination,
  ProductSkeletonGrid,
  Skeleton,
} from "../../components/common/ui";
import {
  PRODUCT_CATEGORY_TREE,
  findProductCategoryPath,
  getProductCategoryLeafSlugs,
  getProductCategorySlugName,
  isProductCategorySlug,
  type ProductCategoryGroup,
  type ProductCategoryLeaf,
  type ProductCategorySection,
} from "../../constants/categoryTree";
import {
  type SessionStorageCodec,
  nullablePositiveIntegerSessionCodec,
  nullableStringSessionCodec,
  positiveIntegerSessionCodec,
  stringSessionCodec,
  useSessionStorageState,
} from "../../hooks/useSessionStorageState";

interface Props {
  query: string;
  category?: string;
  categoryContext?: string;
  productGroup?: string;
  categoryId?: number;
  categoryIds?: number[];
  resetFilters?: boolean;
  pageSize: number;
  onPageSizeChange: (pageSize: number) => void;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  productCompareList: Set<string>;
  cartProductIds: ReadonlySet<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
  onToggleFavorite: (id: string) => void;
  onToggleProductCompare: (id: string, category: string) => void;
}

type SortOption = string;

interface ProductFilterState {
  priceMin: string;
  priceMax: string;
  selectedBrands: Set<number>;
  selectedStores: Set<number>;
  selectedAttributes: Record<string, string>;
  attributeRanges: Record<string, { min: string; max: string }>;
  availableOnly: boolean;
}

const emptyFilters: ProductFilterState = {
  priceMin: "",
  priceMax: "",
  selectedBrands: new Set(),
  selectedStores: new Set(),
  selectedAttributes: {},
  attributeRanges: {},
  availableOnly: false,
};

/**
 * Valida el rango de precios antes de consultar la API. Recibe el mínimo y el
 * máximo como texto y devuelve un mensaje en español cuando son inválidos.
 */
function getPriceRangeError(min: string, max: string): string | null {
  const minimum = min.trim() === "" ? null : Number(min);
  const maximum = max.trim() === "" ? null : Number(max);
  if (
    (minimum !== null && (!Number.isFinite(minimum) || minimum < 0)) ||
    (maximum !== null && (!Number.isFinite(maximum) || maximum < 0))
  ) {
    return "Los precios deben ser números iguales o mayores que cero.";
  }
  if (minimum !== null && maximum !== null && minimum > maximum) {
    return "El precio mínimo no puede ser mayor que el precio máximo.";
  }
  return null;
}

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isStringRecord(value: unknown): value is Record<string, string> {
  return isRecord(value) && Object.values(value).every((item) => typeof item === "string");
}

const productFiltersSessionCodec: SessionStorageCodec<Record<string, ProductFilterState>> = {
  decode: (value) => {
    if (!isRecord(value)) return undefined;
    const restored: Record<string, ProductFilterState> = {};
    for (const [key, candidate] of Object.entries(value)) {
      if (!isRecord(candidate)) continue;
      const { priceMin, priceMax, selectedBrands, selectedStores, availableOnly } = candidate;
      const { selectedAttributes, attributeRanges } = candidate;
      if (
        typeof priceMin !== "string" ||
        typeof priceMax !== "string" ||
        !Array.isArray(selectedBrands) ||
        !selectedBrands.every((id) => typeof id === "number" && Number.isInteger(id)) ||
        !Array.isArray(selectedStores) ||
        !selectedStores.every((id) => typeof id === "number" && Number.isInteger(id)) ||
        typeof availableOnly !== "boolean" ||
        !isStringRecord(selectedAttributes) ||
        !isRecord(attributeRanges) ||
        getPriceRangeError(priceMin, priceMax) !== null
      ) {
        continue;
      }
      const restoredRanges: Record<string, { min: string; max: string }> = {};
      for (const [name, range] of Object.entries(attributeRanges)) {
        if (isRecord(range) && typeof range.min === "string" && typeof range.max === "string") {
          restoredRanges[name] = { min: range.min, max: range.max };
        }
      }
      restored[key] = {
        priceMin,
        priceMax,
        selectedBrands: new Set(selectedBrands),
        selectedStores: new Set(selectedStores),
        selectedAttributes,
        attributeRanges: restoredRanges,
        availableOnly,
      };
    }
    return restored;
  },
  encode: (filters) =>
    Object.fromEntries(
      Object.entries(filters).map(([key, filter]) => [
        key,
        {
          ...filter,
          selectedBrands: [...filter.selectedBrands],
          selectedStores: [...filter.selectedStores],
        },
      ]),
    ),
};

const normalizeCategoryKey = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase("es");

export default function SearchResultsPage({
  query,
  category,
  categoryContext,
  productGroup,
  categoryId,
  categoryIds,
  resetFilters,
  pageSize,
  onPageSizeChange,
  navigate,
  favorites,
  productCompareList,
  cartProductIds,
  onAddToCart,
  onToggleFavorite,
  onToggleProductCompare,
}: Props) {
  const sessionKey = `soloservice.product-search:${JSON.stringify([
    query,
    category,
    categoryContext,
    productGroup,
    categoryId,
    categoryIds,
  ])}`;
  // Con resetFilters se usa una clave de filtros nueva en cada apertura desde
  // el catálogo, para no heredar precio ni disponibilidad guardados antes.
  const [filtersSessionSuffix] = useState(() =>
    resetFilters ? `:${Date.now()}-${Math.random().toString(36).slice(2)}` : "",
  );
  const [sort, setSort] = useSessionStorageState<SortOption>(
    `${sessionKey}:sort`,
    () => "name",
    stringSessionCodec,
  );
  const activeProductGroup = productGroup ?? "all";
  // El backend acepta slugs NV1/NV2/NV3 en `category`; las rutas antiguas
  // usan productGroup=home|technology, que también se traducen a slug.
  const routeCategorySlug = category && isProductCategorySlug(category) ? category : null;
  const legacyGroupSlugs: Record<string, string> = { home: "hogar", technology: "tecnologia" };
  const groupScopeSlug =
    legacyGroupSlugs[activeProductGroup] ??
    (isProductCategorySlug(activeProductGroup) ? activeProductGroup : null);
  const isMultiCategoryView = Boolean(categoryIds?.length);
  const [filtersByGroup, setFiltersByGroup] = useSessionStorageState<
    Record<string, ProductFilterState>
  >(`${sessionKey}:filters${filtersSessionSuffix}`, () => ({}), productFiltersSessionCodec);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useSessionStorageState(
    `${sessionKey}:page`,
    () => 1,
    positiveIntegerSessionCodec,
  );
  const [products, setProducts] = useState<Product[]>([]);
  const [resultTotal, setResultTotal] = useState(0);
  const [facets, setFacets] = useState<SearchFacets | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadedSearchKey, setLoadedSearchKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [errorOpen, setErrorOpen] = useState(false);
  const [errorSource, setErrorSource] = useState<"price" | "search" | null>(null);
  const [categoryError, setCategoryError] = useState<string | null>(null);
  const [categories, setCategories] = useState<ApiProductCategory[]>([]);
  const [categoriesLoaded, setCategoriesLoaded] = useState(false);
  const [dynamicCategories, setDynamicCategories] = useState<ProductCategory[]>([]);
  const [selectedCategory, setSelectedCategory] = useSessionStorageState(
    `${sessionKey}:category`,
    () => categoryId ?? null,
    nullablePositiveIntegerSessionCodec,
  );
  const [selectedCategorySlug, setSelectedCategorySlug] = useSessionStorageState(
    `${sessionKey}:category-slug`,
    () => null,
    nullableStringSessionCodec,
  );
  const [searchAttempt, setSearchAttempt] = useState(0);
  const hasCategoryContext =
    Boolean(category) ||
    Boolean(categoryContext) ||
    categoryId !== undefined ||
    isMultiCategoryView;
  const isGroupedProductsView =
    groupScopeSlug !== null &&
    !category &&
    categoryId === undefined &&
    !isMultiCategoryView &&
    selectedCategory === null &&
    selectedCategorySlug === null;
  const showCategoryFilter = !hasCategoryContext || isGroupedProductsView;
  const groupCategoryLabel = groupScopeSlug
    ? (getProductCategorySlugName(groupScopeSlug) ?? "Categorías")
    : "Categorías";
  const activeFilterKey = activeProductGroup;
  const activeFilters = filtersByGroup[activeFilterKey] ?? emptyFilters;
  const {
    priceMin,
    priceMax,
    selectedBrands,
    selectedStores,
    selectedAttributes,
    attributeRanges,
    availableOnly,
  } = activeFilters;
  const [priceMinDraft, setPriceMinDraft] = useState(priceMin);
  const [priceMaxDraft, setPriceMaxDraft] = useState(priceMax);

  /**
   * Mezcla filtros del grupo activo, cierra errores previos y vuelve a la
   * primera página. Recibe el parche a aplicar y no devuelve nada.
   */
  const updateActiveFilters = (update: Partial<ProductFilterState>) => {
    setError(null);
    setErrorOpen(false);
    setErrorSource(null);
    setFiltersByGroup((previous) => ({
      ...previous,
      [activeFilterKey]: { ...(previous[activeFilterKey] ?? emptyFilters), ...update },
    }));
    setPage(1);
  };

  /**
   * Actualiza filtros mediante una función para preservar cambios simultáneos,
   * cierra errores previos y vuelve a la primera página.
   */
  const updateActiveFiltersWith = (update: (current: ProductFilterState) => ProductFilterState) => {
    setError(null);
    setErrorOpen(false);
    setErrorSource(null);
    setFiltersByGroup((previous) => ({
      ...previous,
      [activeFilterKey]: update(previous[activeFilterKey] ?? emptyFilters),
    }));
    setPage(1);
  };

  /**
   * Aplica los borradores de precio solo cuando la persona pulsa el botón.
   * Recibe ningún argumento, valida el rango en el navegador y no consulta la
   * API si el mínimo supera al máximo.
   */
  const applyPriceFilters = () => {
    const nextMin = priceMinDraft.trim();
    const nextMax = priceMaxDraft.trim();
    const validationError = getPriceRangeError(nextMin, nextMax);
    if (validationError) {
      setError(validationError);
      setErrorOpen(true);
      setErrorSource("price");
      return;
    }
    updateActiveFilters({ priceMin: nextMin, priceMax: nextMax });
  };

  useEffect(() => {
    getCategories()
      .then((result) => {
        setCategories(result);
        setCategoriesLoaded(true);
      })
      .catch((err: unknown) => {
        setCategoriesLoaded(true);
        setCategoryError(err instanceof Error ? err.message : "No se pudieron cargar categorías.");
      });
    getDynamicCategories()
      .then(setDynamicCategories)
      .catch((err: unknown) =>
        setCategoryError(err instanceof Error ? err.message : "No se pudieron cargar categorías."),
      );
  }, []);

  useEffect(() => {
    if (!filtersOpen) return;
    const closeOnEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") setFiltersOpen(false);
    };
    document.addEventListener("keydown", closeOnEscape);
    return () => document.removeEventListener("keydown", closeOnEscape);
  }, [filtersOpen]);

  const searchParams = useMemo<SearchProductsParams>(() => {
    const attributes: NonNullable<SearchProductsParams["attributes"]> = {};
    for (const [key, value] of Object.entries(activeFilters.selectedAttributes)) {
      attributes[key] = { value };
    }
    for (const [key, range] of Object.entries(activeFilters.attributeRanges)) {
      if (range.min || range.max) {
        attributes[key] = {
          ...attributes[key],
          ...(range.min ? { min: range.min } : {}),
          ...(range.max ? { max: range.max } : {}),
        };
      }
    }
    const categoryFromRoute = routeCategorySlug
      ? undefined
      : categories.find(
          (item) => normalizeCategoryKey(item.name) === normalizeCategoryKey(category ?? ""),
        )?.id;
    return {
      query,
      category: isMultiCategoryView
        ? undefined
        : (selectedCategorySlug ??
          selectedCategory ??
          categoryId ??
          routeCategorySlug ??
          categoryFromRoute),
      brandIds: [...activeFilters.selectedBrands],
      storeIds: [...activeFilters.selectedStores],
      minPrice: activeFilters.priceMin || undefined,
      maxPrice: activeFilters.priceMax || undefined,
      available: activeFilters.availableOnly || undefined,
      attributes,
      sort,
      limit: pageSize,
      offset: (page - 1) * pageSize,
    };
  }, [
    activeFilters,
    categories,
    category,
    categoryId,
    isMultiCategoryView,
    page,
    pageSize,
    query,
    routeCategorySlug,
    selectedCategory,
    selectedCategorySlug,
    sort,
  ]);
  const searchRequestKey = JSON.stringify([
    searchParams,
    categoryIds,
    searchAttempt,
    groupScopeSlug,
    isGroupedProductsView,
  ]);
  const searching =
    loading ||
    loadedSearchKey !== searchRequestKey ||
    (isMultiCategoryView && !categoriesLoaded) ||
    (Boolean(category) && !routeCategorySlug && !categoriesLoaded);

  useEffect(() => {
    if ((isMultiCategoryView || (Boolean(category) && !routeCategorySlug)) && !categoriesLoaded) {
      return;
    }
    let cancelled = false;
    const loadResults = async () => {
      if (
        category &&
        !routeCategorySlug &&
        !categories.some(
          (item) => normalizeCategoryKey(item.name) === normalizeCategoryKey(category),
        )
      ) {
        throw new Error(
          categoryError ?? `No se encontró la categoría "${category}" en el catálogo.`,
        );
      }
      return Promise.all(
        isMultiCategoryView
          ? [
              searchProductsAcrossCategories(searchParams, categoryIds ?? []),
              getSearchFacetsAcrossCategories(searchParams, categoryIds ?? []),
            ]
          : isGroupedProductsView
            ? [
                searchProducts({ ...searchParams, category: groupScopeSlug ?? undefined }),
                getSearchFacets({ ...searchParams, category: groupScopeSlug ?? undefined }),
              ]
            : [searchProducts(searchParams), getSearchFacets(searchParams)],
      );
    };
    loadResults()
      .then(async ([result, nextFacets]) => {
        const lastPage = Math.max(1, Math.ceil(result.total / pageSize));
        if (page > lastPage) {
          if (!cancelled) setPage(lastPage);
          return;
        }
        const details = await Promise.all(result.items.map((item) => getProductById(item.id)));
        const validDetails = details.filter((product): product is Product => product !== null);
        if (validDetails.length !== details.length) {
          throw new Error("Un producto cambió mientras se actualizaban los resultados. Reintenta.");
        }
        if (!cancelled) {
          setProducts(validDetails);
          setResultTotal(result.total);
          setFacets(nextFacets);
          setError(null);
          setErrorOpen(false);
          setErrorSource(null);
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Error al cargar productos.");
          setErrorOpen(true);
          setErrorSource("search");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadedSearchKey(searchRequestKey);
          setLoading(false);
        }
      });
    return () => {
      cancelled = true;
    };
  }, [
    categoriesLoaded,
    categories,
    category,
    categoryError,
    categoryIds,
    groupScopeSlug,
    isMultiCategoryView,
    isGroupedProductsView,
    page,
    pageSize,
    routeCategorySlug,
    searchParams,
    searchAttempt,
    searchRequestKey,
    setPage,
  ]);

  const selectedCategoryName = categories.find((cat) => cat.id === selectedCategory)?.name ?? "";
  const routeCategoryName = routeCategorySlug
    ? getProductCategorySlugName(routeCategorySlug)
    : null;
  const selectedDynamicCategoryName =
    dynamicCategories.find((item) => item.slug === selectedCategorySlug)?.name ?? "";
  const getCategoryPath = (categoryIdValue: number) => {
    const path: ApiProductCategory[] = [];
    let current = categories.find((item) => item.id === categoryIdValue);
    while (current) {
      path.push(current);
      const parentId = current.parentId;
      current = parentId === null ? undefined : categories.find((item) => item.id === parentId);
    }
    return path.reverse();
  };
  // Las hojas del grupo abierto acotan los atributos específicos; sin grupo
  // (o con una hoja elegida) se ofrecen todas las categorías con atributos.
  const groupLeafSlugs = groupScopeSlug ? getProductCategoryLeafSlugs(groupScopeSlug) : null;
  const availableDynamicCategories =
    selectedCategorySlug !== null || !groupLeafSlugs
      ? dynamicCategories
      : dynamicCategories.filter((item) => groupLeafSlugs.includes(item.slug));
  const attributeCategorySlug = dynamicCategories.some((item) => item.slug === selectedCategorySlug)
    ? selectedCategorySlug
    : "";
  const selectCategory = (nextCategory: number | null) => {
    if (nextCategory === selectedCategory && selectedCategorySlug === null) return;
    setLoading(true);
    setError(null);
    setPage(1);
    setSort("name");
    setSelectedCategorySlug(null);
    setSelectedCategory(nextCategory);
    updateActiveFilters({ selectedAttributes: {}, attributeRanges: {} });
  };
  const selectDynamicCategory = (slug: string) => {
    setPage(1);
    setSort("name");
    setSelectedCategory(null);
    setSelectedCategorySlug(slug || null);
    updateActiveFilters({ selectedAttributes: {}, attributeRanges: {} });
  };

  const brands = facets?.brands ?? [];
  const attributeFacets = selectedCategorySlug ? (facets?.attributes ?? []) : [];
  const paginated = products;

  const toggleBrand = (brandId: number) => {
    updateActiveFiltersWith((current) => {
      const selectedBrands = new Set(current.selectedBrands);
      if (selectedBrands.has(brandId)) selectedBrands.delete(brandId);
      else selectedBrands.add(brandId);
      return { ...current, selectedBrands };
    });
  };

  const toggleStore = (storeId: number) => {
    updateActiveFiltersWith((current) => {
      const selectedStores = new Set(current.selectedStores);
      if (selectedStores.has(storeId)) selectedStores.delete(storeId);
      else selectedStores.add(storeId);
      return { ...current, selectedStores };
    });
  };

  const toggleAttributeValue = (key: string, value: string) => {
    updateActiveFiltersWith((current) => {
      const selectedAttributes = { ...current.selectedAttributes };
      if (selectedAttributes[key] === value) delete selectedAttributes[key];
      else selectedAttributes[key] = value;
      return { ...current, selectedAttributes };
    });
  };

  /**
   * Limpia los filtros aplicados y los borradores de precio para devolver la
   * búsqueda a su estado inicial. No recibe argumentos ni devuelve nada.
   */
  const clearFilters = () => {
    setPriceMinDraft("");
    setPriceMaxDraft("");
    updateActiveFilters({ ...emptyFilters });
    if (showCategoryFilter) {
      selectCategory(null);
      setSelectedCategorySlug(null);
    }
  };

  /**
   * Pinta un nodo del árbol NV (NV1, NV2 o NV3) y sus hijos. Seleccionarlo
   * filtra la búsqueda con `category=<slug>`, que el backend resuelve con
   * todos sus descendientes.
   * @param node Nodo del árbol a pintar.
   * @param depth Profundidad para la sangría.
   * @returns El bloque del nodo con sus hijos.
   */
  const renderCategoryOption = (
    node: ProductCategorySection | ProductCategoryGroup | ProductCategoryLeaf,
    depth = 0,
  ) => {
    const selected = selectedCategorySlug === node.slug && selectedCategory === null;
    return (
      <div key={node.slug}>
        <button
          type="button"
          aria-pressed={selected}
          onClick={() => selectDynamicCategory(node.slug)}
          style={{ marginLeft: `${depth * 12}px` }}
          className={`flex w-[calc(100%-0.75rem)] items-center justify-between rounded-lg px-2.5 py-2 text-left text-sm transition-colors ${
            selected
              ? "bg-prime-muted font-semibold text-prime"
              : "text-muted-2 hover:bg-surface-2 hover:text-text"
          }`}
        >
          {node.name}
          {selected && <span aria-hidden="true">✓</span>}
        </button>
        {"children" in node && node.children.map((child) => renderCategoryOption(child, depth + 1))}
      </div>
    );
  };
  const activeFilterCount =
    selectedBrands.size +
    selectedStores.size +
    Object.keys(selectedAttributes).length +
    Object.keys(attributeRanges).filter(
      (key) => attributeRanges[key].min || attributeRanges[key].max,
    ).length +
    Number(Boolean(priceMin || priceMax)) +
    Number(availableOnly) +
    Number(showCategoryFilter && (selectedCategory !== null || selectedCategorySlug !== null));
  const priceFiltersChanged = priceMinDraft !== priceMin || priceMaxDraft !== priceMax;

  const renderFilters = () => (
    <div className="space-y-1">
      {showCategoryFilter && (
        <details className="group border-b border-bd pb-3" open={isGroupedProductsView}>
          <summary className="flex cursor-pointer list-none items-center justify-between py-3 text-sm font-semibold text-text">
            Categorías
            <span className="text-muted transition-transform group-open:rotate-180">⌄</span>
          </summary>
          <div className="space-y-1 pb-2">
            <button
              type="button"
              onClick={() => selectCategory(null)}
              aria-pressed={selectedCategory === null && selectedCategorySlug === null}
              className={`flex w-full items-center justify-between rounded-lg px-2.5 py-2 text-left text-sm transition-colors ${
                selectedCategory === null && selectedCategorySlug === null
                  ? "bg-prime-muted font-semibold text-prime"
                  : "text-muted-2 hover:bg-surface-2 hover:text-text"
              }`}
            >
              {isGroupedProductsView ? `Todo ${groupCategoryLabel}` : "Todas las categorías"}
              {selectedCategory === null && selectedCategorySlug === null && (
                <span aria-hidden="true">✓</span>
              )}
            </button>
            {PRODUCT_CATEGORY_TREE.map((root) => renderCategoryOption(root))}
          </div>
          {availableDynamicCategories.length > 0 && (
            <label className="mt-2 block text-xs font-medium text-muted">
              Atributos específicos
              <select
                value={attributeCategorySlug}
                onChange={(event) => selectDynamicCategory(event.target.value)}
                className="mt-1.5 w-full rounded-lg border border-bd bg-surface-2 px-2.5 py-2 text-sm text-text focus:border-prime focus:outline-none"
              >
                <option value="">Sin atributos específicos</option>
                {availableDynamicCategories.map((item) => (
                  <option key={item.slug} value={item.slug}>
                    {item.name}
                  </option>
                ))}
              </select>
            </label>
          )}
          {categoryError && <p className="mt-2 text-xs text-warn">{categoryError}</p>}
        </details>
      )}
      {categoryError && !showCategoryFilter && <p className="text-xs text-warn">{categoryError}</p>}
      <details className="group border-b border-bd pb-3" open>
        <summary className="flex cursor-pointer list-none items-center justify-between py-3 text-sm font-semibold text-text">
          Precio <span className="text-muted transition-transform group-open:rotate-180">⌄</span>
        </summary>
        <div className="flex gap-2 pb-2 pt-1">
          <label className="min-w-0 flex-1 text-xs text-muted">
            Mínimo
            <input
              type="number"
              min="0"
              value={priceMinDraft}
              onChange={(event) => setPriceMinDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  applyPriceFilters();
                }
              }}
              placeholder="$0"
              className="mt-1 w-full rounded-lg border border-bd bg-surface-2 px-2.5 py-2 text-sm text-text placeholder:text-muted focus:border-prime focus:outline-none"
            />
          </label>
          <label className="min-w-0 flex-1 text-xs text-muted">
            Máximo
            <input
              type="number"
              min="0"
              value={priceMaxDraft}
              onChange={(event) => setPriceMaxDraft(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === "Enter") {
                  event.preventDefault();
                  applyPriceFilters();
                }
              }}
              placeholder="Sin límite"
              className="mt-1 w-full rounded-lg border border-bd bg-surface-2 px-2.5 py-2 text-sm text-text placeholder:text-muted focus:border-prime focus:outline-none"
            />
          </label>
        </div>
        <button
          type="button"
          onClick={applyPriceFilters}
          disabled={!priceFiltersChanged}
          className="mb-2 w-full rounded-lg bg-gradient-to-r from-prime to-prime-dark px-3 py-2 text-sm font-semibold text-[#181714] transition hover:brightness-110 disabled:cursor-not-allowed disabled:opacity-50"
        >
          Aplicar precio
        </button>
      </details>

      <details className="group border-b border-bd pb-3">
        <summary className="flex cursor-pointer list-none items-center justify-between py-3 text-sm font-semibold text-text">
          Marca <span className="text-muted transition-transform group-open:rotate-180">⌄</span>
        </summary>
        <div className="max-h-48 space-y-2 overflow-y-auto pb-2 pt-1">
          {brands.length ? (
            brands.map((brand) => (
              <label
                key={brand.id}
                className="flex cursor-pointer items-center gap-2 text-sm text-muted-2"
              >
                <input
                  type="checkbox"
                  checked={selectedBrands.has(brand.id)}
                  onChange={() => toggleBrand(brand.id)}
                  className="accent-prime"
                />
                <span>
                  {brand.name} ({brand.count})
                </span>
              </label>
            ))
          ) : (
            <p className="text-xs text-muted">No hay marcas disponibles para estos resultados.</p>
          )}
        </div>
      </details>

      <details className="group border-b border-bd pb-3">
        <summary className="flex cursor-pointer list-none items-center justify-between py-3 text-sm font-semibold text-text">
          Tienda <span className="text-muted transition-transform group-open:rotate-180">⌄</span>
        </summary>
        {facets?.stores.length ? (
          <div className="max-h-48 space-y-2 overflow-y-auto pb-2 pt-1">
            {facets.stores.map((store) => (
              <label
                key={store.id}
                className="flex cursor-pointer items-center gap-2 text-sm text-muted-2"
              >
                <input
                  type="checkbox"
                  checked={selectedStores.has(store.id)}
                  onChange={() => toggleStore(store.id)}
                  className="accent-prime"
                />
                <span>
                  {store.name} ({store.count})
                </span>
              </label>
            ))}
          </div>
        ) : (
          <p className="pb-2 text-xs text-muted">
            No hay tiendas con ofertas disponibles para estos resultados.
          </p>
        )}
      </details>

      <details className="group border-b border-bd pb-3">
        <summary className="flex cursor-pointer list-none items-center justify-between py-3 text-sm font-semibold text-text">
          Disponibilidad{" "}
          <span className="text-muted transition-transform group-open:rotate-180">⌄</span>
        </summary>
        <label className="flex cursor-pointer items-center gap-2 pb-2 pt-1 text-sm text-muted-2">
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

      {attributeFacets.map((attribute) => {
        const numeric = attribute.dataType === "integer" || attribute.dataType === "decimal";
        const range = attributeRanges[attribute.key] ?? { min: "", max: "" };
        return (
          <details key={attribute.key} className="group border-b border-bd pb-3">
            <summary className="flex cursor-pointer list-none items-center justify-between py-3 text-sm font-semibold text-text">
              {attribute.label}
              {attribute.unit ? ` (${attribute.unit})` : ""}
              <span className="text-muted transition-transform group-open:rotate-180">⌄</span>
            </summary>
            {numeric ? (
              <div className="flex gap-2 pb-2 pt-1">
                {(["min", "max"] as const).map((bound) => (
                  <label key={bound} className="min-w-0 flex-1 text-xs text-muted">
                    {bound === "min" ? "Mínimo" : "Máximo"}
                    <input
                      type="number"
                      step={attribute.dataType === "integer" ? "1" : "any"}
                      value={range[bound]}
                      min={attribute.min}
                      max={attribute.max}
                      onChange={(event) =>
                        updateActiveFiltersWith((current) => ({
                          ...current,
                          attributeRanges: {
                            ...current.attributeRanges,
                            [attribute.key]: {
                              ...(current.attributeRanges[attribute.key] ?? { min: "", max: "" }),
                              [bound]: event.target.value,
                            },
                          },
                        }))
                      }
                      className="mt-1 w-full rounded-lg border border-bd bg-surface-2 px-2 py-2 text-sm text-text focus:border-prime focus:outline-none"
                    />
                  </label>
                ))}
              </div>
            ) : (
              <div className="max-h-64 space-y-1.5 overflow-y-auto pb-2 pt-1">
                {(attribute.values ?? []).map(({ value, count }) => {
                  const stringValue = String(value);
                  return (
                    <label
                      key={stringValue}
                      className="flex cursor-pointer items-start gap-2 text-xs text-muted-2"
                    >
                      <input
                        type="checkbox"
                        checked={selectedAttributes[attribute.key] === stringValue}
                        onChange={() => toggleAttributeValue(attribute.key, stringValue)}
                        className="mt-0.5 accent-prime"
                      />
                      <span>
                        {stringValue} ({count})
                      </span>
                    </label>
                  );
                })}
              </div>
            )}
          </details>
        );
      })}

      {(selectedBrands.size > 0 ||
        selectedStores.size > 0 ||
        Object.keys(selectedAttributes).length > 0 ||
        Object.keys(attributeRanges).some(
          (key) => attributeRanges[key].min || attributeRanges[key].max,
        ) ||
        priceMin ||
        priceMax ||
        availableOnly ||
        (showCategoryFilter && (selectedCategory !== null || selectedCategorySlug !== null))) && (
        <button
          onClick={clearFilters}
          className="pt-3 text-xs font-semibold text-prime hover:text-prime-dark"
        >
          Limpiar filtros
        </button>
      )}
    </div>
  );
  const categoryPath = selectedCategory === null ? [] : getCategoryPath(selectedCategory);
  const routeCategoryPath = routeCategorySlug
    ? (findProductCategoryPath(routeCategorySlug) ?? [])
    : [];
  const breadcrumbItems: Array<{ label: string; onClick?: () => void }> = [
    { label: "Inicio", onClick: () => navigate({ id: "home" }) },
    {
      label: "Productos",
      onClick: () => navigate({ id: "product-categories" }),
    },
  ];
  if (activeProductGroup !== "all") {
    breadcrumbItems.push({
      label: groupCategoryLabel,
      ...(selectedCategory !== null || selectedCategorySlug !== null
        ? {
            onClick: () =>
              navigate({
                id: "search-products",
                query: "",
                ...(groupScopeSlug
                  ? { category: groupScopeSlug, categoryContext: groupCategoryLabel }
                  : { productGroup: activeProductGroup }),
              }),
          }
        : {}),
    });
  }
  for (const [index, node] of routeCategoryPath.entries()) {
    breadcrumbItems.push({
      label: node.name,
      ...(index < routeCategoryPath.length - 1
        ? {
            onClick: () =>
              navigate({
                id: "search-products",
                query: "",
                category: node.slug,
                categoryContext: node.name,
                resetFilters: true,
              }),
          }
        : {}),
    });
  }
  for (const [index, categoryItem] of categoryPath.entries()) {
    breadcrumbItems.push({
      label: categoryItem.name,
      ...(index < categoryPath.length - 1 ||
      (Boolean(categoryContext) &&
        normalizeCategoryKey(categoryContext ?? "") !== normalizeCategoryKey(categoryItem.name))
        ? {
            onClick: () =>
              navigate({
                id: "search-products",
                query: "",
                categoryId: categoryItem.id,
                ...(activeProductGroup !== "all" ? { productGroup: activeProductGroup } : {}),
              }),
          }
        : {}),
    });
  }
  if (
    categoryContext &&
    !breadcrumbItems.some(
      (item) => normalizeCategoryKey(item.label) === normalizeCategoryKey(categoryContext),
    )
  ) {
    breadcrumbItems.push({ label: categoryContext });
  }
  if (
    selectedDynamicCategoryName &&
    !breadcrumbItems.some(
      (item) =>
        normalizeCategoryKey(item.label) === normalizeCategoryKey(selectedDynamicCategoryName),
    )
  ) {
    breadcrumbItems.push({ label: selectedDynamicCategoryName });
  }
  if (
    category &&
    !routeCategorySlug &&
    !breadcrumbItems.some(
      (item) => normalizeCategoryKey(item.label) === normalizeCategoryKey(category),
    )
  ) {
    breadcrumbItems.push({ label: category });
  }
  if (query && !categoryContext && !category) {
    breadcrumbItems.push({ label: `"${query}"` });
  }

  if (searching && !loadedSearchKey) {
    return (
      <div
        className="mx-auto max-w-7xl px-4 py-8 sm:py-10"
        role="status"
        aria-label="Cargando productos"
      >
        <Skeleton className="mb-6 h-4 w-44" />
        <div className="mb-6 border-b border-bd pb-5">
          <Skeleton className="mb-3 h-8 w-64" />
          <Skeleton className="h-4 w-80 max-w-full" />
        </div>
        <div className="grid gap-6 lg:grid-cols-[15rem_minmax(0,1fr)]">
          <aside className="hidden space-y-3 lg:block">
            {[0, 1, 2, 3, 4].map((item) => (
              <Skeleton key={item} className="h-12 w-full" />
            ))}
          </aside>
          <ProductSkeletonGrid count={6} />
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:py-10">
      {error && errorOpen && !searching && (
        <div className="fixed inset-0 z-[60] flex items-center justify-center p-4">
          <button
            type="button"
            aria-label="Cerrar error"
            className="absolute inset-0 bg-black/60 backdrop-blur-sm"
            onClick={() => setErrorOpen(false)}
          />
          <section
            role="alertdialog"
            aria-modal="true"
            aria-labelledby="search-error-title"
            className="relative w-full max-w-md rounded-2xl border border-bd bg-surface p-6 shadow-2xl"
          >
            <div className="mb-4 flex items-start justify-between gap-4">
              <h2 id="search-error-title" className="text-lg font-semibold text-text">
                No se pudo completar la búsqueda
              </h2>
              <button
                type="button"
                aria-label="Cerrar"
                onClick={() => setErrorOpen(false)}
                className="rounded-full border border-bd p-2 text-muted transition-colors hover:text-text"
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
            <p className="text-sm text-muted" role="alert">
              {error.replace(/^Search API:\s*/, "")}
            </p>
            <div className="mt-6 flex justify-end gap-2">
              <button
                type="button"
                onClick={() => setErrorOpen(false)}
                className="rounded-lg border border-bd px-4 py-2 text-sm font-medium text-muted-2 transition-colors hover:text-text"
              >
                Cerrar
              </button>
              {errorSource !== "price" && (
                <button
                  type="button"
                  onClick={() => {
                    setError(null);
                    setErrorOpen(false);
                    setErrorSource(null);
                    setSearchAttempt((attempt) => attempt + 1);
                  }}
                  className="rounded-lg bg-gradient-to-r from-prime to-prime-dark px-4 py-2 text-sm font-semibold text-[#181714] transition hover:brightness-110"
                >
                  Reintentar
                </button>
              )}
            </div>
          </section>
        </div>
      )}
      <Breadcrumb items={breadcrumbItems} />

      <div className="mb-6 mt-5 flex flex-wrap items-end justify-between gap-4 border-b border-white/10 pb-5">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-100">
            {routeCategoryName ||
              category ||
              categoryContext ||
              selectedCategoryName ||
              selectedDynamicCategoryName ||
              (activeProductGroup !== "all"
                ? groupCategoryLabel
                : query
                  ? `Resultados para "${query}"`
                  : "Todos los productos")}
          </h1>
          <p className="mt-1.5 text-sm text-slate-400">
            Mostrando {paginated.length} productos de {resultTotal} resultados
          </p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <button
            type="button"
            disabled={searching}
            onClick={() => setSearchAttempt((attempt) => attempt + 1)}
            className="rounded-lg border border-bd bg-surface-2 px-3 py-2 text-sm text-muted-2 transition-colors hover:border-prime/60 hover:bg-prime-muted hover:text-prime disabled:opacity-50"
          >
            {searching ? "Actualizando..." : "Actualizar"}
          </button>
          {/* Mobile filter button */}
          <button
            onClick={() => setFiltersOpen(true)}
            className="inline-flex items-center gap-2 rounded-lg border border-bd bg-surface-2 px-3 py-2 text-sm text-muted-2 transition-colors hover:border-prime/60 hover:text-text lg:hidden"
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
            {activeFilterCount > 0 && (
              <span className="flex h-5 min-w-5 items-center justify-center rounded-full bg-prime-muted px-1.5 text-[10px] font-semibold text-prime">
                {activeFilterCount}
              </span>
            )}
          </button>

          {/* Sort */}
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as SortOption)}
            aria-label="Ordenar productos"
            className="rounded-lg border border-bd bg-surface-2 px-3 py-2 text-sm text-text transition-colors hover:border-prime/60 hover:bg-prime-muted hover:text-prime focus:border-prime focus:outline-none"
          >
            <option value="name">Relevancia</option>
            <option value="price_asc">Precio: menor a mayor</option>
            <option value="price_desc">Precio: mayor a menor</option>
            <option value="brand_asc">Marca: A a Z</option>
            <option value="brand_desc">Marca: Z a A</option>
            {attributeFacets.flatMap((attribute) => [
              <option key={`${attribute.key}-asc`} value={`attribute_asc:${attribute.key}`}>
                {attribute.label}: menor a mayor
              </option>,
              <option key={`${attribute.key}-desc`} value={`attribute_desc:${attribute.key}`}>
                {attribute.label}: mayor a menor
              </option>,
            ])}
          </select>

          <label className="flex items-center gap-2 text-sm text-muted-2">
            <span className="text-muted">Por página</span>
            <select
              aria-label="Cantidad de productos por página"
              value={pageSize}
              onChange={(event) => {
                onPageSizeChange(Number(event.target.value));
                setPage(1);
              }}
              className="rounded-lg border border-bd bg-surface-2 px-3 py-2 text-sm text-text transition-colors hover:border-prime/60 hover:bg-prime-muted hover:text-prime focus:border-prime focus:outline-none"
            >
              {[10, 20, 30, 40, 50].map((size) => (
                <option key={size} value={size}>
                  {size}
                </option>
              ))}
            </select>
          </label>
        </div>
      </div>

      <div className="flex gap-6">
        {/* Sidebar filters (desktop) */}
        <aside className="sticky top-24 hidden w-60 shrink-0 self-start rounded-2xl border border-bd bg-surface p-4 shadow-xl shadow-black/10 lg:block">
          <div className="mb-3 flex items-center justify-between">
            <h2 className="text-sm font-semibold text-text">Filtrar por</h2>
            {activeFilterCount > 0 && (
              <button
                type="button"
                onClick={clearFilters}
                className="text-xs font-medium text-prime transition-colors hover:text-prime-dark"
              >
                Limpiar
              </button>
            )}
          </div>
          {renderFilters()}
        </aside>

        {/* Mobile filter modal */}
        {filtersOpen && (
          <div className="fixed inset-0 z-50 lg:hidden">
            <button
              type="button"
              aria-label="Cerrar filtros"
              className="absolute inset-0 bg-black/60 backdrop-blur-sm"
              onClick={() => setFiltersOpen(false)}
            />
            <section
              role="dialog"
              aria-modal="true"
              aria-labelledby="mobile-filters-title"
              className="absolute bottom-0 right-0 top-0 w-80 max-w-[90vw] overflow-y-auto border-l border-bd bg-surface p-5 shadow-2xl"
            >
              <div className="mb-5 flex items-center justify-between">
                <div>
                  <h2 id="mobile-filters-title" className="text-base font-semibold text-text">
                    Filtros
                  </h2>
                  <p className="mt-1 text-xs text-muted">Ajusta los resultados a tu búsqueda</p>
                </div>
                <button
                  type="button"
                  aria-label="Cerrar filtros"
                  onClick={() => setFiltersOpen(false)}
                  className="rounded-full border border-bd p-2 text-muted transition-colors hover:text-text"
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
              <div className="sticky bottom-0 mt-5 flex gap-2 border-t border-bd bg-surface py-4">
                <button
                  type="button"
                  onClick={clearFilters}
                  className="flex-1 rounded-lg border border-bd px-3 py-2.5 text-sm font-medium text-muted-2 transition-colors hover:border-prime/60 hover:text-text"
                >
                  Limpiar
                </button>
                <button
                  type="button"
                  onClick={() => setFiltersOpen(false)}
                  className="flex-1 rounded-lg bg-prime px-3 py-2.5 text-sm font-semibold text-[#181714] transition hover:bg-prime-dark"
                >
                  Ver {resultTotal} resultados
                </button>
              </div>
            </section>
          </div>
        )}

        {/* Results */}
        <div className="flex-1 min-w-0">
          {searching ? (
            <ProductSkeletonGrid count={6} />
          ) : paginated.length === 0 ? (
            <>
              <EmptyState
                title={
                  resultTotal === 0
                    ? "No encontramos productos"
                    : "No hay coincidencias en esta página"
                }
                description={
                  resultTotal > 0
                    ? "Vuelve a la primera página para seguir explorando los resultados."
                    : query
                      ? `No encontramos productos para "${query}" con los filtros actuales. Prueba quitando alguno.`
                      : "No hay productos que coincidan con los filtros actuales. Prueba quitando alguno."
                }
                action={{
                  label:
                    resultTotal > 0
                      ? "Volver a la primera página"
                      : activeFilterCount > 0
                        ? "Limpiar filtros"
                        : "Ver categorías",
                  onClick:
                    resultTotal > 0
                      ? () => setPage(1)
                      : activeFilterCount > 0
                        ? clearFilters
                        : () => navigate({ id: "product-categories" }),
                }}
              />
              {resultTotal > pageSize && (
                <Pagination page={page} total={resultTotal} perPage={pageSize} onChange={setPage} />
              )}
            </>
          ) : (
            <>
              <div className="grid grid-cols-1 gap-4 xl:grid-cols-2">
                {paginated.map((p) => (
                  <ProductCard
                    key={p.id}
                    product={p}
                    navigate={navigate}
                    isFavorite={favorites.has(p.id)}
                    isComparing={productCompareList.has(p.id)}
                    cartProductIds={cartProductIds}
                    onAddToCart={onAddToCart}
                    onToggleFavorite={onToggleFavorite}
                    onToggleCompare={onToggleProductCompare}
                  />
                ))}
              </div>
              <Pagination page={page} total={resultTotal} perPage={pageSize} onChange={setPage} />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
