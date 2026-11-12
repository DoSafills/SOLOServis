import type {
  Product,
  Service,
  Store,
  StoreDetail,
  StoreLocation,
  StoreProduct,
} from "../../types";
import { parseAmount, toProduct, type ApiProduct } from "./products";
import { API_URLS, fetchServiceResponse, getServiceJson, getServiceResponse } from "./config";

interface ApiStore {
  id: string;
  name: string;
  logo?: string;
  rating?: number | string;
  reviewCount?: number | string;
  productCount?: number | string;
  reputation?: string;
  dispatchTime?: string;
  conditions?: string;
  website?: string;
}

interface ApiStoreProduct {
  id: string;
  offerId: number;
  name: string;
  brand: string;
  model: string;
  category: string;
  description: string;
  price: number | string | null;
  listPrice: number | string | null;
  currency: string;
  shippingCost: number | string | null;
  shippingFree: boolean;
  available: boolean;
  stock: number | string | null;
  condition: string;
  productUrl: string;
  image: string;
  lastUpdated?: string;
}

interface ApiStoreLocation {
  id: number;
  locationId: number;
  address: string;
  postalCode: string;
  country: string;
  region: string;
  city: string;
  commune: string;
}

interface ApiStoreDetail extends ApiStore {
  products: ApiStoreProduct[];
  locations: ApiStoreLocation[];
}

interface ApiListEnvelope<T> {
  value: T[];
  Count: number;
}

async function fetchJson<T>(baseUrl: string, path: string, apiName: string): Promise<T> {
  return getServiceJson<T>(`${baseUrl}${path}`, apiName);
}

async function fetchList<T>(baseUrl: string, path: string, apiName: string): Promise<T[]> {
  const response = await fetchJson<T[] | ApiListEnvelope<T>>(baseUrl, path, apiName);
  return Array.isArray(response) ? response : response.value;
}

const toStore = (store: ApiStore): Store => ({
  id: String(store.id),
  name: store.name ?? "",
  logo: store.logo ?? "",
  rating: parseAmount(store.rating),
  reviewCount: parseAmount(store.reviewCount, 0),
  productCount: parseAmount(store.productCount),
  reputation: store.reputation ?? "",
  dispatchTime: store.dispatchTime ?? "",
  conditions: store.conditions ?? "",
  website: store.website ?? "",
});

const toStoreProduct = (product: ApiStoreProduct): StoreProduct => ({
  id: product.id,
  offerId: product.offerId,
  name: product.name,
  brand: product.brand,
  model: product.model,
  category: product.category,
  description: product.description,
  price: parseAmount(product.price),
  listPrice: parseAmount(product.listPrice),
  currency: product.currency,
  shippingCost: parseAmount(product.shippingCost),
  shippingFree: product.shippingFree,
  available: product.available,
  stock: product.stock === null || product.stock === undefined ? null : Number(product.stock),
  condition: product.condition,
  productUrl: product.productUrl ?? "",
  image: product.image ?? "",
  lastUpdated: product.lastUpdated,
});

const toStoreLocation = (location: ApiStoreLocation): StoreLocation => ({
  id: location.id,
  locationId: location.locationId,
  address: location.address,
  postalCode: location.postalCode,
  country: location.country,
  region: location.region,
  city: location.city,
  commune: location.commune,
});

const toStoreDetail = (detail: ApiStoreDetail): StoreDetail => ({
  ...toStore(detail),
  products: (detail.products ?? []).map(toStoreProduct),
  locations: (detail.locations ?? []).map(toStoreLocation),
});

export { toProduct, type ApiProduct } from "./products";

export async function getProducts(categoryId?: number): Promise<Product[]> {
  const query = categoryId === undefined ? "" : `?category=${categoryId}`;
  const products = await fetchList<ApiProduct>(
    API_URLS.products,
    `/products${query}`,
    "Product API",
  );
  return products.map(toProduct);
}

export async function getAllProducts(): Promise<Product[]> {
  const limit = 100;
  const firstQuery = new URLSearchParams({ limit: String(limit), offset: "0" });
  const firstResponse = await getServiceResponse(
    `${API_URLS.products}/products?${firstQuery}`,
    "Product API",
  );
  const totalHeader = firstResponse.headers.get("X-Total-Count");
  const total = Number(totalHeader);
  if (totalHeader === null || !Number.isSafeInteger(total) || total < 0) {
    throw new Error("Product API: el total de productos devuelto no es válido.");
  }

  const firstPage: unknown = await firstResponse.json();
  if (!Array.isArray(firstPage)) {
    throw new Error("Product API: la respuesta de productos no tiene un formato válido.");
  }
  const pages = [firstPage as ApiProduct[]];
  const offsets = Array.from(
    { length: Math.ceil(Math.max(0, total - limit) / limit) },
    (_, index) => (index + 1) * limit,
  );
  const remainingPages = await Promise.all(
    offsets.map(async (offset) => {
      const query = new URLSearchParams({ limit: String(limit), offset: String(offset) });
      const response = await getServiceResponse(
        `${API_URLS.products}/products?${query}`,
        "Product API",
      );
      const items: unknown = await response.json();
      if (!Array.isArray(items)) {
        throw new Error("Product API: la respuesta de productos no tiene un formato válido.");
      }
      return items as ApiProduct[];
    }),
  );
  pages.push(...remainingPages);

  const allProducts = pages.flat();
  if (allProducts.length !== total) {
    throw new Error("Product API: no se pudieron cargar todos los productos del catálogo.");
  }
  return allProducts.map(toProduct);
}

export async function getProductById(id: string): Promise<Product | null> {
  const response = await fetchServiceResponse(
    `${API_URLS.products}/products/${encodeURIComponent(id)}`,
    "Product API",
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Product API: error HTTP ${response.status} al obtener el producto.`);
  }

  return toProduct((await response.json()) as ApiProduct);
}

export async function getServices(params?: { category?: string }): Promise<Service[]> {
  const query = new URLSearchParams(
    Object.entries(params ?? {}).filter(([, value]) => Boolean(value)) as [string, string][],
  ).toString();

  return fetchList<Service>(
    API_URLS.services,
    `/services${query ? `?${query}` : ""}`,
    "Service API",
  );
}

export async function getServiceById(id: string): Promise<Service | null> {
  const response = await fetchServiceResponse(
    `${API_URLS.services}/services/${encodeURIComponent(id)}`,
    "Service API",
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Service API: error HTTP ${response.status} al obtener el servicio.`);
  }

  return response.json() as Promise<Service>;
}

export async function getStores(): Promise<Store[]> {
  const stores = await fetchList<ApiStore>(API_URLS.stores, "/stores", "Store API");
  return stores.map(toStore);
}

export async function getStoreById(id: string): Promise<StoreDetail | null> {
  const response = await fetchServiceResponse(
    `${API_URLS.stores}/stores/${encodeURIComponent(id)}`,
    "Store API",
  );

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Store API: error HTTP ${response.status} al obtener la tienda.`);
  }

  return toStoreDetail((await response.json()) as ApiStoreDetail);
}
