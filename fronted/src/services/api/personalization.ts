import type { AnyCartItem, Product, Service } from "../../types";
import { getProductById, getServiceById } from "../../services/api/api";

const FAVORITES_API_URL = import.meta.env.VITE_FAVORITES_API_URL ?? "http://localhost:8089";
const CART_API_URL = import.meta.env.VITE_CART_API_URL ?? "http://localhost:8090";
const COMPARISON_API_URL = import.meta.env.VITE_COMPARISON_API_URL ?? "http://localhost:8087";
const AUTH_API_URL = import.meta.env.VITE_AUTH_API_URL ?? "http://localhost:8088";
const SEARCH_API_URL = import.meta.env.VITE_SEARCH_API_URL ?? "http://localhost:8086";

const AUTH_SESSION_KEY = "soloservice.auth-session";
const COMPARISON_ID_KEY = "soloservice.product-comparison-id";
const SERVICE_COMPARISON_ID_KEY = "soloservice.service-comparison-id";

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

class ApiError extends Error {
  constructor(
    message: string,
    readonly status: number,
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

interface ApiServiceComparison {
  id: number;
  services: Array<{ id: string }>;
}

async function request<T>(
  baseUrl: string,
  path: string,
  init?: RequestInit,
  retryWithRefresh = true,
): Promise<T> {
  const session = getStoredAuthSession();
  const response = await fetch(`${baseUrl}${path}`, {
    ...init,
    headers: {
      ...(init?.body ? { "Content-Type": "application/json" } : {}),
      ...(session ? { Authorization: `Bearer ${session.accessToken}` } : {}),
      ...init?.headers,
    },
  });

  if (response.status === 401 && retryWithRefresh && baseUrl !== AUTH_API_URL && session) {
    const refreshResponse = await fetch(`${AUTH_API_URL}/auth/refresh`, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ refreshToken: session.refreshToken }),
    });
    if (refreshResponse.ok) {
      const tokens = (await refreshResponse.json()) as TokenResponse;
      if (tokens.accessToken && tokens.refreshToken) {
        storeAuthSession({
          ...session,
          accessToken: tokens.accessToken,
          refreshToken: tokens.refreshToken,
          accessTokenExpiresAt: Date.now() + tokens.expiresIn * 1000,
        });
        return request<T>(baseUrl, path, init, false);
      }
    } else if (refreshResponse.status === 401) {
      localStorage.removeItem(AUTH_SESSION_KEY);
    }
  }

  if (!response.ok) {
    let message = `Error ${response.status} al consultar ${path}`;
    try {
      const body = (await response.json()) as { error?: string; message?: string };
      message = body.error ?? body.message ?? message;
    } catch {
      // Keep the HTTP status message when an endpoint does not return JSON.
    }
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) return undefined as T;
  return (await response.json()) as T;
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
  const tokens = await request<TokenResponse>(AUTH_API_URL, "/auth/login", {
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
  const tokens = await request<TokenResponse>(AUTH_API_URL, "/auth/register", {
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
      await request<{ valid: boolean }>(AUTH_API_URL, "/auth/validate");
      return session;
    } catch (error) {
      if (!(error instanceof ApiError) || error.status !== 401) throw error;
    }
  }

  try {
    const tokens = await request<TokenResponse>(AUTH_API_URL, "/auth/refresh", {
      method: "POST",
      body: JSON.stringify({ refreshToken: session.refreshToken }),
    });
    return createSession(tokens);
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) {
      localStorage.removeItem(AUTH_SESSION_KEY);
      return null;
    }
    throw error;
  }
}

export async function logoutAccount(): Promise<void> {
  const session = getStoredAuthSession();
  try {
    if (session) {
      await request<void>(AUTH_API_URL, "/auth/logout", {
        method: "POST",
        body: JSON.stringify({ refreshToken: session.refreshToken }),
      });
    }
  } finally {
    localStorage.removeItem(AUTH_SESSION_KEY);
  }
}

export async function getSearchHistory(userId: number): Promise<SearchHistoryEntry[]> {
  return request<SearchHistoryEntry[]>(SEARCH_API_URL, `/search/history?userId=${userId}`);
}

export async function addSearchHistory(
  userId: number,
  query: string,
  type: "product" | "service",
): Promise<void> {
  await request<void>(SEARCH_API_URL, "/search/history", {
    method: "POST",
    body: JSON.stringify({ userId, query, type }),
  });
}

