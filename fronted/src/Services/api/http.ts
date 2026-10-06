/**
 * TODO (RF-28 / RNF-03): apuntar al API Gateway cuando exista.
 * Hoy esta URL es el servidor combinado (cmd/server), no un Gateway: no hay
 * autenticación (RNF-14) ni rate limiting (RNF-07) delante de la API, y las
 * rutas de escritura de productos están abiertas. Mientras tanto, el cliente
 * sigue usando una sola URL base y nunca llama directo a products-api,
 * stores-api o services-api.
 */
const API_URL = import.meta.env.VITE_API_URL ?? "http://localhost:8080";

async function request(path: string): Promise<Response> {
  const response = await fetch(`${API_URL}${path}`);

  if (!response.ok && response.status !== 404) {
    throw new Error(`Error ${response.status} al consultar ${path}`);
  }

  return response;
}

export async function fetchJson<T>(path: string): Promise<T> {
  const response = await request(path);

  if (response.status === 404) {
    throw new Error(`No se encontró ${path}`);
  }

  return response.json() as Promise<T>;
}

/** Igual que fetchJson, pero devuelve null si el recurso no existe (404). */
export async function fetchJsonOrNull<T>(path: string): Promise<T | null> {
  const response = await request(path);

  return response.status === 404 ? null : (response.json() as Promise<T>);
}
