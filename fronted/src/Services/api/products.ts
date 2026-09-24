import type { Product } from "../../types";

interface ApiImage {
  url: string;
  altText: string;
  sortOrder: number;
}

interface ApiOffer {
  storeId: number;
  storeName: string;
  price: string;
  shippingCost: string;
  shippingFree: boolean;
  available: boolean;
  productUrl: string;
}

export interface ApiProduct {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  description: string;
  rating: number;
  reviewCount: number;
  images?: ApiImage[];
  offers?: ApiOffer[];
}

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

const parseAmount = (value: string | undefined): number => {
  const amount = Number(value);
  return Number.isFinite(amount) ? amount : 0;
};

export const toProduct = (product: ApiProduct): Product => {
  const images = (product.images ?? []).map((image) => image.url).filter(Boolean);
  const offers = (product.offers ?? []).map((offer) => ({
    storeId: String(offer.storeId),
    storeName: offer.storeName,
    price: parseAmount(offer.price),
    available: offer.available,
    shipping: offer.shippingFree ? 0 : parseAmount(offer.shippingCost),
    url: offer.productUrl || undefined,
  }));

  return {
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    category: product.category,
    subcategory: "",
    image: images[0] ?? "",
    images,
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.model ? { Modelo: product.model } : {},
    offers,
    priceHistory: [],
    offerPriceHistory: [],
    tags: [],
  };
};

export async function getProducts(): Promise<ApiProduct[]> {
  const response = await fetch(`${API_URL}/products`);

  if (!response.ok) {
    throw new Error(`Failed to fetch products: ${response.status}`);
  }

  return response.json();
}

export async function getProductById(id: string): Promise<Product | null> {
  const response = await fetch(`${API_URL}/products/${id}`);
  if (response.status === 404) return null;
  if (!response.ok) {
    throw new Error(`Failed to fetch product: ${response.status}`);
  }
  return toProduct((await response.json()) as ApiProduct);
}