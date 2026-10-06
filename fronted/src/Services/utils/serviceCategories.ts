import type { Service } from "../../types";

export type ServiceMacroCategory = "internet" | "insurance" | "technical" | "education";

const normalize = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase()
    .trim();

export const getServiceMacroCategory = (service: Service): ServiceMacroCategory | "other" => {
  const category = normalize(`${service.category} ${service.subcategory}`);
  if (/internet|fibra|telefonia/.test(category)) return "internet";
  if (/segur|seguro|vpn|antivirus/.test(category)) return "insurance";
  if (/educacion|educativo|academia|curso/.test(category)) return "education";
  if (/tecnico|soporte|reparacion|instalacion/.test(category)) return "technical";
  return "other";
};
