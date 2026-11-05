// The frontend host routes API path prefixes to the corresponding services.
export const API_URLS = {
  products: import.meta.env.VITE_PRODUCTS_API_URL ?? import.meta.env.VITE_PRODUCT_API_URL ?? "",
  stores: import.meta.env.VITE_STORES_API_URL ?? import.meta.env.VITE_STORE_API_URL ?? "",
  services: import.meta.env.VITE_SERVICES_API_URL ?? import.meta.env.VITE_SERVICE_API_URL ?? "",
  search: import.meta.env.VITE_SEARCH_API_URL ?? "",
  auth: import.meta.env.VITE_AUTH_API_URL ?? "",
  favorites: import.meta.env.VITE_FAVORITES_API_URL ?? "",
  cart: import.meta.env.VITE_CART_API_URL ?? "",
  comparison: import.meta.env.VITE_COMPARISON_API_URL ?? "",
};

export async function getJson<T>(url: string, apiName = "API"): Promise<T> {
  const response = await getResponse(url, apiName);
  return response.json() as Promise<T>;
}

export async function getResponse(url: string, apiName = "API"): Promise<Response> {
  return getServiceResponse(url, apiName);
}

export async function fetchServiceResponse(url: string, apiName: string): Promise<Response> {
  try {
    return await fetch(url);
  } catch (error) {
    if (error instanceof TypeError) {
      const connectionError = new Error(
        `${apiName}: no se pudo conectar. Verifica que el servicio esté disponible.`,
      );
      Object.defineProperty(connectionError, "cause", { value: error });
      throw connectionError;
    }
    throw error;
  }
}

export async function getServiceResponse(url: string, apiName: string): Promise<Response> {
  const response = await fetchServiceResponse(url, apiName);
  if (!response.ok) {
    let message = `${apiName}: error HTTP ${response.status}.`;
    try {
      const problem = (await response.json()) as { detail?: string };
      if (problem.detail) message = `${apiName}: ${problem.detail}`;
    } catch {
      // Retain the HTTP status when the server does not return JSON.
    }
    throw new Error(message);
  }
  return response;
}

export function getServiceJson<T>(url: string, apiName: string): Promise<T> {
  return getServiceResponse(url, apiName).then((response) => response.json() as Promise<T>);
}
