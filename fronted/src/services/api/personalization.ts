import type { AnyCartItem, Product, Service } from "../../types";
import { getProductById, getServiceById } from "../../services/api/api";
import { API_URLS } from "./config";

const AUTH_SESSION_KEY = "soloservice.auth-session";
const COMPARISON_ID_KEY = "soloservice.product-comparison-id";
const SERVICE_COMPARISON_ID_KEY = "soloservice.service-comparison-id";
let refreshPromise: Promise<AuthSession | null> | null = null;

export interface AuthUser {
  userId: number;
  name: string;
  email: string;
}

export interface AuthSession extends AuthUser {
  accessToken: string;
  refreshToken: string;
  accessTokenExpiresAt: number;
}

export interface SearchHistoryEntry {
  id: number;
  query: string;
  type: string;
  createdAt: string;
}

export interface ApiFieldError {
  field: string;
  code: string;
  message: string;
}

export class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
    readonly fieldErrors: ApiFieldError[] = [],
  ) {
    super(message);
  }
}

interface ApiCartItem {
  id: number;
  itemType: "product" | "service";
  productId: string | null;
  offerId: number | null;
  serviceId: string | null;
  serviceOfferId: number | null;
  serviceProvider: string | null;
  serviceMonthlyPrice: number | null;
  serviceBillingPeriod: string | null;
  serviceInstallationCost: number | null;
  serviceContractPeriod: string | null;
  quantity: number;
}

interface ApiComparison {
  id: number;
  products: Array<{ id: string }>;
}

export interface ApiProductAttributeRow {
  key: string;
  label: string;
  dataType: string;
  unit: string | null;
  values: Array<string | number | boolean | null>;
  differs: boolean;
}

export interface ApiProductComparisonMatrix {
  category: { slug: string | null; name: string } | null;
  products: Array<{ id: string; name: string; brand: string; model: string }>;
  attributes: ApiProductAttributeRow[];
}

interface ApiServiceComparison {
  id: number;
  services: Array<{ id: string }>;
}

export async function request<T>(
  baseUrl: string,
  path: string,
  init?: RequestInit,
  retryWithRefresh = true,
): Promise<T> {
  const session = getStoredAuthSession();
  const apiName = getApiName(baseUrl);
  const response = await fetchWithServiceError(
    `${baseUrl}${path}`,
    {
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...(session ? { Authorization: `Bearer ${session.accessToken}` } : {}),
        ...init?.headers,
      },
    },
    apiName,
  );

  if (response.status === 401 && retryWithRefresh && baseUrl !== API_URLS.auth && session) {
    const refreshedSession = await refreshAuthSession(session);
    if (refreshedSession) {
      return request<T>(baseUrl, path, init, false);
    }
  }

  if (!response.ok) {
    let message = `${apiName}: error HTTP ${response.status} al consultar ${path}`;
    let fieldErrors: ApiFieldError[] = [];
    try {
      // problem+json (estándar TAL-78) trae el mensaje en detail.
      const body = (await response.json()) as {
        detail?: string;
        error?: string;
        message?: string;
        errors?: ApiFieldError[];
      };
      message = `${apiName}: ${body.detail ?? body.error ?? body.message ?? `error HTTP ${response.status}`}`;
      fieldErrors = Array.isArray(body.errors)
        ? body.errors.filter(
            (item) =>
              typeof item.field === "string" &&
              typeof item.code === "string" &&
              typeof item.message === "string",
          )
        : [];
    } catch {
      // Keep the HTTP status message when an endpoint does not return JSON.
    }
    throw new ApiError(message, response.status, fieldErrors);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
}

function getApiName(baseUrl: string): string {
  if (baseUrl === API_URLS.products) return "Product API";
  if (baseUrl === API_URLS.stores) return "Store API";
  if (baseUrl === API_URLS.services) return "Services API";
  if (baseUrl === API_URLS.auth) return "Auth API";
  if (baseUrl === API_URLS.favorites) return "Favorites API";
  if (baseUrl === API_URLS.cart) return "Cart API";
  if (baseUrl === API_URLS.comparison) return "Comparison API";
  if (baseUrl === API_URLS.search) return "Search API";
  return "API";
}