export function isApiNotFound(error: unknown): boolean {
  return error instanceof ApiError && error.status === 404;
}

export async function getFavoriteProductIds(userId: number): Promise<string[]> {
  const favorites = await request<Array<{ productId: string }>>(
    FAVORITES_API_URL,
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
  await request<void>(FAVORITES_API_URL, path, {
    method: favorite ? "POST" : "DELETE",
    ...(favorite ? { body: JSON.stringify({ userId }) } : {}),
  });
}

export async function getFavoriteServiceIds(userId: number): Promise<string[]> {
  const favorites = await request<Array<{ serviceId: string }>>(
    FAVORITES_API_URL,
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
  await request<void>(FAVORITES_API_URL, path, {
    method: favorite ? "POST" : "DELETE",
    ...(favorite ? { body: JSON.stringify({ userId }) } : {}),
  });
}

export async function getCartItems(userId: number): Promise<AnyCartItem[]> {
  const items = await request<ApiCartItem[]>(CART_API_URL, `/cart?userId=${userId}`);
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
  await request(CART_API_URL, "/cart/items", {
    method: "POST",
    body: JSON.stringify({ userId, productId, offerId, quantity: 1 }),
  });
}

export async function addServiceCartItem(
  userId: number,
  serviceId: string,
  serviceOfferId: number,
): Promise<void> {
  await request(CART_API_URL, "/cart/items", {
    method: "POST",
    body: JSON.stringify({ userId, serviceId, serviceOfferId, quantity: 1 }),
  });
}

export async function updateCartItem(
  userId: number,
  cartItemId: number,
  quantity: number,
): Promise<void> {
  await request(CART_API_URL, `/cart/items/${cartItemId}`, {
    method: "PUT",
    body: JSON.stringify({ userId, quantity }),
  });
}

export async function removeCartItem(userId: number, cartItemId: number): Promise<void> {
  await request(CART_API_URL, `/cart/items/${cartItemId}?userId=${userId}`, {
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
  return request<ApiComparison>(COMPARISON_API_URL, `/comparisons/products/${id}`);
}

export async function createProductComparison(userId: number, productId: string): Promise<number> {
  const result = await request<{ id: number }>(COMPARISON_API_URL, "/comparisons/products", {
    method: "POST",
    body: JSON.stringify({ userId, productIds: [productId] }),
  });
  return result.id;
}

export async function addProductToComparison(
  comparisonId: number,
  productId: string,
): Promise<void> {
  await request(COMPARISON_API_URL, `/comparisons/products/${comparisonId}/items`, {
    method: "POST",
    body: JSON.stringify({ productId }),
  });
}

export async function removeProductFromComparison(
  comparisonId: number,
  productId: string,
): Promise<void> {
  await request(
    COMPARISON_API_URL,
    `/comparisons/products/${comparisonId}/items/${encodeURIComponent(productId)}`,
    { method: "DELETE" },
  );
}

export async function deleteProductComparison(comparisonId: number): Promise<void> {
  await request(COMPARISON_API_URL, `/comparisons/products/${comparisonId}`, {
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
  return request<ApiServiceComparison>(COMPARISON_API_URL, `/comparisons/services/${id}`);
}

export async function createServiceComparison(userId: number, serviceId: string): Promise<number> {
  const result = await request<{ id: number }>(COMPARISON_API_URL, "/comparisons/services", {
    method: "POST",
    body: JSON.stringify({ userId, serviceIds: [serviceId] }),
  });
  return result.id;
}

export async function addServiceToComparison(
  comparisonId: number,
  serviceId: string,
): Promise<void> {
  await request(COMPARISON_API_URL, `/comparisons/services/${comparisonId}/items`, {
    method: "POST",
    body: JSON.stringify({ serviceId }),
  });
}

export async function removeServiceFromComparison(
  comparisonId: number,
  serviceId: string,
): Promise<void> {
  await request(
    COMPARISON_API_URL,
    `/comparisons/services/${comparisonId}/items/${encodeURIComponent(serviceId)}`,
    { method: "DELETE" },
  );
}

export async function deleteServiceComparison(comparisonId: number): Promise<void> {
  await request(COMPARISON_API_URL, `/comparisons/services/${comparisonId}`, {
    method: "DELETE",
  });
}
