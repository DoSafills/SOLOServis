import type { Store } from "../../types";
import { fetchJson, fetchJsonOrNull } from "./http";

export const getStores = () => fetchJson<Store[]>("/stores");

export const getStoreById = (id: string) => fetchJsonOrNull<Store>(`/stores/${id}`);
