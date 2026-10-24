import type { Service } from "../../types";
import { API_URLS, getJson } from "./config";

export interface ApiService {
  id: string;
  name: string;
  provider: string;
  category: string;
  description: string;
  monthlyPrice: number;
  installationCost: number;
  contractPeriod: string;
  rating: number;
  reviewCount: number;
  coverage: string;
  image: string;
}

export function toService(service: ApiService): Service {
  return {
    ...service,
    subcategory: "",
    contractMonths: null,
    specs: {},
    benefits: [],
    priceHistory: [],
    installationCost: service.installationCost || null,
  };
}

export const getServices = () => getJson<ApiService[]>(`${API_URLS.services}/services`);
export const getService = (id: string) => getJson<ApiService>(`${API_URLS.services}/services/${id}`);
