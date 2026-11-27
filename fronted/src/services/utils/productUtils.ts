import type { PricePoint, Product, StoreOffer } from "../../types";

const clpFormatter = new Intl.NumberFormat("es-CL", {
  style: "currency",
  currency: "CLP",
  maximumFractionDigits: 0,
});

/**
 * Formatea un precio en pesos chilenos sin decimales.
 *
 * @param price Precio numérico que se quiere mostrar.
 * @returns El precio formateado o un mensaje cuando no está disponible.
 */
export const formatPrice = (price: number | null | undefined): string =>
  price === null || price === undefined || !Number.isFinite(price)
    ? "Precio no disponible"
    : clpFormatter.format(price);

export const STALE_PRICE_DAYS = 7;

/**
 * Convierte una fecha válida y pasada en una marca de tiempo.
 *
 * @param value Fecha de actualización en texto.
 * @returns La marca de tiempo o `undefined` si falta o no es una fecha pasada válida.
 */
const getPastTimestamp = (value: string | null | undefined): number | undefined => {
  if (!value) return undefined;
  const timestamp = Date.parse(value);
  return Number.isFinite(timestamp) && timestamp <= Date.now() ? timestamp : undefined;
};

/**
 * Expresa una antigüedad en unidades relativas legibles en español.
 *
 * @param timestamp Marca de tiempo de la actualización.
 * @returns Un texto como «ayer» o «hace 3 horas».
 */
const relativeTime = (timestamp: number): string => {
  const elapsedSeconds = Math.round((Date.now() - timestamp) / 1000);
  if (elapsedSeconds < 60) return "hace menos de un minuto";

  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ["year", 31_536_000],
    ["month", 2_592_000],
    ["day", 86_400],
    ["hour", 3_600],
    ["minute", 60],
  ];
  const [unit, secondsPerUnit] = units.find(([, seconds]) => elapsedSeconds >= seconds)!;
  const count = Math.round(elapsedSeconds / secondsPerUnit);
  return new Intl.RelativeTimeFormat("es", { numeric: "auto" }).format(-count, unit);
};

/**
 * Devuelve una descripción relativa de la última actualización.
 *
 * @param value Fecha de actualización en texto.
 * @returns La antigüedad formateada o un aviso cuando la fecha no está disponible.
 */
export const relativeUpdate = (value: string | null | undefined): string => {
  const timestamp = getPastTimestamp(value);
  return timestamp === undefined
    ? "Fecha de actualización no disponible"
    : `Actualizado ${relativeTime(timestamp)}`;
};

/**
 * Comprueba si una oferta lleva más de siete días sin actualizarse.
 *
 * @param value Fecha de actualización en texto.
 * @returns `true` cuando la fecha pasada supera el umbral de desactualización.
 */
export const isStalePrice = (value: string | null | undefined): boolean => {
  const timestamp = getPastTimestamp(value);
  return timestamp !== undefined && Date.now() - timestamp > STALE_PRICE_DAYS * 24 * 60 * 60 * 1000;
};

/**
 * Formatea la antigüedad de una oferta y marca las precios desactualizados.
 *
 * @param value Fecha de actualización en texto.
 * @returns La antigüedad, «Desactualizado» o un aviso de fecha no disponible.
 */
export const formatPriceUpdate = (value: string | null | undefined): string => {
  const timestamp = getPastTimestamp(value);
  return timestamp === undefined
    ? "Fecha de actualización no disponible"
    : isStalePrice(value)
      ? "Desactualizado"
      : relativeTime(timestamp);
};

/**
 * Formatea una fecha pasada válida con fecha y hora locales.
 *
 * @param value Fecha de actualización en texto.
 * @returns La fecha legible o `undefined` cuando no se puede interpretar.
 */
export const formatExactUpdateDate = (value: string | null | undefined): string | undefined => {
  const timestamp = getPastTimestamp(value);
  return timestamp === undefined ? undefined : new Date(timestamp).toLocaleString("es-CL");
};

/**
 * Conserva una sola oferta por tienda: la que tiene la actualización más reciente.
 *
 * @param offers Ofertas del producto, incluidas posibles duplicados de una tienda.
 * @returns Las ofertas seleccionadas, conservando el orden de aparición de cada tienda.
 */
export const keepLatestOfferPerStore = (offers: readonly StoreOffer[]): StoreOffer[] => {
  const latestByStore = new Map<string, StoreOffer>();

  for (const offer of offers) {
    const current = latestByStore.get(offer.storeId);
    if (!current) {
      latestByStore.set(offer.storeId, offer);
      continue;
    }

    const currentTimestamp = getPastTimestamp(current.lastUpdated);
    const offerTimestamp = getPastTimestamp(offer.lastUpdated);
    if (
      offerTimestamp !== undefined &&
      (currentTimestamp === undefined || offerTimestamp > currentTimestamp)
    ) {
      latestByStore.set(offer.storeId, offer);
    }
  }

  return [...latestByStore.values()];
};

export interface OfferComparisonValidation {
  validPriceOffers: StoreOffer[];
  availableOffers: StoreOffer[];
  currencyCompatible: boolean;
  hasMixedCurrencies: boolean;
  canCompare: boolean;
}

/**
 * Valida precios, disponibilidad y monedas antes de comparar ofertas.
 *
 * @param offers Ofertas que se quieren validar.
 * @returns Los subconjuntos válidos y los indicadores de compatibilidad.
 */
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

