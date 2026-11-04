import type { Product, StoreOffer } from "../../types";

const clpFormatter = new Intl.NumberFormat("es-CL", {
  style: "currency",
  currency: "CLP",
  maximumFractionDigits: 0,
});

export const formatPrice = (price: number | null | undefined): string =>
  price === null || price === undefined || !Number.isFinite(price)
    ? "Sin precio"
    : clpFormatter.format(price);

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

export const getMinPrice = (product: Product): number | null => {
  const availableOffers = product.offers
    .filter((offer) => offer.available)
    .map((offer) => offer.price)
    .filter((price): price is number => price !== null && Number.isFinite(price));

  return availableOffers.length === 0 ? null : Math.min(...availableOffers);
};

export const getMinOffer = (product: Product) => {
  return product.offers
    .filter(
      (offer): offer is StoreOffer & { price: number } =>
        offer.available && offer.price !== null && Number.isFinite(offer.price),
    )
    .sort((a, b) => a.price - b.price)[0];
};

export const getAvailableStoreCount = (product: Product): number =>
  product.offers.filter((offer) => offer.available).length;
