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

export interface ApiProductOfferUpdate {
  productId: string;
  offerId: number;
  lastUpdated: string;
}

const isApiProductOfferUpdate = (value: unknown): value is ApiProductOfferUpdate =>
  typeof value === "object" &&
  value !== null &&
  "productId" in value &&
  typeof value.productId === "string" &&
  "offerId" in value &&
  Number.isSafeInteger(value.offerId) &&
  "lastUpdated" in value &&
  typeof value.lastUpdated === "string";

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

export async function getProductOfferUpdates(
  productId: string,
): Promise<ApiProductOfferUpdate[] | undefined> {
  const controller = new AbortController();
  const timeoutId = setTimeout(() => controller.abort(), 5000);
  const offers: ApiProductOfferUpdate[] = [];
  let totalCount: number | undefined;

  try {
    while (totalCount === undefined || offers.length < totalCount) {
      const query = new URLSearchParams({
        product: productId,
        limit: "100",
        offset: String(offers.length),
      });
      const response = await fetch(`${API_URLS.stores}/stores/offers?${query}`, {
        signal: controller.signal,
      });

      if (!response.ok) {
        throw new Error(`Store API respondió HTTP ${response.status}.`);
      }

      const contentType = response.headers.get("content-type")?.split(";")[0].trim().toLowerCase();
      if (contentType !== "application/json") {
        throw new Error("Store API no devolvió una respuesta JSON.");
      }

      const totalHeader = response.headers.get("X-Total-Count");
      const pageTotal = totalHeader === null ? Number.NaN : Number(totalHeader);
      if (!Number.isSafeInteger(pageTotal) || pageTotal < 0) {
        throw new Error("Store API devolvió un total de ofertas inválido.");
      }
      if (totalCount !== undefined && pageTotal !== totalCount) {
        throw new Error("Store API devolvió totales de ofertas inconsistentes.");
      }
      totalCount = pageTotal;

      const page: unknown = await response.json();
      if (
        !Array.isArray(page) ||
        !page.every((item) => isApiProductOfferUpdate(item) && item.productId === productId)
      ) {
        throw new Error("Store API devolvió ofertas con un formato inválido.");
      }
      if (page.length === 0 && offers.length < totalCount) {
        throw new Error("Store API devolvió una página de ofertas incompleta.");
      }

      offers.push(...page);
      if (offers.length > totalCount) {
        throw new Error("Store API devolvió más ofertas que las informadas.");
      }
    }

    return offers;
  } catch (error) {
    console.warn("No se pudieron cargar las fechas de verificación de ofertas.", error);
    return undefined;
  } finally {
    clearTimeout(timeoutId);
  }
}
