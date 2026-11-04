import { API_URLS, getServiceJson, getServiceResponse } from "./config";

export interface SearchAttributeFilter {
  value?: string;
  min?: string;
  max?: string;
}

export interface SearchProductsParams {
  query?: string;
  category?: string | number;
  brandIds?: number[];
  storeIds?: number[];
  minPrice?: string;
  maxPrice?: string;
  available?: boolean;
  attributes?: Record<string, SearchAttributeFilter>;
  sort?: string;
  limit?: number;
  offset?: number;
}

export interface SearchProductResult {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  categorySlug: string | null;
  imageUrl: string | null;
  bestPrice: string | null;
  offerCount: number;
  attributes: Record<string, string | number | boolean>;
}

export interface SearchFacets {
  total: number;
  price: { min: string | null; max: string | null };
  brands: Array<{ id: number; name: string; count: number }>;
  stores: Array<{ id: number; name: string; count: number }>;
  attributes: Array<{
    key: string;
    label: string;
    dataType: "string" | "integer" | "decimal" | "boolean" | "enum";
    unit: string | null;
    values?: Array<{ value: string | number | boolean; count: number }>;
    min?: number;
    max?: number;
  }>;
}

export interface SearchBrand {
  id: number;
  name: string;
  logoUrl: string | null;
}

function buildSearchQuery(params: SearchProductsParams): URLSearchParams {
  const query = new URLSearchParams();
  if (params.query?.trim()) query.set("q", params.query.trim());
  if (params.category !== undefined) query.set("category", String(params.category));
  if (params.brandIds?.length) query.set("brand", params.brandIds.join(","));
  if (params.storeIds?.length) query.set("store", params.storeIds.join(","));
  if (params.minPrice) query.set("minPrice", params.minPrice);
  if (params.maxPrice) query.set("maxPrice", params.maxPrice);
  if (params.available !== undefined) query.set("available", String(params.available));
  if (params.sort) query.set("sort", params.sort);
  if (params.limit !== undefined) query.set("limit", String(params.limit));
  if (params.offset !== undefined) query.set("offset", String(params.offset));

  for (const [key, filter] of Object.entries(params.attributes ?? {})) {
    if (filter.value !== undefined) query.set(`attr.${key}`, filter.value);
    if (filter.min !== undefined) query.set(`attr.${key}.min`, filter.min);
    if (filter.max !== undefined) query.set(`attr.${key}.max`, filter.max);
  }
  return query;
}

function fetchSearch<T>(path: string, params: SearchProductsParams): Promise<T> {
  const query = buildSearchQuery(params).toString();
  return getServiceJson<T>(`${API_URLS.search}${path}${query ? `?${query}` : ""}`, "Search API");
}

export async function searchProducts(
  params: SearchProductsParams,
): Promise<{ items: SearchProductResult[]; total: number }> {
  const query = buildSearchQuery(params).toString();
  const response = await getServiceResponse(
    `${API_URLS.search}/search/products?${query}`,
    "Search API",
  );
  const total = Number(response.headers.get("X-Total-Count") ?? 0);
  if (!Number.isSafeInteger(total) || total < 0)
    throw new Error("La búsqueda devolvió un total de resultados inválido.");
  return {
    items: (await response.json()) as SearchProductResult[],
    total,
  };
}

export function getSearchFacets(params: SearchProductsParams): Promise<SearchFacets> {
  return fetchSearch<SearchFacets>("/search/facets", params);
}

const SEARCH_PAGE_LIMIT = 100;

const compareValues = (
  first: string | number | boolean | null | undefined,
  second: string | number | boolean | null | undefined,
  direction: "asc" | "desc",
): number => {
  if (first === null || first === undefined) return second === null || second === undefined ? 0 : 1;
  if (second === null || second === undefined) return -1;

  const firstNumber = typeof first === "number" ? first : Number(first);
  const secondNumber = typeof second === "number" ? second : Number(second);
  const comparison =
    Number.isFinite(firstNumber) && Number.isFinite(secondNumber)
      ? firstNumber - secondNumber
      : String(first).localeCompare(String(second), "es", { numeric: true, sensitivity: "base" });
  return direction === "asc" ? comparison : -comparison;
};

