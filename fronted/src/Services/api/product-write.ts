import { request } from "./personalization";
import { API_URLS } from "./config";

export interface CreateProductInput {
  categorySlug: string;
  name: string;
  model?: string;
  sku?: string;
  description?: string;
  imageUrl: string;
  attributes: Record<string, string | number | boolean>;
}

export type UpdateProductInput = Omit<CreateProductInput, "imageUrl">;

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
