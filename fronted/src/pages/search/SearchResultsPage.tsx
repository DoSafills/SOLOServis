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
import { Breadcrumb, EmptyState, Pagination } from "../../components/common/ui";
import { getProductCategoryGroup } from "../../constants/productCatalog";

interface Props {
  query: string;
  category?: string;
  categoryContext?: string;
  productGroup?: string;
  categoryId?: number;
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
  const [sort, setSort] = useState<SortOption>("name");
  const activeProductGroup = productGroup ?? "all";
  const [filtersByGroup, setFiltersByGroup] = useState<Record<string, ProductFilterState>>({});
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(1);
  const [products, setProducts] = useState<Product[]>([]);
  const [resultTotal, setResultTotal] = useState(0);
  const [facets, setFacets] = useState<SearchFacets | null>(null);
  const [loading, setLoading] = useState(true);
  const [loadedSearchKey, setLoadedSearchKey] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [categoryError, setCategoryError] = useState<string | null>(null);
  const [categories, setCategories] = useState<ApiProductCategory[]>([]);
  const [categoriesLoaded, setCategoriesLoaded] = useState(false);
  const [dynamicCategories, setDynamicCategories] = useState<ProductCategory[]>([]);
  const [selectedCategory, setSelectedCategory] = useState<number | null>(categoryId ?? null);
  const [selectedCategorySlug, setSelectedCategorySlug] = useState<string | null>(null);
  const [searchAttempt, setSearchAttempt] = useState(0);
  const hasCategoryContext =
    Boolean(category) || Boolean(categoryContext) || categoryId !== undefined;
  const isGroupedProductsView =
    activeProductGroup !== "all" &&
    !category &&
    categoryId === undefined &&
    selectedCategory === null &&
    selectedCategorySlug === null;
  const showCategoryFilter = !hasCategoryContext || isGroupedProductsView;
  const groupCategoryLabel =
    activeProductGroup === "home"
      ? "Hogar"
      : activeProductGroup === "technology"
        ? "Tecnología"
        : "Otras categorías";
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

