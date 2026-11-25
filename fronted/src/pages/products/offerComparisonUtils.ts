import type { StoreOffer } from "../../types";
import { isStalePrice, type OfferComparisonValidation } from "../../services/utils/productUtils";

export type OfferAdvantageTone = "prime" | "success" | "neutral" | "warn" | "danger";

export interface OfferAdvantage {
  id: string;
  label: string;
  tone: OfferAdvantageTone;
  title?: string;
}

export interface OfferComparisonHighlights {
  totalsByOfferId: ReadonlyMap<number, number | null>;
  bestTotalOfferIds: ReadonlySet<number>;
  bestPriceOfferIds: ReadonlySet<number>;
  hasUnknownShipping: boolean;
}

export type OfferWarrantyState = "yes" | "no" | "unknown";

/**
 * Calcula el total de una oferta solo cuando su precio, moneda y envío son válidos.
 *
 * @param offer Oferta cuyo total se quiere calcular.
 * @param validation Resultado de validar el conjunto de ofertas comparables.
 * @returns El precio más envío, o `null` cuando el total no es determinable.
 */
export const getOfferTotal = (
  offer: StoreOffer,
  validation: OfferComparisonValidation,
): number | null => {
  if (
    !validation.validPriceOffers.includes(offer) ||
    !validation.currencyCompatible ||
    offer.price === null
  ) {
    return null;
  }

  const shipping = offer.shippingFree ? 0 : offer.shipping;
  if (shipping === null || !Number.isFinite(shipping) || shipping < 0) return null;
  const total = offer.price + shipping;
  return Number.isFinite(total) && total > 0 ? total : null;
};

/**
 * Determina qué ofertas tienen el menor precio y el menor total sin ocultar empates.
 *
 * @param offers Ofertas que se están comparando.
 * @param validation Resultado de validar precio, disponibilidad y moneda.
 * @returns Totales por oferta, conjuntos de ganadoras y si falta algún envío.
 */
export const getOfferComparisonHighlights = (
  offers: readonly StoreOffer[],
  validation: OfferComparisonValidation,
): OfferComparisonHighlights => {
  const totalsByOfferId = new Map<number, number | null>();
  for (const offer of offers) {
    totalsByOfferId.set(offer.offerId, getOfferTotal(offer, validation));
  }

  const hasUnknownShipping = validation.availableOffers.some(
    (offer) => getOfferTotal(offer, validation) === null,
  );
  const comparableTotals = validation.availableOffers
    .map((offer) => ({ offer, total: getOfferTotal(offer, validation) }))
    .filter((entry): entry is { offer: StoreOffer; total: number } => entry.total !== null);
  const bestTotalOfferIds = new Set<number>();

  if (validation.canCompare && !hasUnknownShipping && comparableTotals.length > 0) {
    const lowestTotal = Math.min(...comparableTotals.map((entry) => entry.total));
    for (const entry of comparableTotals) {
      if (entry.total === lowestTotal) bestTotalOfferIds.add(entry.offer.offerId);
    }
  }

  const bestPriceOfferIds = new Set<number>();
  const comparablePrices = validation.availableOffers
    .map((offer) => offer.price)
    .filter((price): price is number => price !== null);

  if (validation.canCompare && comparablePrices.length > 0) {
    const lowestPrice = Math.min(...comparablePrices);
    for (const offer of validation.availableOffers) {
      if (offer.price === lowestPrice) bestPriceOfferIds.add(offer.offerId);
    }
  }

  return {
    totalsByOfferId,
    bestTotalOfferIds,
    bestPriceOfferIds,
    hasUnknownShipping,
  };
};

/**
 * Clasifica la garantía sin confundir un dato faltante con una garantía inexistente.
 *
 * @param value Texto de garantía entregado por la tienda.
 * @returns `yes` si existe garantía, `no` si se declara su ausencia y `unknown` si no se informa.
 */
