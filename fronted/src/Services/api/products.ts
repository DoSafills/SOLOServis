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
  images?: Array<{ url: string; altText: string; sortOrder?: number }>;
  specs?: Record<string, string>;
  priceHistory?: Array<{ date: string; price: number }>;
  offerPrice?: number;
  offerPriceHistory?: Array<{ date: string; price: number }>;
  tags?: string[];
}

export async function getProducts(): Promise<ApiProduct[]> {
  return getJson<ApiProduct[]>(`${API_URLS.products}/products`);
}