  const rootCategories = useMemo(
    () => categories.filter((categoryItem) => categoryItem.parentId === null),
    [categories],
  );
  const productGroupCategoryIds = useMemo(
    () =>
      rootCategories
        .filter((categoryItem) => getProductCategoryGroup(categoryItem.name) === activeProductGroup)
        .map((categoryItem) => categoryItem.id),
    [activeProductGroup, rootCategories],
  );

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
    const categoryFromRoute = categories.find(
      (item) => normalizeCategoryKey(item.name) === normalizeCategoryKey(category ?? ""),
    )?.id;
    return {
      query,
      category: selectedCategorySlug ?? selectedCategory ?? categoryId ?? categoryFromRoute,
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
    page,
    pageSize,
    query,
    selectedCategory,
    selectedCategorySlug,
    sort,
  ]);
  const searchRequestKey = JSON.stringify([
    searchParams,
    searchAttempt,
    productGroupCategoryIds,
    isGroupedProductsView,
  ]);
  const searching =
    loading ||
    loadedSearchKey !== searchRequestKey ||
    (isGroupedProductsView && !categoriesLoaded) ||
    (Boolean(category) && !categoriesLoaded);

  useEffect(() => {
    if ((isGroupedProductsView || category) && !categoriesLoaded) return;
    let cancelled = false;
    const loadResults = async () => {
      if (
        category &&
        !categories.some(
          (item) => normalizeCategoryKey(item.name) === normalizeCategoryKey(category),
        )
      ) {
        throw new Error(
          categoryError ?? `No se encontró la categoría "${category}" en el catálogo.`,
        );
      }
      return Promise.all(
        isGroupedProductsView
          ? [
              searchProductsAcrossCategories(searchParams, productGroupCategoryIds),
              getSearchFacetsAcrossCategories(searchParams, productGroupCategoryIds),
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
        }
      })
      .catch((err: unknown) => {
        if (!cancelled) setError(err instanceof Error ? err.message : "Error al cargar productos.");
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
    productGroupCategoryIds,
    isGroupedProductsView,
    groupCategoryLabel,
    page,
    pageSize,
    searchParams,
    searchAttempt,
    searchRequestKey,
  ]);

  const subcategoriesOf = (parentId: number) =>
    categories.filter((cat) => cat.parentId === parentId);
  const selectedCategoryName = categories.find((cat) => cat.id === selectedCategory)?.name ?? "";
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
  const getCategoryDescendants = (categoryIdValue: number) => {
    const descendants = new Set([categoryIdValue]);
    const pending = [categoryIdValue];
    while (pending.length > 0) {
      const current = pending.pop();
      if (current === undefined) continue;
      for (const child of subcategoriesOf(current)) {
        if (!descendants.has(child.id)) {
          descendants.add(child.id);
          pending.push(child.id);
        }
      }
    }
    return descendants;
  };
  const visibleRootCategories =
    activeProductGroup !== "all"
      ? rootCategories.filter(
          (categoryItem) => getProductCategoryGroup(categoryItem.name) === activeProductGroup,
        )
      : rootCategories;
  const relevantCategoryNames = new Set(
    categories
      .filter((item) => {
        if (selectedCategory !== null) {
          return getCategoryDescendants(selectedCategory).has(item.id);
        }
        return (
          activeProductGroup === "all" ||
          productGroupCategoryIds.some((rootId) => getCategoryDescendants(rootId).has(item.id))
        );
      })
      .flatMap((item) => [item.name, item.name.replace(/\s+/g, "")])
      .map(normalizeCategoryKey),
  );
  const availableDynamicCategories = dynamicCategories.filter(
    (item) =>
      selectedCategorySlug !== null ||
      (selectedCategory === null && activeProductGroup === "all") ||
      relevantCategoryNames.has(normalizeCategoryKey(item.name)) ||
      relevantCategoryNames.has(normalizeCategoryKey(item.slug)),
  );
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

  const clearFilters = () => {
    updateActiveFilters({ ...emptyFilters });
    if (showCategoryFilter) {
      selectCategory(null);
      setSelectedCategorySlug(null);
    }
  };

  const renderCategoryOption = (categoryItem: ApiProductCategory, depth = 0) => (
    <div key={categoryItem.id}>
      <button
        type="button"
        aria-pressed={selectedCategory === categoryItem.id}
        onClick={() => selectCategory(categoryItem.id)}
        style={{ marginLeft: `${depth * 12}px` }}
        className={`flex w-[calc(100%-0.75rem)] items-center justify-between rounded-lg px-2.5 py-2 text-left text-sm transition-colors ${
          selectedCategory === categoryItem.id
            ? "bg-prime-muted font-semibold text-prime"
            : "text-muted-2 hover:bg-surface-2 hover:text-text"
        }`}
      >
        {categoryItem.name}
        {selectedCategory === categoryItem.id && <span aria-hidden="true">✓</span>}
      </button>
      {subcategoriesOf(categoryItem.id).map((child) => renderCategoryOption(child, depth + 1))}
    </div>
  );
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
            {visibleRootCategories.map((root) => renderCategoryOption(root))}
          </div>
          {availableDynamicCategories.length > 0 && (
            <label className="mt-2 block text-xs font-medium text-muted">
              Atributos específicos
              <select
                value={selectedCategorySlug ?? ""}
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
              value={priceMin}
              onChange={(event) => {
                updateActiveFilters({ priceMin: event.target.value });
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
              value={priceMax}
              onChange={(event) => {
                updateActiveFilters({ priceMax: event.target.value });
              }}
              placeholder="Sin límite"
              className="mt-1 w-full rounded-lg border border-bd bg-surface-2 px-2.5 py-2 text-sm text-text placeholder:text-muted focus:border-prime focus:outline-none"
            />
          </label>
        </div>
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
                productGroup: activeProductGroup,
              }),
          }
        : {}),
    });
  }
  for (const [index, categoryItem] of categoryPath.entries()) {
    if (
      activeProductGroup !== "all" &&
      index === 0 &&
      getProductCategoryGroup(categoryItem.name) === activeProductGroup
    ) {
      continue;
    }
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
    !breadcrumbItems.some(
      (item) => normalizeCategoryKey(item.label) === normalizeCategoryKey(category),
    )
  ) {
    breadcrumbItems.push({ label: category });
  }
  if (query && !categoryContext && !category) {
    breadcrumbItems.push({ label: `"${query}"` });
  }

  if (searching) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="text-sm text-slate-400" role="status">
          Cargando productos...
        </p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="mb-4 text-sm text-amber-200" role="alert">
          {error}
        </p>
        <button
          onClick={() => setSearchAttempt((attempt) => attempt + 1)}
          className="rounded-lg bg-gradient-to-r from-cyan-600 to-violet-600 px-5 py-2 text-sm font-semibold text-white transition hover:brightness-110"
        >
          Reintentar
        </button>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:py-10">
      <Breadcrumb items={breadcrumbItems} />

      <div className="mb-6 mt-5 flex flex-wrap items-end justify-between gap-4 border-b border-white/10 pb-5">
        <div>
          <h1 className="text-2xl font-semibold tracking-tight text-slate-100">
            {category ||
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

          {/* Compare button */}
          {productCompareList.size >= 2 && (
            <button
              onClick={() =>
                navigate({
                  id: "product-comparison",
                  productIds: [...productCompareList],
                })
              }
              className="rounded-lg bg-gradient-to-r from-cyan-600 to-violet-600 px-4 py-2 text-sm font-semibold text-white transition hover:brightness-110"
            >
              Comparar {productCompareList.size} productos
            </button>
          )}
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
          {paginated.length === 0 ? (
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
