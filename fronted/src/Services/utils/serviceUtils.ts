import { formatPrice } from "./productUtils";

/** null significa que el proveedor no informa costo de instalación. */
export const formatInstallation = (cost: number | null): string => {
  if (cost === 0) return "Gratis";
  return cost ? formatPrice(cost) : "Sin costo";
};

export const formatContract = (months: number | null): string =>
  months ? `${months} meses` : "Sin permanencia";
