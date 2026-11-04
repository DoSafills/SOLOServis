export function getApiBaseUrl(configuredUrl: string | undefined, port: number): string {
  return configuredUrl ?? `http://${window.location.hostname}:${port}`;
}

export const API_URLS = {
  products: getApiBaseUrl(
    import.meta.env.VITE_PRODUCTS_API_URL ?? import.meta.env.VITE_PRODUCT_API_URL,
    8081,
  ),
  stores: getApiBaseUrl(
    import.meta.env.VITE_STORES_API_URL ?? import.meta.env.VITE_STORE_API_URL,
    8082,
  ),
  services: getApiBaseUrl(
    import.meta.env.VITE_SERVICES_API_URL ?? import.meta.env.VITE_SERVICE_API_URL,
    8083,
  ),
};

export async function getJson<T>(url: string): Promise<T> {
  const response = await fetch(url);
  if (!response.ok) throw new Error(`API request failed: ${response.status}`);
  return response.json() as Promise<T>;
}
