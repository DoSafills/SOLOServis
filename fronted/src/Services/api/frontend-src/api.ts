import type { Product, Service, Store } from '../types'

const API_BASE = import.meta.env.VITE_API_URL ?? 'http://localhost:4000/api'

async function fetchJson<T>(path: string): Promise<T> {
  const res = await fetch(`${API_BASE}${path}`)
  if (!res.ok) {
    throw new Error(`Error ${res.status} al consultar ${path}`)
  }
  return res.json() as Promise<T>
}

// ==================== PRODUCTOS ====================
export async function getProducts(params?: { category?: string; search?: string }): Promise<Product[]> {
  const query = new URLSearchParams(
    Object.entries(params ?? {}).filter(([, v]) => Boolean(v)) as [string, string][]
  ).toString()
  return fetchJson<Product[]>(`/products${query ? `?${query}` : ''}`)
}

export async function getProductById(id: string): Promise<Product | null> {
  const res = await fetch(`${API_BASE}/products/${id}`)
  if (res.status === 404) return null
  if (!res.ok) throw new Error('Error al obtener el producto')
  return res.json()
}

// ==================== SERVICIOS ====================
export async function getServices(params?: { category?: string }): Promise<Service[]> {
  const query = new URLSearchParams(
    Object.entries(params ?? {}).filter(([, v]) => Boolean(v)) as [string, string][]
  ).toString()
  return fetchJson<Service[]>(`/services${query ? `?${query}` : ''}`)
}

export async function getServiceById(id: string): Promise<Service | null> {
  const res = await fetch(`${API_BASE}/services/${id}`)
  if (res.status === 404) return null
  if (!res.ok) throw new Error('Error al obtener el servicio')
  return res.json()
}

// ==================== TIENDAS ====================
export async function getStores(): Promise<Store[]> {
  return fetchJson<Store[]>('/stores')
}

// ==================== HELPERS PUROS (idénticos a los del mock, no requieren red) ====================
export const formatPrice = (price: number): string => `$${price.toLocaleString('es-CL')}`

export const getMinPrice = (product: Product): number =>
  Math.min(...product.offers.filter((o) => o.available).map((o) => o.price))

export const getMinOffer = (product: Product) =>
  product.offers.filter((o) => o.available).sort((a, b) => a.price - b.price)[0]

export const getAvailableStoreCount = (product: Product): number =>
  product.offers.filter((o) => o.available).length
