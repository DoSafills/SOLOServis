import { API_URLS, getJson } from "./config";

export interface ApiProduct {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  description: string;
  rating: number;
  reviewCount: number;
}

export async function getProducts(): Promise<ApiProduct[]> {
  return getJson<ApiProduct[]>(`${API_URLS.products}/products`);
}