import type { Product } from "../../types";
import { API_URLS, getJson } from "./config";
import type { ApiProduct } from "./products";

export interface ApiProductDetail extends ApiProduct {
  images: Array<{ url: string; altText: string }>;
  offers: Array<{
    storeId: number;
    storeName: string;
    price: string;
    shippingCost: string;
    shippingFree: boolean;
    available: boolean;
    productUrl: string;
  }>;
  specifications: ApiProductSpecification[];
}

export interface ApiProductSpecification {
  name: string;
  value: string;
  unit: string;
  dataType: string;
  comparable: boolean;
}

function buildSpecs(model: string, specifications: ApiProductSpecification[]): Record<string, string> {
  const specs: Record<string, string> = {};
  if (model) specs.Modelo = model;
  for (const spec of specifications) {
    specs[spec.name] = spec.unit ? `${spec.value} ${spec.unit}` : spec.value;
  }
  return specs;
}

export function toProduct(product: ApiProduct | ApiProductDetail): Product {
  const detail = "images" in product ? product : undefined;
  const images = detail?.images.map((image) => image.url) ?? [];
  return {
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    categoryId: product.categoryId,
    category: product.category,
    subcategory: product.subcategory,
    image: images[0] ?? "",
    images,
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: buildSpecs(product.model, detail?.specifications ?? []),
    offers: detail?.offers.map((offer) => ({
      storeId: String(offer.storeId),
      storeName: offer.storeName,
      price: Number(offer.price) || 0,
      available: offer.available,
      shipping: offer.shippingFree ? 0 : Number(offer.shippingCost) || null,
      url: offer.productUrl,
    })) ?? [],
    priceHistory: [],
    offerPriceHistory: [],
    tags: [],
  };
}

export const getProduct = (id: string) =>
  getJson<ApiProductDetail>(`${API_URLS.products}/products/${id}`);

export interface ApiProductReview {
  author: string;
  /** El autor confirmó su email. No indica que haya comprado el producto. */
  authorVerified: boolean;
  rating: number;
  title: string;
  content: string;
  createdAt: string;
}

export const getProductReviews = (id: string) =>
  getJson<ApiProductReview[]>(`${API_URLS.products}/products/${id}/reviews`);
