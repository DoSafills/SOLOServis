import type { Product, PricePoint } from "../../types";
import { fetchJson, fetchJsonOrNull } from "./http";

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

interface ApiProduct {
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

const parseAmount = (value: string | undefined): number => {
  const amount = Number(value);
  return Number.isFinite(amount) ? amount : 0;
};

const toPricePoints = (points: ApiPricePoint[] | undefined): PricePoint[] =>
  (points ?? []).map((point) => ({
    date: point.date,
    price: parseAmount(point.price),
  }));

const toProduct = (product: ApiProduct): Product => {
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
    // El modelo se muestra como una especificación más (también en el comparador).
    specs: { ...(product.model ? { Modelo: product.model } : {}), ...product.specs },
    offers,
    priceHistory,
    offerPriceHistory,
    offerPrice: currentOffer,
    tags: [],
  };
};

/** Filtrar por una categoría incluye los productos de sus subcategorías. */
export async function getProducts(categoryId?: number): Promise<Product[]> {
  const query = categoryId ? `?category=${categoryId}` : "";
  const products = await fetchJson<ApiProduct[]>(`/products${query}`);

  return products.map(toProduct);
}

export async function getProductById(id: string): Promise<Product | null> {
  const product = await fetchJsonOrNull<ApiProduct>(`/products/${id}`);

  return product && toProduct(product);
}

export const getCategories = () => fetchJson<ApiProductCategory[]>("/categories");

export const getProductReviews = (id: string) =>
  fetchJson<ApiProductReview[]>(`/products/${id}/reviews`);
