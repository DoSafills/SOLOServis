import type { Product, StoreOffer } from "../../types";

const clpFormatter = new Intl.NumberFormat("es-CL", {
  style: "currency",
  currency: "CLP",
  maximumFractionDigits: 0,
});

export const formatPrice = (price: number | null | undefined): string =>
  price === null || price === undefined || !Number.isFinite(price)
    ? "Precio no disponible"
    : clpFormatter.format(price);

export interface OfferComparisonValidation {
  validPriceOffers: StoreOffer[];
  availableOffers: StoreOffer[];
  currencyCompatible: boolean;
  hasMixedCurrencies: boolean;
  canCompare: boolean;
}

export const validateOfferComparison = (
  offers: readonly StoreOffer[],
): OfferComparisonValidation => {
  const validPriceOffers = offers.filter(
    (offer) => typeof offer.price === "number" && Number.isFinite(offer.price) && offer.price > 0,
  );
  const availableOffers = validPriceOffers.filter((offer) => offer.available);
  const currencies = new Set(
    validPriceOffers.map((offer) =>
      typeof offer.currency === "string" ? offer.currency.trim().toUpperCase() : "",
    ),
  );
  const hasMixedCurrencies = currencies.size > 1;
  const currencyCompatible = currencies.size === 1 && currencies.has("CLP");

  return {
    validPriceOffers,
    availableOffers,
    currencyCompatible,
    hasMixedCurrencies,
    canCompare: availableOffers.length >= 2 && currencyCompatible,
  };
};

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
  const validation = validateOfferComparison(product.offers);

  if (!validation.currencyCompatible || validation.availableOffers.length === 0) {
    return null;
  }

  const prices = validation.availableOffers.flatMap((offer) =>
    offer.price === null ? [] : [offer.price],
  );
  return Math.min(...prices);
};

export const getMinOffer = (product: Product) => {
  const validation = validateOfferComparison(product.offers);
  if (!validation.currencyCompatible) return undefined;
  return [...validation.availableOffers].sort((a, b) => a.price! - b.price!)[0];
};

export const getAvailableStoreCount = (product: Product): number =>
  product.offers.filter((offer) => offer.available).length;
