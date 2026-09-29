import type { Product, Service, Store } from "../../types";
import { getProducts as fetchProducts, toProduct } from "./products";

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

async function fetchJson<T>(path: string): Promise<T> {
  const response = await fetch(`${API_URL}${path}`);

  if (!response.ok) {
    throw new Error(`Error ${response.status} al consultar ${path}`);
  }

  return response.json() as Promise<T>;
}

//PRODUCTOS 
export async function getProducts(): Promise<Product[]> {
  const products = await fetchProducts();
  return products.map(toProduct);
}

export async function getProductById(id: string): Promise<Product | null> {
  const response = await fetch(`${API_URL}/products/${id}`);

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Error ${response.status} al obtener el producto`);
  }

  return toProduct(await response.json());
}

//SERVICIOS 

export async function getServices(params?: { category?: string }): Promise<Service[]> {
  const query = new URLSearchParams(
    Object.entries(params ?? {}).filter(([, value]) => Boolean(value)) as [string, string][],
  ).toString();

  return fetchJson<Service[]>(`/services${query ? `?${query}` : ""}`);
}

export async function getServiceById(id: string): Promise<Service | null> {
  const response = await fetch(`${API_URL}/services/${id}`);

  if (response.status === 404) {
    return null;
  }

  if (!response.ok) {
    throw new Error(`Error ${response.status} al obtener el servicio`);
  }

  return response.json() as Promise<Service>;
}

//TIENDAS 

export async function getStores(): Promise<Store[]> {
  return fetchJson<Store[]>("/stores");
}


