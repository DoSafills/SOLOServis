import type { Service } from "../../types";
import { fetchJson, fetchJsonOrNull } from "./http";

export const getServices = () => fetchJson<Service[]>("/services");

export const getServiceById = (id: string) => fetchJsonOrNull<Service>(`/services/${id}`);
