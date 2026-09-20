import type { Product } from "../../types";
import { API_URLS, getJson } from "./config";
import type { ApiProduct } from "./products";

export interface ApiProductDetail extends ApiProduct {
  images: Array<{ url: string; altText: string; sortOrder?: number }>;
  offers: Array<{
    storeId: number;
    storeName: string;
    price: string;
    shippingCost: string;
    shippingFree: boolean;
    available: boolean;
    productUrl: string;
  }>;
}

export function toProduct(product: ApiProduct | ApiProductDetail): Product {
  const detail = "images" in product ? product : undefined;
  const images = detail?.images.map((image) => image.url) ?? [];
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
    specs: product.specs ?? (product.model ? { Modelo: product.model } : {}),
    offers: detail?.offers.map((offer) => ({
      storeId: String(offer.storeId),
      storeName: offer.storeName,
      price: Number(offer.price) || 0,
      available: offer.available,
      shipping: offer.shippingFree ? 0 : Number(offer.shippingCost) || null,
      url: offer.productUrl,
    })) ?? [],
    priceHistory: product.priceHistory ?? [],
    offerPrice: product.offerPrice,
    offerPriceHistory: product.offerPriceHistory ?? [],
    tags: product.tags ?? [],
  };
}

export const getProduct = (id: string) =>
  getJson<ApiProductDetail>(`${API_URLS.products}/products/${id}`);