const compareSearchResults = (params: SearchProductsParams) => {
  const sort = params.sort ?? "name";
  return (first: SearchProductResult, second: SearchProductResult) => {
    let comparison: number;
    if (sort === "price_asc" || sort === "price_desc") {
      comparison = compareValues(
        first.bestPrice,
        second.bestPrice,
        sort === "price_asc" ? "asc" : "desc",
      );
    } else if (sort === "brand_asc" || sort === "brand_desc") {
      comparison = compareValues(first.brand, second.brand, sort === "brand_asc" ? "asc" : "desc");
    } else if (sort.startsWith("attribute_asc:") || sort.startsWith("attribute_desc:")) {
      const direction = sort.startsWith("attribute_asc:") ? "asc" : "desc";
      const key = sort.slice(sort.indexOf(":") + 1);
      comparison = compareValues(first.attributes[key], second.attributes[key], direction);
    } else {
      comparison = first.name.localeCompare(second.name, "es", {
        numeric: true,
        sensitivity: "base",
      });
    }
    return comparison || first.id.localeCompare(second.id);
  };
};

async function searchCategoryTopResults(
  params: SearchProductsParams,
  categoryId: number,
  itemCount: number,
): Promise<{ items: SearchProductResult[]; total: number }> {
  const items: SearchProductResult[] = [];
  let total = 0;

  for (let offset = 0; offset < itemCount; offset += SEARCH_PAGE_LIMIT) {
    const limit = Math.min(SEARCH_PAGE_LIMIT, itemCount - offset);
    const result = await searchProducts({ ...params, category: categoryId, limit, offset });
    total = result.total;
    items.push(...result.items);
    if (items.length >= total || result.items.length < limit) break;
  }

  return { items, total };
}

export async function searchProductsAcrossCategories(
  params: SearchProductsParams,
  categoryIds: number[],
): Promise<{ items: SearchProductResult[]; total: number }> {
  const limit = params.limit ?? 20;
  const offset = params.offset ?? 0;
  const categoryResults = await Promise.all(
    [...new Set(categoryIds)].map((categoryId) =>
      searchCategoryTopResults(params, categoryId, offset + limit),
    ),
  );
  const total = categoryResults.reduce((sum, result) => sum + result.total, 0);
  const uniqueItems = new Map(
    categoryResults.flatMap((result) => result.items).map((item) => [item.id, item]),
  );
  const items = [...uniqueItems.values()]
    .sort(compareSearchResults(params))
    .slice(offset, offset + limit);
  return { items, total };
}

export async function getSearchFacetsAcrossCategories(
  params: SearchProductsParams,
  categoryIds: number[],
): Promise<SearchFacets> {
  const facets = await Promise.all(
    [...new Set(categoryIds)].map((categoryId) =>
      getSearchFacets({ ...params, category: categoryId }),
    ),
  );
  const brands = new Map<number, SearchFacets["brands"][number]>();
  const stores = new Map<number, SearchFacets["stores"][number]>();
  const prices = facets.flatMap((facet) =>
    [facet.price.min, facet.price.max]
      .filter((price): price is string => price !== null)
      .map(Number)
      .filter(Number.isFinite),
  );

  for (const facet of facets) {
    for (const brand of facet.brands) {
      const existing = brands.get(brand.id);
      brands.set(brand.id, { ...brand, count: brand.count + (existing?.count ?? 0) });
    }
    for (const store of facet.stores) {
      const existing = stores.get(store.id);
      stores.set(store.id, { ...store, count: store.count + (existing?.count ?? 0) });
    }
  }

  return {
    total: facets.reduce((sum, facet) => sum + facet.total, 0),
    price: {
      min: prices.length ? String(Math.min(...prices)) : null,
      max: prices.length ? String(Math.max(...prices)) : null,
    },
    brands: [...brands.values()].sort((first, second) =>
      first.name.localeCompare(second.name, "es"),
    ),
    stores: [...stores.values()].sort((first, second) =>
      first.name.localeCompare(second.name, "es"),
    ),
    attributes: [],
  };
}

export function getSearchBrands(): Promise<SearchBrand[]> {
  return getServiceJson<SearchBrand[]>(`${API_URLS.search}/search/brands`, "Search API");
}
