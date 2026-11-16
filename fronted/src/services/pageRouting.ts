import type { Page, Product, PricePoint, StoreOffer } from "../types";

const OFFER_COMPARISON_STORAGE_PREFIX = "soloservice.offer-comparison.";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null;
}

function isStringArray(value: unknown): value is string[] {
  return Array.isArray(value) && value.every((item) => typeof item === "string");
}

function isPricePoint(value: unknown): value is PricePoint {
  return (
    isRecord(value) &&
    typeof value.date === "string" &&
    typeof value.price === "number" &&
    Number.isFinite(value.price)
  );
}

function isStoreOffer(value: unknown): value is StoreOffer {
  return (
    isRecord(value) &&
    typeof value.offerId === "number" &&
    typeof value.storeId === "string" &&
    typeof value.storeName === "string" &&
    typeof value.price === "number" &&
    typeof value.available === "boolean"
  );
}

function isProduct(value: unknown): value is Product {
  return (
    isRecord(value) &&
    typeof value.id === "string" &&
    typeof value.name === "string" &&
    typeof value.brand === "string" &&
    typeof value.model === "string" &&
    typeof value.category === "string" &&
    typeof value.subcategory === "string" &&
    typeof value.image === "string" &&
    isStringArray(value.images) &&
    typeof value.description === "string" &&
    typeof value.rating === "number" &&
    typeof value.reviewCount === "number" &&
    isRecord(value.specs) &&
    Array.isArray(value.offers) &&
    value.offers.every(isStoreOffer) &&
    Array.isArray(value.priceHistory) &&
    value.priceHistory.every(isPricePoint) &&
    Array.isArray(value.offerPriceHistory) &&
    value.offerPriceHistory.every(isPricePoint) &&
    isStringArray(value.tags)
  );
}

function isPage(value: unknown): value is Page {
  if (!isRecord(value) || typeof value.id !== "string") return false;

  switch (value.id) {
    case "home":
    case "product-categories":
    case "service-categories":
    case "stores":
    case "favorites":
    case "cart":
      return true;
    case "search-products":
      return (
        typeof value.query === "string" &&
        (value.category === undefined || typeof value.category === "string") &&
        (value.categoryContext === undefined || typeof value.categoryContext === "string") &&
        (value.productGroup === undefined || typeof value.productGroup === "string") &&
        (value.categoryId === undefined ||
          (typeof value.categoryId === "number" && Number.isInteger(value.categoryId)))
      );
    case "product-detail":
      return typeof value.productId === "string";
    case "product-comparison":
      return isStringArray(value.productIds);
    case "offer-comparison":
      return isProduct(value.product) && isStringArray(value.storeIds);
    case "search-services":
      return (
        typeof value.query === "string" &&
        (value.category === undefined || typeof value.category === "string") &&
        (value.group === undefined || typeof value.group === "string")
      );
    case "service-detail":
      return typeof value.serviceId === "string";
    case "service-comparison":
      return isStringArray(value.serviceIds);
    case "store-detail":
      return typeof value.storeId === "string";
    case "user":
      return (
        value.section === undefined ||
        value.section === "profile" ||
        value.section === "favorites" ||
        value.section === "history"
      );
    default:
      return false;
  }
}

function addStringParameter(params: URLSearchParams, key: string, value: string | undefined) {
  if (value !== undefined) params.set(key, value);
}

function pageUrl(page: Page): string {
  if (page.id === "home") return window.location.pathname;

  const params = new URLSearchParams({ page: page.id });
  switch (page.id) {
    case "search-products":
      params.set("query", page.query);
      addStringParameter(params, "category", page.category);
      addStringParameter(params, "categoryContext", page.categoryContext);
      addStringParameter(params, "productGroup", page.productGroup);
      if (page.categoryId !== undefined) params.set("categoryId", String(page.categoryId));
      break;
    case "product-detail":
      params.set("productId", page.productId);
      break;
    case "product-comparison":
      page.productIds.forEach((id) => params.append("productId", id));
      break;
    case "offer-comparison":
      params.set("productId", page.product.id);
      page.storeIds.forEach((id) => params.append("storeId", id));
      break;
    case "search-services":
      params.set("query", page.query);
      addStringParameter(params, "category", page.category);
      addStringParameter(params, "group", page.group);
      break;
    case "service-detail":
      params.set("serviceId", page.serviceId);
      break;
    case "service-comparison":
      page.serviceIds.forEach((id) => params.append("serviceId", id));
      break;
    case "store-detail":
      params.set("storeId", page.storeId);
      break;
    case "user":
      addStringParameter(params, "section", page.section);
      break;
    default:
      break;
  }
  return `${window.location.pathname}?${params.toString()}`;
}

