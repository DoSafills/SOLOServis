import type { Product } from "../../types";

export const formatPrice = (price: number): string => `$${price.toLocaleString("es-CL")}`;

export const formatServicePrice = (price: number, currency = "CLP"): string =>
  new Intl.NumberFormat("es-CL", {
    style: "currency",
    currency,
    maximumFractionDigits: currency === "CLP" ? 0 : 2,
  }).format(price);

export const getServiceBillingPeriodLabel = (billingPeriod?: string | null): string => {
  switch (billingPeriod) {
    case "monthly":
      return "/mes";
    case "yearly":
      return "/año";
    case "one_time":
      return "pago único";
    default:
      return "";
  }
};

export const getMinPrice = (product: Product): number => {
  const availableOffers = product.offers.filter((offer) => offer.available);

  if (availableOffers.length === 0) {
    return 0;
  }

  return Math.min(...availableOffers.map((offer) => offer.price));
};

export const getMinOffer = (product: Product) => {
  return product.offers.filter((offer) => offer.available).sort((a, b) => a.price - b.price)[0];
};

export const getAvailableStoreCount = (product: Product): number =>
  product.offers.filter((offer) => offer.available).length;