/**
 * Normaliza el historial de precios y descarta puntos inválidos o futuros.
 *
 * @param points Historial original de precios.
 * @returns Los puntos válidos ordenados cronológicamente.
 */
export const normalizePriceHistory = (points: readonly PricePoint[]): PricePoint[] => {
  const now = Date.now();

  return points
    .map((point, index) => ({
      point,
      index,
      timestamp: Date.parse(point.recordedAt ?? point.date),
    }))
    .filter(({ point, timestamp }) => {
      // Se descarta como error de extracción aunque la base permita almacenar 0.
      return (
        Number.isFinite(point.price) &&
        point.price > 0 &&
        Number.isFinite(timestamp) &&
        timestamp <= now
      );
    })
    .sort((first, second) => first.timestamp - second.timestamp || first.index - second.index)
    .map(({ point }) => point);
};

export const ANOMALY_MIN_OFFERS = 3;
export const ANOMALY_LOW_RATIO = 0.5;
export const ANOMALY_MIN_HISTORY_POINTS = 3;
export const ANOMALY_HISTORY_DROP_RATIO = 0.5;

/**
 * Detecta precios inusuales respecto de otras ofertas y caídas bruscas del historial.
 *
 * @param offers Ofertas actuales del producto.
 * @param history Historial de precios del producto.
 * @returns Los identificadores de ofertas anómalas y si hubo una caída brusca.
 */
export const detectPriceAnomalies = (
  offers: readonly StoreOffer[],
  history: readonly PricePoint[],
): { anomalousOfferIds: ReadonlySet<number>; hasAbruptHistoryDrop: boolean } => {
  const validation = validateOfferComparison(offers);
  const anomalousOfferIds = new Set<number>();

  if (validation.currencyCompatible && validation.availableOffers.length >= ANOMALY_MIN_OFFERS) {
    for (const offer of validation.availableOffers) {
      const peerPrices = validation.availableOffers
        .filter((peer) => peer !== offer)
        .flatMap((peer) => (peer.price === null ? [] : [peer.price]))
        .sort((first, second) => first - second);
      const middle = Math.floor(peerPrices.length / 2);
      const median =
        peerPrices.length % 2 === 0
          ? (peerPrices[middle - 1] + peerPrices[middle]) / 2
          : peerPrices[middle];

      if (offer.price !== null && offer.price < median * ANOMALY_LOW_RATIO) {
        anomalousOfferIds.add(offer.offerId);
      }
    }
  }

  const validHistory = normalizePriceHistory(history);
  const historyByOffer = new Map<number, PricePoint[]>();
  for (const point of validHistory) {
    if (point.offerId === undefined) continue;
    const offerHistory = historyByOffer.get(point.offerId) ?? [];
    offerHistory.push(point);
    historyByOffer.set(point.offerId, offerHistory);
  }

  /**
   * Comprueba si los últimos puntos de un historial muestran una caída abrupta.
   *
   * @param points Puntos válidos de una oferta o del producto.
   * @returns `true` cuando el último precio cayó al menos a la mitad del anterior.
   */
  const hasAbruptDrop = (points: readonly PricePoint[]): boolean =>
    points.length >= ANOMALY_MIN_HISTORY_POINTS &&
    points[points.length - 1].price <= points[points.length - 2].price * ANOMALY_HISTORY_DROP_RATIO;
  const hasAbruptHistoryDrop =
    historyByOffer.size > 0
      ? [...historyByOffer.values()].some(hasAbruptDrop)
      : hasAbruptDrop(validHistory);

  return { anomalousOfferIds, hasAbruptHistoryDrop };
};

/**
 * Devuelve el sufijo visible para un periodo de facturación.
 *
 * @param period Periodo técnico del servicio.
 * @returns El sufijo en español o el valor mensual por defecto.
 */
export const getBillingPeriodText = (period: string | null | undefined): string => {
  if (period === "yearly") return "/año";
  if (period === "one_time") return "pago único";
  return "/mes";
};

/**
 * Devuelve el nombre legible de un periodo de facturación.
 *
 * @param period Periodo técnico del servicio.
 * @returns «anual», «mensual» o «pago único».
 */
export const getBillingPeriodName = (period: string | null | undefined): string => {
  if (period === "yearly") return "anual";
  if (period === "one_time") return "pago único";
  return "mensual";
};

/**
 * Calcula el menor precio entre ofertas disponibles y comparables.
 *
 * @param product Producto cuyas ofertas se quieren evaluar.
 * @returns El menor precio o `null` si no hay ofertas comparables.
 */
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

/**
 * Selecciona la oferta más barata entre las más recientes de cada tienda.
 *
 * @param product Producto cuyas ofertas se quieren evaluar.
 * @returns La mejor oferta comparable o `undefined` si no existe.
 */
export const getMinOffer = (product: Product) => {
  const validation = validateOfferComparison(keepLatestOfferPerStore(product.offers));
  if (!validation.currencyCompatible) return undefined;
  return [...validation.availableOffers].sort((a, b) => a.price! - b.price!)[0];
};

/**
 * Cuenta cuántas ofertas del producto están disponibles.
 *
 * @param product Producto cuyas ofertas se quieren contar.
 * @returns La cantidad de ofertas disponibles.
 */
export const getAvailableStoreCount = (product: Product): number =>
  product.offers.filter((offer) => offer.available).length;