async function fetchWithServiceError(
  url: string,
  init: RequestInit,
  apiName: string,
): Promise<Response> {
  try {
    return await fetch(url, init);
  } catch (error) {
    if (error instanceof TypeError) {
      throw new ApiError(
        `${apiName}: no se pudo conectar. Verifica que el servicio esté disponible.`,
        0,
      );
    }
    throw error;
  }
}

interface TokenResponse {
  userId: number;
  name: string;
  email: string;
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
}

function getStoredAuthSession(): AuthSession | null {
  try {
    const value: unknown = JSON.parse(localStorage.getItem(AUTH_SESSION_KEY) ?? "null");
    if (typeof value !== "object" || value === null) return null;
    const session = value as Partial<AuthSession>;
    if (
      typeof session.userId === "number" &&
      Number.isInteger(session.userId) &&
      session.userId > 0 &&
      typeof session.name === "string" &&
      typeof session.email === "string" &&
      typeof session.accessToken === "string" &&
      typeof session.refreshToken === "string" &&
      typeof session.accessTokenExpiresAt === "number"
    ) {
      return session as AuthSession;
    }
  } catch {
    localStorage.removeItem(AUTH_SESSION_KEY);
  }

  return null;
}

function storeAuthSession(session: AuthSession): void {
  localStorage.setItem(AUTH_SESSION_KEY, JSON.stringify(session));
}

async function refreshAuthSession(session: AuthSession): Promise<AuthSession | null> {
  if (refreshPromise) return refreshPromise;

  refreshPromise = (async () => {
    try {
      const response = await fetch(`${API_URLS.auth}/auth/refresh`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ refreshToken: session.refreshToken }),
      });
      if (!response.ok) {
        if (response.status === 401) localStorage.removeItem(AUTH_SESSION_KEY);
        return null;
      }
      const tokens = (await response.json()) as TokenResponse;
      return await createSession(tokens);
    } catch {
      return null;
    } finally {
      refreshPromise = null;
    }
  })();

  return refreshPromise;
}

async function createSession(tokens: TokenResponse): Promise<AuthSession> {
  if (
    !Number.isInteger(tokens.userId) ||
    tokens.userId <= 0 ||
    !tokens.name ||
    !tokens.email ||
    !tokens.accessToken ||
    !tokens.refreshToken
  ) {
    throw new Error("La API de autenticación devolvió una sesión inválida.");
  }

  const session: AuthSession = {
    userId: tokens.userId,
    name: tokens.name,
    email: tokens.email,
    accessToken: tokens.accessToken,
    refreshToken: tokens.refreshToken,
    accessTokenExpiresAt: Date.now() + tokens.expiresIn * 1000,
  };
  storeAuthSession(session);
  return session;
}

export async function loginAccount(email: string, password: string): Promise<AuthSession> {
  const tokens = await request<TokenResponse>(API_URLS.auth, "/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password }),
  });
  return createSession(tokens);
}

export async function registerAccount(
  name: string,
  email: string,
  password: string,
): Promise<AuthSession> {
  const tokens = await request<TokenResponse>(API_URLS.auth, "/auth/register", {
    method: "POST",
    body: JSON.stringify({ name, email, password }),
  });
  return createSession(tokens);
}

export async function restoreAuthSession(): Promise<AuthSession | null> {
  const session = getStoredAuthSession();
  if (!session) return null;

  if (session.accessTokenExpiresAt > Date.now()) {
    try {
      await request<{ valid: boolean }>(API_URLS.auth, "/auth/validate");
      return session;
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401) throw error;
    }
  }

  return refreshAuthSession(session);
}

export async function logoutAccount(): Promise<void> {
  const session = getStoredAuthSession();
  try {
    if (session) {
      await request<void>(API_URLS.auth, "/auth/logout", {
        method: "POST",
        body: JSON.stringify({ refreshToken: session.refreshToken }),
      });
    }
  } finally {
    localStorage.removeItem(AUTH_SESSION_KEY);
  }
}

export async function getSearchHistory(userId: number): Promise<SearchHistoryEntry[]> {
  return request<SearchHistoryEntry[]>(API_URLS.search, `/search/history?userId=${userId}`);
}

export async function addSearchHistory(
  userId: number,
  query: string,
  type: "product" | "service",
): Promise<void> {
  await request<void>(API_URLS.search, "/search/history", {
    method: "POST",
    body: JSON.stringify({ userId, query, type }),
  });
}

