import type { Product, StoreOffer } from "../../types";
import { getProducts, type ApiProduct } from "./products";
import { getOffers, type ApiOffer } from "./offers";

// Subconjunto de campos que comparten `ApiOffer` (GET /product-offers) y
// `ApiProductDetailOffer` (ofertas embebidas en GET /products/{id}), para
// poder mapear cualquiera de las dos con la misma función.
export interface OfferLike {
  id: number;
  storeId: number;
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

export function mapOffer(offer: OfferLike): StoreOffer {
  return {
    id: offer.id,
    storeId: String(offer.storeId),
    storeName: offer.storeName,
    price: Number(offer.price),
    listPrice: offer.listPrice ? Number(offer.listPrice) : undefined,
    available: offer.available,
    shipping: offer.shippingFree ? 0 : Number(offer.shippingCost),
    url: offer.productUrl || undefined,
  };
}

export function mapProduct(product: ApiProduct, offers: ApiOffer[]): Product {
  return {
    id: product.id,
    name: product.name,
    brand: product.brand,
    model: product.model,
    category: product.category,
    subcategory: "",
    image: "",
    images: [],
    description: product.description,
    rating: product.rating,
    reviewCount: product.reviewCount,
    specs: product.model ? { Modelo: product.model } : {},
    offers: offers.map(mapOffer),
    priceHistory: [],
    offerPriceHistory: [],
    tags: [],
  };
}

export async function getCatalogProducts(): Promise<Product[]> {
  const [products, offers] = await Promise.all([getProducts(), getOffers()]);

  const offersByProduct = new Map<string, ApiOffer[]>();
  for (const offer of offers) {
    const list = offersByProduct.get(offer.productPublicId) ?? [];
    list.push(offer);
    offersByProduct.set(offer.productPublicId, list);
  }

  return products.map((product) => mapProduct(product, offersByProduct.get(product.id) ?? []));
}
