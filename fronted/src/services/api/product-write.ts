import { request } from "./personalization";
import { API_URLS } from "./config";

export interface CreateProductInput {
  categorySlug: string;
  brandId?: number;
  name: string;
  model?: string;
  sku?: string;
  description?: string;
  imageUrl: string;
  attributes: Record<string, string | number | boolean>;
}

export type UpdateProductInput = Omit<CreateProductInput, "imageUrl">;

export interface DuplicateCandidateInput {
  categorySlug: string;
  brandId?: number;
  sku?: string;
  model?: string;
  excludeProductId?: string;
}

export interface DuplicateCandidate {
  id: string;
  name: string;
  brand: string;
  model: string;
  category: string;
  offerCount: number;
  matchType: "sku" | "category_brand_model";
}

export function checkProductDuplicates(
  input: DuplicateCandidateInput,
): Promise<DuplicateCandidate[]> {
  return request<DuplicateCandidate[]>(API_URLS.products, "/products/duplicate-candidates", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function createProduct(input: CreateProductInput): Promise<unknown> {
  return request<unknown>(API_URLS.products, "/products", {
    method: "POST",
    body: JSON.stringify(input),
  });
}

export function updateProduct(id: string, input: UpdateProductInput): Promise<unknown> {
  return request<unknown>(API_URLS.products, `/products/${encodeURIComponent(id)}`, {
    method: "PUT",
    body: JSON.stringify(input),
  });
}
