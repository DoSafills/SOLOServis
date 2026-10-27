import type { Product } from "../../types";

const clpFormatter = new Intl.NumberFormat("es-CL", {
  style: "currency",
  currency: "CLP",
  maximumFractionDigits: 0,
});

export const formatPrice = (price: number): string => clpFormatter.format(price);

export const getBillingPeriodText = (period: string | null | undefined): string => {
  if (period === "yearly") return "/año";
  if (period === "one_time") return "pago único";
  return "/mes";
};

export const getBillingPeriodName = (period: string | null | undefined): string => {
  if (period === "yearly") return "anual";
  if (period === "one_time") return "pago único";
  return "mensual";
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
