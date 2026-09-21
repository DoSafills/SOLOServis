import type { Store } from "../../types";
import { API_URLS, getJson } from "./config";

export interface ApiStore {
  id: number;
  name: string;
  website: string;
  logo: string;
  rating: number;
  reputation: string;
  shippingInformation: string;
  generalConditions: string;
  productCount: number;
}

export function toStore(store: ApiStore): Store {
  return {
    id: String(store.id),
    name: store.name,
    logo: store.logo || store.name.slice(0, 2).toUpperCase(),
    rating: store.rating,
    reviewCount: 0,
    productCount: store.productCount,
    reputation: store.reputation,
    dispatchTime: store.shippingInformation,
    conditions: store.generalConditions,
    website: store.website,
  };
}

export const getStores = () => getJson<ApiStore[]>(`${API_URLS.stores}/stores`);
export const getStore = (id: string) => getJson<ApiStore>(`${API_URLS.stores}/stores/${id}`);
