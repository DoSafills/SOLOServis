import { API_URLS, getJson } from "./config";

export interface ApiProduct {
  id: string;
  name: string;
  brand: string;
  model: string;
  categoryId: number;
  category: string;
  subcategory: string;
  description: string;
  rating: number;
  reviewCount: number;
}

export interface ApiProductCategory {
  id: number;
  /** null en las categorías raíz */
  parentId: number | null;
  name: string;
  description: string;
}

/** Filtrar por una categoría incluye los productos de sus subcategorías. */
export async function getProducts(categoryId?: number): Promise<ApiProduct[]> {
  const query = categoryId ? `?category=${categoryId}` : "";
  return getJson<ApiProduct[]>(`${API_URLS.products}/products${query}`);
}

export async function getCategories(): Promise<ApiProductCategory[]> {
  return getJson<ApiProductCategory[]>(`${API_URLS.products}/categories`);
}