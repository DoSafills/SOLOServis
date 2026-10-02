import type { Product, PricePoint } from "../../types";

interface ApiImage {
  url: string;
  altText: string;
  sortOrder: number;
}

interface ApiOffer {
  storeId: string;
  storeName: string;
  price: string;
  listPrice: string;
  currency: string;
  shippingCost: string;
  shippingFree: boolean;
  available: boolean;
  stock: number | null;
  condition: string;
  productUrl: string;
}

interface ApiPricePoint {
  date: string;
  price: string;
}

export interface ApiProductCategory {
  id: number;
  /** null en las categorías raíz */
  parentId: number | null;
  name: string;
  description: string;
}

export interface ApiProductReview {
  author: string;
  /** El autor confirmó su email. No indica que haya comprado el producto. */
  authorVerified: boolean;
  rating: number;
  title: string;
  content: string;
  createdAt: string;
}

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
  specs?: Record<string, string>;
  images?: ApiImage[];
  offers?: ApiOffer[];
  priceHistory?: ApiPricePoint[];
  offerPriceHistory?: ApiPricePoint[];
}

const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

const parseAmount = (value: string | undefined): number => {
  const amount = Number(value);
  return Number.isFinite(amount) ? amount : 0;
};

const toPricePoints = (points: ApiPricePoint[] | undefined): PricePoint[] =>
  (points ?? []).map((point) => ({
    date: point.date,
    price: parseAmount(point.price),
  }));

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

  const priceHistory = toPricePoints(product.priceHistory);
  const offerPriceHistory = toPricePoints(product.offerPriceHistory);

  const currentOffer =
    offerPriceHistory.length > 0
      ? offerPriceHistory[offerPriceHistory.length - 1].price
      : undefined;

  return {
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    categoryId: product.categoryId,
    category: product.category,
    subcategory: product.subcategory ?? "",
    image: images[0] ?? "",
    images,
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.specs ?? {},
    offers,
    priceHistory,
    offerPriceHistory,
    offerPrice: currentOffer,
    tags: [],
  };
};

/** Filtrar por una categoría incluye los productos de sus subcategorías. */
export async function getProducts(categoryId?: number): Promise<ApiProduct[]> {
  const query = categoryId ? `?category=${categoryId}` : "";
  const response = await fetch(`${API_URL}/products${query}`);

  if (!response.ok) {
    throw new Error(`Failed to fetch products: ${response.status}`);
  }

  return response.json();
}

export async function getCategories(): Promise<ApiProductCategory[]> {
  const response = await fetch(`${API_URL}/categories`);

  if (!response.ok) {
    throw new Error(`Failed to fetch categories: ${response.status}`);
  }

  return response.json();
}

export async function getProductReviews(id: string): Promise<ApiProductReview[]> {
  const response = await fetch(`${API_URL}/products/${id}/reviews`);

  if (!response.ok) {
    throw new Error(`Failed to fetch reviews: ${response.status}`);
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

