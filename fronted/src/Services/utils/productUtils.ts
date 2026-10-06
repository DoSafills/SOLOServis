import type { Product } from "../../types";

export const formatPrice = (price: number): string => `$${price.toLocaleString("es-CL")}`;

const availableOffers = (product: Product) => product.offers.filter((offer) => offer.available);

/** Precio más bajo entre las ofertas disponibles, o 0 si no hay ninguna. */
export const getMinPrice = (product: Product): number => {
  const offers = availableOffers(product);

  return offers.length === 0 ? 0 : Math.min(...offers.map((offer) => offer.price));
};

/** Oferta disponible más barata, o undefined si no hay ninguna. */
export const getMinOffer = (product: Product) =>
  [...availableOffers(product)].sort((a, b) => a.price - b.price)[0];

export const getAvailableStoreCount = (product: Product): number => availableOffers(product).length;