export const getOfferWarrantyState = (value: string | undefined): OfferWarrantyState => {
  const normalized = value?.trim().toLocaleLowerCase("es") ?? "";
  if (!normalized || ["no informado", "no disponible", "n/a", "na", "-"].includes(normalized)) {
    return "unknown";
  }

  const explicitAbsence = [
    /^sin\s+(garant[ií]a|cobertura)$/,
    /^no\s+(incluye\s+|tiene\s+)?(garant[ií]a|cobertura)$/,
    /^(garant[ií]a|cobertura)\s+no\s+aplica$/,
    /^(no|sin)$/,
    /^(no\s+aplica|ninguna|cero|0)(\s+(mes|meses|d[ií]a|d[ií]as|a[nñ]o|a[nñ]os))?$/,
  ].some((pattern) => pattern.test(normalized));

  return explicitAbsence ? "no" : "yes";
};

/**
 * Devuelve la etiqueta breve que corresponde al estado de garantía.
 *
 * @param value Texto de garantía entregado por la tienda.
 * @returns Una etiqueta explícita de garantía presente, ausente o no informada.
 */
export const getOfferWarrantyLabel = (value: string | undefined): string => {
  const state = getOfferWarrantyState(value);
  if (state === "yes") return "Con garantía";
  if (state === "no") return "Sin garantía";
  return "No informado";
};

/**
 * Reúne las ventajas y advertencias visibles de una oferta para el comparador.
 *
 * @param offer Oferta de la tienda que se está mostrando.
 * @param highlights Resultados de menor precio y menor total del conjunto.
 * @param anomalousOfferIds Identificadores de ofertas con precios inusuales.
 * @returns Etiquetas ordenadas con su tono visual y contexto opcional.
 */
export const getOfferAdvantages = (
  offer: StoreOffer,
  highlights: OfferComparisonHighlights,
  anomalousOfferIds: ReadonlySet<number>,
): OfferAdvantage[] => {
  const advantages: OfferAdvantage[] = [];

  if (highlights.bestTotalOfferIds.has(offer.offerId)) {
    advantages.push({
      id: "lowest-total",
      label: "Menor total",
      tone: "prime",
      title: "Precio del producto más el costo de envío.",
    });
  }
  if (highlights.bestPriceOfferIds.has(offer.offerId)) {
    advantages.push({
      id: "lowest-price",
      label: "Mejor precio",
      tone: "success",
      title: "Precio del producto sin considerar el envío.",
    });
  }
  if (
    offer.price !== null &&
    offer.price > 0 &&
    offer.listPrice !== null &&
    offer.listPrice !== undefined &&
    offer.listPrice > offer.price
  ) {
    advantages.push({ id: "discounted", label: "Precio oferta", tone: "neutral" });
  }
  if (offer.available && offer.shippingFree) {
    advantages.push({
      id: "free-shipping",
      label: "Envío gratis",
      tone: "success",
      title: "La tienda declara envío sin costo.",
    });
  }
  if (offer.available && !offer.shippingFree && offer.shipping === null) {
    advantages.push({
      id: "unknown-shipping",
      label: "Envío no informado",
      tone: "warn",
      title: "No se puede calcular el total final de esta oferta.",
    });
  }

  const warrantyState = getOfferWarrantyState(offer.warranty);
  if (offer.available && warrantyState === "yes") {
    advantages.push({
      id: "warranty",
      label: "Con garantía",
      tone: "success",
      title: offer.warranty ?? "La tienda informa una garantía.",
    });
  }
  if (offer.available && warrantyState === "no") {
    advantages.push({
      id: "no-warranty",
      label: "Sin garantía",
      tone: "warn",
      title: offer.warranty ?? "La tienda declara que no hay garantía.",
    });
  }
  if (offer.available && offer.condition?.trim().toLowerCase() === "new") {
    advantages.push({ id: "new-condition", label: "Nuevo", tone: "success" });
  }
  if (offer.available) {
    advantages.push({ id: "available", label: "Disponible", tone: "success" });
  } else {
    advantages.push({ id: "unavailable", label: "No disponible", tone: "danger" });
  }
  if (anomalousOfferIds.has(offer.offerId)) {
    advantages.push({
      id: "anomalous-price",
      label: "Precio inusual",
      tone: "warn",
      title: "Menos de la mitad de la mediana de otras ofertas.",
    });
  }
  if (isStalePrice(offer.lastUpdated)) {
    advantages.push({
      id: "stale-price",
      label: "Desactualizado",
      tone: "warn",
      title: "La verificación tiene más de siete días de antigüedad.",
    });
  }

  return advantages;
};