export function isApiNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export async function getFavoriteProductIds(userId: number): Promise<string[]> {
  const favorites = await request<Array<{ productId: string }>>(
    API_URLS.favorites,
    `/favorites?userId=${userId}`,
  );
  return favorites.map((favorite) => favorite.productId);
}

export async function setFavoriteProduct(
  userId: number,
  productId: string,
  favorite: boolean,
): Promise<void> {
  const path = `/favorites/${encodeURIComponent(productId)}${favorite ? "" : `?userId=${userId}`}`;
  await request<void>(API_URLS.favorites, path, {
    method: favorite ? "POST" : "DELETE",
    ...(favorite ? { body: JSON.stringify({ userId }) } : {}),
  });
}

export async function getFavoriteServiceIds(userId: number): Promise<string[]> {
  const favorites = await request<Array<{ serviceId: string }>>(
    API_URLS.favorites,
    `/favorites/services?userId=${userId}`,
  );
  return favorites.map((favorite) => favorite.serviceId);
}

export async function setFavoriteService(
  userId: number,
  serviceId: string,
  favorite: boolean,
): Promise<void> {
  const path = `/favorites/services/${encodeURIComponent(serviceId)}${favorite ? "" : `?userId=${userId}`}`;
  await request<void>(API_URLS.favorites, path, {
    method: favorite ? "POST" : "DELETE",
    ...(favorite ? { body: JSON.stringify({ userId }) } : {}),
  });
}

export async function getCartItems(userId: number): Promise<AnyCartItem[]> {
  const items = await request<ApiCartItem[]>(API_URLS.cart, `/cart?userId=${userId}`);
  const productsById = new Map<string, Product>();
  const servicesById = new Map<string, Service>();
  await Promise.all(
    [...new Set(items.flatMap((item) => (item.productId ? [item.productId] : [])))].map(
      async (productId) => {
        const product = await getProductById(productId);
        if (!product) throw new Error(`No se encontró el producto ${productId} de la cesta.`);
        productsById.set(productId, product);
      },
    ),
  );
  await Promise.all(
    [...new Set(items.flatMap((item) => (item.serviceId ? [item.serviceId] : [])))].map(
      async (serviceId) => {
        const service = await getServiceById(serviceId);
        if (!service) throw new Error(`No se encontró el servicio ${serviceId} de la cesta.`);
        servicesById.set(serviceId, service);
      },
    ),
  );

  return items.map((item) => {
    if (item.itemType === "service") {
      const service = item.serviceId ? servicesById.get(item.serviceId) : undefined;
      if (!service || !item.serviceOfferId || item.serviceMonthlyPrice === null) {
        throw new Error(
          `No se encontró la oferta de servicio ${item.serviceOfferId ?? ""} de la cesta.`,
        );
      }
      const contractMonths = item.serviceContractPeriod?.match(/^\s*(\d+)/)?.[1];
      return {
        type: "service",
        service: {
          ...service,
          provider: item.serviceProvider ?? service.provider,
          monthlyPrice: item.serviceMonthlyPrice,
          billingPeriod: item.serviceBillingPeriod ?? service.billingPeriod ?? "monthly",
          installationCost: item.serviceInstallationCost,
          contractMonths: contractMonths ? Number(contractMonths) : null,
          offerId: item.serviceOfferId,
        },
        serviceOfferId: item.serviceOfferId,
        quantity: item.quantity,
        cartItemId: item.id,
      };
    }

    if (!item.productId || !item.offerId) {
      throw new Error(`No se encontró la oferta ${item.offerId} de la cesta.`);
    }
    const product = productsById.get(item.productId);
    const offer = product?.offers.find((candidate) => candidate.offerId === item.offerId);
    if (!product || !offer) {
      throw new Error(`No se encontró la oferta ${item.offerId} de la cesta.`);
    }
    return { type: "product", product, offer, quantity: item.quantity, cartItemId: item.id };
  });
}

export async function addCartItem(userId: number, productId: string, offerId: number) {
  await request(API_URLS.cart, "/cart/items", {
    method: "POST",
    body: JSON.stringify({ userId, productId, offerId, quantity: 1 }),
  });
}

