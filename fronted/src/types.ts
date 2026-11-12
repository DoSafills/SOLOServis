export type Page =
  | { id: "home" }
  | { id: "product-categories" }
  | { id: "service-categories" }
  | {
      id: "search-products";
      query: string;
      category?: string;
      categoryContext?: string;
      productGroup?: string;
      categoryId?: number;
    }
  | {
      id: "product-detail";
      productId: string;
    }
  | { id: "product-comparison"; productIds: string[] }
  | { id: "offer-comparison"; product: Product; storeIds: string[] }
  | {
      id: "search-services";
      query: string;
      category?: string;
      group?: string;
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
  | { id: "user" }
  | { id: "cart" };

export interface StoreOffer {
  offerId: number;
  storeId: string;
  storeName: string;
  price: number | null;
  listPrice?: number | null;
  available: boolean;
  shipping: number | null;
  shippingFree?: boolean;
  stock?: number | null;
  condition?: string;
  url?: string;
  deliveryTime?: string;
  warranty?: string;
  lastUpdated?: string;
}

export interface PricePoint {
  date: string;
  price: number;
}

export interface Product {
  id: string;
  name: string;
  brand: string;
  brandId?: number | null;
  model: string;
  /** Id de la categoría del producto en el backend (la subcategoría si la tiene) */
  categoryId?: number;
  categorySlug?: string | null;
  category: string;
  subcategory: string;
  image: string;
  images: string[];
  description: string;
  rating: number;
  reviewCount: number;
  specs: Record<string, string>;
  attributes?: Record<string, string | number | boolean>;
  offers: StoreOffer[];
  priceHistory: PricePoint[];
  /** Current sale/offer price if product is on promotion right now */
  offerPrice?: number;
  /** Historical record of offer prices; empty array means no offers have occurred */
  offerPriceHistory: PricePoint[];
  tags: string[];
}

export interface CartItem {
  type: "product";
  product: Product;
  offer: StoreOffer;
  quantity: number;
  cartItemId: number;
}

export interface ServiceCartItem {
  type: "service";
  service: Service;
  serviceOfferId: number;
  quantity: number;
  cartItemId: number;
}

export type AnyCartItem = CartItem | ServiceCartItem;

export interface Service {
  id: string;
  offerId?: number;
  name: string;
  provider: string;
  category: string;
  subcategory: string;
  description: string;
  monthlyPrice: number;
  billingPeriod?: string;
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

export interface StoreProduct {
  id: string;
  offerId: number | null;
  name: string;
  brand: string;
  model: string;
  category: string;
  description: string;
  price: number | null;
  listPrice: number | null;
  currency: string;
  shippingCost: number | null;
  shippingFree: boolean;
  available: boolean;
  stock: number | null;
  condition: string;
  productUrl: string;
  image: string;
  lastUpdated?: string;
}

export interface StoreLocation {
  id: number;
  locationId: number;
  address: string;
  postalCode: string;
  country: string;
  region: string;
  city: string;
  commune: string;
}

export interface StoreDetail extends Store {
  products: StoreProduct[];
  locations: StoreLocation[];
}
