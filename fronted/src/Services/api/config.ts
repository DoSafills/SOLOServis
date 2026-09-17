export const API_URLS = {
  products: import.meta.env.VITE_PRODUCTS_API_URL ?? "http://localhost:8081",
  stores: import.meta.env.VITE_STORES_API_URL ?? "http://localhost:8082",
  services: import.meta.env.VITE_SERVICES_API_URL ?? "http://localhost:8083",
};

export async function getJson<T>(url: string): Promise<T> {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`API request failed: ${response.status}`);
  return response.json() as Promise<T>;
}
