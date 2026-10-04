import type {
  Product,
  Service,
  Store,
  StoreDetail,
  StoreLocation,
  StoreProduct,
} from "../../types";
import { parseAmount, toProduct, type ApiProduct } from "./products";

const PRODUCT_API_URL = import.meta.env.VITE_PRODUCT_API_URL ?? "http://localhost:8081";
const SERVICE_API_URL = import.meta.env.VITE_SERVICE_API_URL ?? "http://localhost:8083";
const STORE_API_URL = import.meta.env.VITE_STORE_API_URL ?? "http://localhost:8082";

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

async function fetchJson<T>(baseUrl: string, path: string): Promise<T> {
  const response = await fetch(`${baseUrl}${path}`);

  if (!response.ok) {
    throw new Error(`Error ${response.status} al consultar ${path}`);
  }

  return response.json() as Promise<T>;
}

async function fetchList<T>(baseUrl: string, path: string): Promise<T[]> {
  const response = await fetchJson<T[] | ApiListEnvelope<T>>(baseUrl, path);
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

export async function getProducts(): Promise<Product[]> {
  const products = await fetchList<ApiProduct>(PRODUCT_API_URL, "/products");
  return products.map(toProduct);
}

export async function getProductById(id: string): Promise<Product | null> {
  const response = await fetch(`${PRODUCT_API_URL}/products/${id}`);

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Error ${response.status} al obtener el producto`);
  }

  return toProduct((await response.json()) as ApiProduct);
}

export async function getServices(params?: { category?: string }): Promise<Service[]> {
  const query = new URLSearchParams(
    Object.entries(params ?? {}).filter(([, value]) => Boolean(value)) as [string, string][],
  ).toString();

  return fetchList<Service>(SERVICE_API_URL, `/services${query ? `?${query}` : ""}`);
}

export async function getServiceById(id: string): Promise<Service | null> {
  const response = await fetch(`${SERVICE_API_URL}/services/${id}`);

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Error ${response.status} al obtener el servicio`);
  }

  return response.json() as Promise<Service>;
}

export async function getStores(): Promise<Store[]> {
  const stores = await fetchList<ApiStore>(STORE_API_URL, "/stores");
  return stores.map(toStore);
}

export async function getStoreById(id: string): Promise<StoreDetail | null> {
  const response = await fetch(`${STORE_API_URL}/stores/${id}`);

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Error ${response.status} al obtener la tienda`);
  }

  return toStoreDetail((await response.json()) as ApiStoreDetail);
}
