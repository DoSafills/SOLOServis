export interface ApiCategory {
  id: number;
  parentCategoryId: number | null;
  name: string;
  description: string;
  active: boolean;
}

const API_URL = import.meta.env.VITE_API_URL;

export async function getCategories(): Promise<ApiCategory[]> {
  const response = await fetch(`${API_URL}/categories`);

  if (!response.ok) {
    throw new Error(`Failed to fetch categories: ${response.status}`);
  }

  return response.json();
}
