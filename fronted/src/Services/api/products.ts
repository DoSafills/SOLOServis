import type { Product, PricePoint } from "../../types";

interface ApiImage {
  url: string;
  altText: string;
  sortOrder: number;
}

interface ApiOffer {
  offerId: number;
  storeId: string;
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

export interface ApiProduct {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  description: string;
  rating: number;
  reviewCount: number;
  specs?: Record<string, string>;
  images?: ApiImage[];
  offers?: ApiOffer[];
  priceHistory?: ApiPricePoint[];
  offerPriceHistory?: ApiPricePoint[];
}

export const parseAmount = (value: string | number | null | undefined, fallback = 0): number => {
  const amount = Number(value ?? fallback);
  return Number.isFinite(amount) ? amount : fallback;
};

const toPricePoints = (points: ApiPricePoint[] | undefined): PricePoint[] =>
  (points ?? []).map((point) => ({
    date: point.date,
    price: parseAmount(point.price),
  }));

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
    model: product.model,
    category: product.category,
    subcategory: "",
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