export async function addServiceCartItem(
  userId: number,
  serviceId: string,
  serviceOfferId: number,
): Promise<void> {
  await request(API_URLS.cart, "/cart/items", {
    method: "POST",
    body: JSON.stringify({ userId, serviceId, serviceOfferId, quantity: 1 }),
  });
}

export async function updateCartItem(
  userId: number,
  cartItemId: number,
  quantity: number,
): Promise<void> {
  await request(API_URLS.cart, `/cart/items/${cartItemId}`, {
    method: "PUT",
    body: JSON.stringify({ userId, quantity }),
  });
}

export async function removeCartItem(userId: number, cartItemId: number): Promise<void> {
  await request(API_URLS.cart, `/cart/items/${cartItemId}?userId=${userId}`, {
    method: "DELETE",
  });
}

export function getStoredComparisonId(): number | null {
  const id = Number(localStorage.getItem(COMPARISON_ID_KEY));
  return Number.isInteger(id) && id > 0 ? id : null;
}

export function storeComparisonId(id: number | null): void {
  if (id === null) localStorage.removeItem(COMPARISON_ID_KEY);
  else localStorage.setItem(COMPARISON_ID_KEY, String(id));
}

export async function getProductComparison(id: number): Promise<ApiComparison> {
  return request<ApiComparison>(API_URLS.comparison, `/comparisons/products/${id}`);
}

export async function compareProducts(ids: string[]): Promise<ApiProductComparisonMatrix> {
  const query = new URLSearchParams({ ids: ids.join(",") });
  return request<ApiProductComparisonMatrix>(
    API_URLS.comparison,
    `/comparisons/products/compare?${query.toString()}`,
  );
}

export async function createProductComparison(userId: number, productId: string): Promise<number> {
  const result = await request<{ id: number }>(API_URLS.comparison, "/comparisons/products", {
    method: "POST",
    body: JSON.stringify({ userId, productIds: [productId] }),
  });
  return result.id;
}

export async function addProductToComparison(
  comparisonId: number,
  productId: string,
): Promise<void> {
  await request(API_URLS.comparison, `/comparisons/products/${comparisonId}/items`, {
    method: "POST",
    body: JSON.stringify({ productId }),
  });
}

export async function removeProductFromComparison(
  comparisonId: number,
  productId: string,
): Promise<void> {
  await request(
    API_URLS.comparison,
    `/comparisons/products/${comparisonId}/items/${encodeURIComponent(productId)}`,
    { method: "DELETE" },
  );
}

export async function deleteProductComparison(comparisonId: number): Promise<void> {
  await request(API_URLS.comparison, `/comparisons/products/${comparisonId}`, {
    method: "DELETE",
  });
}

export function getStoredServiceComparisonId(): number | null {
  const id = Number(localStorage.getItem(SERVICE_COMPARISON_ID_KEY));
  return Number.isInteger(id) && id > 0 ? id : null;
}

export function storeServiceComparisonId(id: number | null): void {
  if (id === null) localStorage.removeItem(SERVICE_COMPARISON_ID_KEY);
  else localStorage.setItem(SERVICE_COMPARISON_ID_KEY, String(id));
}

export async function getServiceComparison(id: number): Promise<ApiServiceComparison> {
  return request<ApiServiceComparison>(API_URLS.comparison, `/comparisons/services/${id}`);
}

export async function createServiceComparison(userId: number, serviceId: string): Promise<number> {
  const result = await request<{ id: number }>(API_URLS.comparison, "/comparisons/services", {
    method: "POST",
    body: JSON.stringify({ userId, serviceIds: [serviceId] }),
  });
  return result.id;
}

export async function addServiceToComparison(
  comparisonId: number,
  serviceId: string,
): Promise<void> {
  await request(API_URLS.comparison, `/comparisons/services/${comparisonId}/items`, {
    method: "POST",
    body: JSON.stringify({ serviceId }),
  });
}

export async function removeServiceFromComparison(
  comparisonId: number,
  serviceId: string,
): Promise<void> {
  await request(
    API_URLS.comparison,
    `/comparisons/services/${comparisonId}/items/${encodeURIComponent(serviceId)}`,
    { method: "DELETE" },
  );
}

export async function deleteServiceComparison(comparisonId: number): Promise<void> {
  await request(API_URLS.comparison, `/comparisons/services/${comparisonId}`, {
    method: "DELETE",
  });
}
