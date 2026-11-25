const compatibleCategoryAliases = [
  ["notebook", "laptop", "computador portatil", "computadora portatil"],
  ["refrigerador", "nevera"],
];

const unknownFeatureValue = /^(?:no informado|no disponible|desconocido|null|n\/a|na|-)?$/;
const affirmativeFeatureValue = /^(?:si|true|1|con|incluido|incluye|tiene|dispone)$/;
const negativeFeatureValue =
  /^(?:no|false|0|sin|no incluye|no tiene|no dispone|no aplica|ciclico|manual|clasico|convencional)$/;
const absentFeaturePhrase =
  /\b(?:sin|no)\s+(?:wifi|wi\s*fi|bluetooth|inverter|no\s*frost|pantalla\s*tactil|autolimpieza|dispensador|fabrica\s+de\s+hielo)\b/;
const presentFeaturePhrase =
  /no\s*frost|inverter|wi\s*fi|wifi|bluetooth|smart|inteligente|asistente\s+de\s+voz|pantalla\s+tactil|autolimpieza|secado|dispensador|fabrica\s+de\s+hielo|silencioso/;
const beneficialFeatureSpec =
  /no\s*frost|inverter|deshielo|descongelacion|dispensador|fabrica\s+de\s+hielo|wifi|wi\s*fi|bluetooth|conectividad|smart|inteligente|asistente\s+de\s+voz|pantalla\s+tactil|autolimpieza|secado|silencioso|ruido/;

export type BeneficialFeaturePresence = "present" | "absent" | "unknown";

/**
 * Normaliza texto para comparar nombres y valores sin distinguir tildes o mayúsculas.
 *
 * @param value Texto que se quiere normalizar.
 * @returns El texto en minúsculas, sin tildes ni espacios sobrantes.
 */
const normalizeText = (value: string): string =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim();

/**
 * Normaliza una categoría y elimina plurales simples para aceptar alias equivalentes.
 *
 * @param category Nombre de la categoría entregado por el producto.
 * @returns La categoría normalizada y sin plurales simples.
 */
const normalizeCategory = (category: string): string =>
  normalizeText(category)
    .split(/\s+/)
    .map((word) => {
      if (word.length > 4 && word.endsWith("es")) return word.slice(0, -2);
      if (word.length > 3 && word.endsWith("s")) return word.slice(0, -1);
      return word;
    })
    .join(" ");

/**
 * Comprueba si dos categorías son iguales o pertenecen a un alias compatible.
 *
 * @param first Primera categoría que se quiere comparar.
 * @param second Segunda categoría que se quiere comparar.
 * @returns `true` cuando las categorías pueden compararse; en otro caso, `false`.
 */
export const areProductCategoriesCompatible = (first: string, second: string): boolean => {
  const firstCategory = normalizeCategory(first);
  const secondCategory = normalizeCategory(second);

  if (!firstCategory || !secondCategory) return false;
  if (firstCategory === secondCategory) return true;

  return compatibleCategoryAliases.some(
    (aliases) => aliases.includes(firstCategory) && aliases.includes(secondCategory),
  );
};

/**
 * Determina si una característica puede tratarse como un beneficio explícito.
 *
 * @param specName Nombre de la característica.
 * @returns `true` si el nombre corresponde a una función beneficiosa reconocida.
 */
const isBeneficialFeatureSpec = (specName: string): boolean =>
  beneficialFeatureSpec.test(normalizeText(specName));

/**
 * Interpreta si una función beneficiosa está presente, ausente o no informada.
 *
 * @param specName Nombre de la característica.
 * @param value Valor textual entregado por la tienda.
 * @returns El estado de presencia; `unknown` evita inventar una ausencia.
 */
const getBeneficialFeaturePresence = (
  specName: string,
  value: string | undefined,
): BeneficialFeaturePresence => {
  const normalizedValue = normalizeText(value ?? "");
  if (!isBeneficialFeatureSpec(specName) || unknownFeatureValue.test(normalizedValue)) {
    return "unknown";
  }
  if (affirmativeFeatureValue.test(normalizedValue)) return "present";
  if (negativeFeatureValue.test(normalizedValue) || absentFeaturePhrase.test(normalizedValue)) {
    return "absent";
  }
  if (presentFeaturePhrase.test(normalizedValue)) return "present";
  return "unknown";
};

/**
 * Compara una función beneficiosa solo cuando todos los datos son interpretables.
 *
 * @param specName Nombre de la característica.
 * @param values Valores de la característica para cada producto.
 * @returns Un indicador de presencia por producto, o `null` si falta información.
 */
export const compareBeneficialFeature = (
  specName: string,
  values: ReadonlyArray<string | undefined>,
): boolean[] | null => {
  if (!isBeneficialFeatureSpec(specName)) return null;

  const presences = values.map((value) => getBeneficialFeaturePresence(specName, value));
  if (!presences.includes("present") || presences.some((presence) => presence === "unknown")) {
    return null;
  }

  return presences.map((presence) => presence === "present");
};

/**
 * Genera una etiqueta breve para una función beneficiosa incluida.
 *
 * @param specName Nombre de la característica.
 * @returns Una etiqueta como «Incluye No Frost» o «Incluye Wi-Fi».
 */
export const getBeneficialFeatureLabel = (specName: string): string => {
  const normalized = normalizeText(specName);
  if (normalized.includes("no frost")) return "Incluye No Frost";
  if (normalized.includes("inverter")) return "Incluye Inverter";
  if (normalized.includes("wifi") || normalized.includes("wi fi")) return "Incluye Wi-Fi";
  if (normalized.includes("bluetooth")) return "Incluye Bluetooth";
  if (normalized.includes("dispensador")) return "Incluye dispensador";
  if (normalized.includes("fabrica de hielo")) return "Incluye fábrica de hielo";
  return `Incluye ${specName}`;
};

/**
 * Devuelve una etiqueta específica cuando el mejor valor tiene un significado claro.
 *
 * @param specName Nombre de la característica comparada.
 * @returns La etiqueta de beneficio correspondiente o «Mejor» como valor general.
 */
export const getBestSpecLabel = (specName: string): string => {
  const normalized = normalizeText(specName);
  if (/(eficiencia|energy.*efficiency|clase.*energetic)/.test(normalized)) {
    return "Mejor eficiencia";
  }
  if (/(capacidad|volumen)/.test(normalized)) return "Mayor capacidad";
  if (/(consumo.*(energia|energetic|electric|kwh)|kwh)/.test(normalized)) {
    return "Menor consumo";
  }
  return "Mejor";
};