function pageFromUrl(): Page {
  const params = new URLSearchParams(window.location.search);
  const id = params.get("page");
  const readList = (key: string) => params.getAll(key).filter(Boolean);

  switch (id) {
    case "product-categories":
      return { id };
    case "service-categories":
      return { id };
    case "search-products": {
      const categoryId = Number(params.get("categoryId"));
      return {
        id,
        query: params.get("query") ?? "",
        ...(params.has("category") ? { category: params.get("category") ?? "" } : {}),
        ...(params.has("categoryContext")
          ? { categoryContext: params.get("categoryContext") ?? "" }
          : {}),
        ...(params.has("productGroup") ? { productGroup: params.get("productGroup") ?? "" } : {}),
        ...(Number.isInteger(categoryId) && categoryId > 0 ? { categoryId } : {}),
      };
    }
    case "product-detail": {
      const productId = params.get("productId");
      return productId ? { id, productId } : { id: "home" };
    }
    case "product-comparison":
      return { id, productIds: readList("productId") };
    case "offer-comparison": {
      const productId = params.get("productId");
      if (productId) {
        try {
          const stored: unknown = JSON.parse(
            sessionStorage.getItem(`${OFFER_COMPARISON_STORAGE_PREFIX}${productId}`) ?? "null",
          );
          if (
            isPage(stored) &&
            stored.id === "offer-comparison" &&
            stored.product.id === productId
          ) {
            return stored;
          }
        } catch {
          return productId ? { id: "product-detail", productId } : { id: "home" };
        }
      }
      return productId ? { id: "product-detail", productId } : { id: "home" };
    }
    case "search-services":
      return {
        id,
        query: params.get("query") ?? "",
        ...(params.has("category") ? { category: params.get("category") ?? "" } : {}),
        ...(params.has("group") ? { group: params.get("group") ?? "" } : {}),
      };
    case "service-detail": {
      const serviceId = params.get("serviceId");
      return serviceId ? { id, serviceId } : { id: "home" };
    }
    case "service-comparison":
      return { id, serviceIds: readList("serviceId") };
    case "stores":
      return { id };
    case "store-detail": {
      const storeId = params.get("storeId");
      return storeId ? { id, storeId } : { id: "home" };
    }
    case "favorites":
      return { id };
    case "user": {
      const section = params.get("section");
      return {
        id,
        ...(section === "profile" || section === "favorites" || section === "history"
          ? { section }
          : {}),
      };
    }
    case "cart":
      return { id };
    default:
      return { id: "home" };
  }
}

export function getPageFromHistoryState(state: unknown): Page | null {
  if (!isRecord(state) || !isPage(state.page)) return null;
  return state.page;
}

export function getInitialPage(): Page {
  return getPageFromHistoryState(window.history.state) ?? pageFromUrl();
}

export function pushPage(page: Page): void {
  if (page.id === "offer-comparison") {
    try {
      sessionStorage.setItem(
        `${OFFER_COMPARISON_STORAGE_PREFIX}${page.product.id}`,
        JSON.stringify(page),
      );
    } catch (error) {
      console.error(
        "No se pudo guardar la comparación de ofertas para restaurarla al recargar.",
        error,
      );
    }
  }
  window.history.pushState({ page }, "", pageUrl(page));
}

export function replacePage(page: Page): void {
  window.history.replaceState({ page }, "", pageUrl(page));
}
