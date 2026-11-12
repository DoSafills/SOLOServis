import type { Product, PricePoint } from "../../types";
import { API_URLS, fetchServiceResponse, getJson } from "./config";

interface ApiImage {
  url: string;
  altText: string;
  sortOrder: number;
}

export interface ApiOffer {
  offerId: number;
  storeId: number;
  storeName: string;
  price: string;
  listPrice: string | number | null;
  currency: string;
  shippingCost: string | number | null;
  shippingFree: boolean;
  available: boolean;
  stock: number | null;
  condition: string;
  productUrl: string;
  deliveryTime?: string | null;
  warranty?: string | null;
  lastUpdated?: string | null;
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
  brandId?: number | null;
  model: string;
  categoryId: number;
  categorySlug?: string | null;
  category: string;
  subcategory: string;
  description: string;
  rating: number;
  reviewCount: number;
  specs?: Record<string, string>;
  attributes?: Record<string, string | number | boolean>;
  images?: ApiImage[];
  offers?: ApiOffer[];
  priceHistory?: ApiPricePoint[];
  offerPriceHistory?: ApiPricePoint[];
}

export function parseAmount(
  value: string | number | null | undefined,
  fallback: number,
): number;
export function parseAmount(
  value: string | number | null | undefined,
  fallback?: null,
): number | null;
export function parseAmount(
  value: string | number | null | undefined,
  fallback: number | null = null,
): number | null {
  if (value === null || value === undefined || (typeof value === "string" && !value.trim())) {
    return fallback;
  }
  const amount = Number(value);
  return Number.isFinite(amount) ? amount : fallback;
}

const toPricePoints = (points: ApiPricePoint[] | undefined): PricePoint[] =>
  (points ?? []).flatMap((point) => {
    const price = parseAmount(point.price);
    return price === null ? [] : [{ date: point.date, price }];
  });

export const toProduct = (product: ApiProduct): Product => {
  const images = (product.images ?? []).map((image) => image.url).filter(Boolean);

  const offers = (product.offers ?? []).map((offer) => {
    const shipping =
      offer.shippingCost === null || offer.shippingCost === ""
        ? null
        : parseAmount(offer.shippingCost);
    const listPrice =
      offer.listPrice === null || offer.listPrice === "" ? null : parseAmount(offer.listPrice);

    return {
      offerId: offer.offerId,
      storeId: String(offer.storeId),
      storeName: offer.storeName,
      price: parseAmount(offer.price),
      listPrice,
      available: offer.available,
      shipping: offer.shippingFree ? 0 : shipping,
      shippingFree: offer.shippingFree,
      stock: offer.stock,
      condition: offer.condition,
      url: offer.productUrl || undefined,
      deliveryTime: offer.deliveryTime ?? undefined,
      warranty: offer.warranty ?? undefined,
      lastUpdated: offer.lastUpdated ?? undefined,
    };
  });

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
    brandId: product.brandId,
    model: product.model,
    categoryId: product.categoryId,
    categorySlug: product.categorySlug ?? null,
    category: product.category,
    subcategory: product.subcategory ?? "",
    image: images[0] ?? "",
    images,
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.specs ?? {},
    attributes: product.attributes ?? {},
    offers,
    priceHistory,
    offerPriceHistory,
    offerPrice: currentOffer,
    tags: [],
  };
};

/** Filtrar por una categoría incluye los productos de sus subcategorías. */
export async function getProducts(categoryId?: number): Promise<ApiProduct[]> {
  const query = categoryId === undefined ? "" : `?category=${categoryId}`;
  return getJson<ApiProduct[]>(`${API_URLS.products}/products${query}`, "Product API");
}

export async function getCategories(): Promise<ApiProductCategory[]> {
  return getJson<ApiProductCategory[]>(`${API_URLS.products}/product-categories`, "Product API");
}

export async function getProductReviews(id: string): Promise<ApiProductReview[]> {
  return getJson<ApiProductReview[]>(`${API_URLS.products}/products/${id}/reviews`, "Product API");
}

export async function getProductById(id: string): Promise<Product | null> {
  const response = await fetchServiceResponse(
    `${API_URLS.products}/products/${encodeURIComponent(id)}`,
    "Product API",
  );

  if (response.status === 404) return null;

  if (!response.ok) {
    throw new Error(`Product API: error HTTP ${response.status}.`);
  }

  return toProduct((await response.json()) as ApiProduct);
}
