import { API_URLS, getJson } from "./config";

export type CategoryAttributeDataType = "string" | "integer" | "decimal" | "boolean" | "enum";

export interface CategoryAttribute {
  key: string;
  label: string;
  dataType: CategoryAttributeDataType;
  unit: string | null;
  required: boolean;
  comparable: boolean;
  filterable: boolean;
  min: number | null;
  max: number | null;
  maxLength: number | null;
  allowedValues: string[];
  displayOrder: number;
}

export interface ProductCategory {
  slug: string;
  name: string;
  itemType: "product" | "service";
  description: string | null;
  attributeCount: number;
}

export interface ProductCategoryDetail extends Omit<ProductCategory, "attributeCount"> {
  attributes: CategoryAttribute[];
}

export async function getDynamicCategories(): Promise<ProductCategory[]> {
  const response = await getJson<{ items: ProductCategory[] }>(
    `${API_URLS.products}/categories?itemType=product&limit=100`,
  );
  return response.items;
}

export function getCategoryBySlug(slug: string): Promise<ProductCategoryDetail> {
  return getJson<ProductCategoryDetail>(
    `${API_URLS.products}/categories/${encodeURIComponent(slug)}`,
  );
}
