export type Page =
  | { id: "home" }
  | { id: "search-products"; query: string }
  | {
      id: "product-detail";
      productId: string;
    }
  | { id: "product-comparison"; productIds: string[] }
  | {
      id: "search-services";
      query: string;
    }
  | { id: "service-detail"; serviceId: string }
  | {
      id: "service-comparison";
      serviceIds: string[];
    }
  | { id: "stores" }
  | { id: "store-detail"; storeId: string }
  | {
      id: "favorites";
    }
  | { id: "user" };

export interface StoreOffer {
  storeId: string;
  storeName: string;
  price: number;
  available: boolean;
  shipping: number | null;
  url?: string;
}

export interface PricePoint {
  date: string;
  price: number;
}

export interface Product {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  subcategory: string;
  image: string;
  images: string[];
  description: string;
  rating: number;
  reviewCount: number;
  specs: Record<string, string>;
  offers: StoreOffer[];
  priceHistory: PricePoint[];
  /** Current sale/offer price if product is on promotion right now */
  offerPrice?: number;
  /** Historical record of offer prices; empty array means no offers have occurred */
  offerPriceHistory: PricePoint[];
  tags: string[];
}

export interface Service {
  id: string;
  name: string;
  provider: string;
  category: string;
  subcategory: string;
  description: string;
  monthlyPrice: number;
  installationCost: number | null;
  contractMonths: number | null;
  rating: number;
  reviewCount: number;
  specs: Record<string, string>;
  benefits: string[];
  coverage: string;
  image: string;
  priceHistory: PricePoint[];
}

export interface Store {
  id: string;
  name: string;
  logo: string;
  rating: number;
  reviewCount: number;
  productCount: number;
  reputation: string;
  dispatchTime: string;
  conditions: string;
  website: string;
}
