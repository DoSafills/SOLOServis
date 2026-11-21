import type { Product } from "../types";
import type { CategoryIconName } from "../components/common/CategoryIcon";

export const SERVICE_CATEGORY_ICONS = {
  internet: "wifi",
  security: "shield",
  education: "book",
  technicians: "wrench",
  other: "grid",
} as const;

const PRODUCT_SUBTYPES = [
  { name: "Televisores", pattern: /televisor|television|\btv\b/ },
  { name: "Freidoras de aire", pattern: /freidora.{0,12}aire|air.?fryer/ },
  { name: "Refrigeradores", pattern: /refriger|nevera|freezer|congelador/ },
  { name: "Microondas", pattern: /microonda/ },
  { name: "Lavadoras", pattern: /lavadora|washer/ },
  { name: "Secadoras", pattern: /secadora|dryer/ },
  { name: "Lavavajillas", pattern: /lavavajilla|lavaplatos|dishwasher/ },
  { name: "Hornos y cocinas", pattern: /horno|cocina|encimera/ },
  { name: "Aspiradoras", pattern: /aspirador|aspiradora|robot.{0,8}limpieza/ },
  { name: "Cafeteras", pattern: /cafetera|coffee maker/ },
  { name: "Licuadoras", pattern: /licuadora|minipimer|batidora/ },
  { name: "Tostadores y hervidores", pattern: /tostador|hervidor|tetera electrica/ },
  { name: "Aire acondicionado", pattern: /aire acondicionado|climatizador/ },
  { name: "Ventiladores y calefactores", pattern: /ventilador|calefactor|estufa electrica/ },
  { name: "Notebooks", pattern: /notebook|laptop|macbook|computador portatil/ },
  {
    name: "Celulares",
    pattern: /celular|smartphone|iphone|telefonia|telefono movil|telefono celular/,
  },
  { name: "Relojes inteligentes", pattern: /reloj|smartwatch/ },
  { name: "Audífonos y audio", pattern: /audif|auricular|headphone|parlante|speaker|audio/ },
  { name: "Monitores", pattern: /monitor/ },
  { name: "Cámaras", pattern: /camara/ },
  {
    name: "Consolas y gaming",
    pattern: /gaming|gamer|consola|playstation|xbox|nintendo|videojuego|gamepad/,
  },
  {
    name: "Muebles",
    pattern: /mueble|mesa|silla|sofa|cama|escritorio|colchon|decoracion|alfombra|ropero|estante/,
  },
];

/**
 * Normaliza un texto para compararlo: quita tildes y lo pasa a minúsculas.
 * @param value Texto original.
 * @returns El texto sin diacríticos y en minúsculas.
 */
const normalize = (value: string): string =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase();

/**
 * Elige el ícono de una categoría de servicio a partir de su nombre.
 * @param name Nombre de la categoría de servicio.
 * @returns Nombre del ícono; "grid" si no reconoce el nombre.
 */
export const getServiceCategoryIcon = (name: string): CategoryIconName => {
  const normalized = normalize(name);
  if (/internet|telefon|telecom/.test(normalized)) return SERVICE_CATEGORY_ICONS.internet;
  if (/seguridad|seguro/.test(normalized)) return SERVICE_CATEGORY_ICONS.security;
  if (/educaci|curso|capacit/.test(normalized)) return SERVICE_CATEGORY_ICONS.education;
  if (/tecnic|repar|manten|electric|gasfiter|plomer|instal/.test(normalized)) {
    return SERVICE_CATEGORY_ICONS.technicians;
  }
  return SERVICE_CATEGORY_ICONS.other;
};

/**
 * Elige el ícono de una categoría de producto a partir de su nombre.
 * @param name Nombre de la categoría o del tipo de producto.
 * @returns Nombre del ícono; "grid" si no reconoce el nombre.
 */
export const getProductCategoryIcon = (name: string): CategoryIconName => {
  const normalized = normalize(name);
  if (/refriger|nevera|freezer|congel/.test(normalized)) return "refrigerator";
  if (/lavador|secador|lavavajilla|lavaplatos/.test(normalized)) return "washer";
  if (/televisor|television|\btv\b|monitor|proyector/.test(normalized)) return "television";
  if (/notebook|laptop|comput|impresora|tablet/.test(normalized)) return "laptop";
  if (/celular|smartphone|iphone|telefono/.test(normalized)) return "phone";
  if (/audif|auricular|headphone|parlante|speaker|audio/.test(normalized)) return "headphones";
  if (/camara|fotograf/.test(normalized)) return "camera";
  if (/reloj|smartwatch/.test(normalized)) return "watch";
  if (/cafetera|coffee/.test(normalized)) return "coffee";
  if (/aspirador/.test(normalized)) return "vacuum";
  if (/aire acondicionado|climatizador/.test(normalized)) return "airConditioning";
  if (/freidora|air.?fryer/.test(normalized)) return "airFryer";
  if (/microonda/.test(normalized)) return "microwave";
  if (/horno|cocina|tostador|hervidor|tetera/.test(normalized)) return "kitchen";
  if (/licuadora|batidora/.test(normalized)) return "blender";
  if (
    /mueble|mesa|silla|sofa|cama|escritorio|colchon|decoracion|alfombra|ropero|estante/.test(
      normalized,
    )
  ) {
    return "furniture";
  }
  if (/gaming|gamer|consola|playstation|xbox|nintendo|videojuego|gamepad/.test(normalized)) {
    return "gamepad";
  }
  if (/electrodom/.test(normalized)) return "refrigerator";
  if (/tecnolog|electron/.test(normalized)) return "bolt";
  return "grid";
};

/**
 * Infiere el tipo concreto de un producto (Televisores, Notebooks…) con los
 * patrones de PRODUCT_SUBTYPES. Se usa para nombrar las tarjetas del catálogo.
 * @param product Producto a clasificar.
 * @returns Nombre del tipo; si ninguno calza, la categoría o "Otros productos".
 */
export const getProductSubtype = (product: Product): string => {
  const searchableText = normalize(
    `${product.category} ${product.subcategory} ${product.name} ${product.model}`,
  );
  const subtype = PRODUCT_SUBTYPES.find((candidate) => candidate.pattern.test(searchableText));

  return subtype?.name ?? (product.category.trim() || "Otros productos");
};

// Claves normalizadas con `normalize`: el tipo se compara sin tildes ni
// mayúsculas, así "Camaras" y "Cámaras" encuentran la misma raíz.
const SUBTYPE_SEARCH_QUERIES: Record<string, string> = {
  televisores: "televisor",
  notebooks: "notebook",
  "consolas y gaming": "consola",
  monitores: "monitor",
  camaras: "camara",
  "relojes inteligentes": "smartwatch",
  "freidoras de aire": "freidora",
  refrigeradores: "refrigerador",
  microondas: "microondas",
  lavadoras: "lavadora",
  secadoras: "secadora",
  lavavajillas: "lavavajilla",
  "hornos y cocinas": "horno",
  aspiradoras: "aspiradora",
  cafeteras: "cafetera",
  licuadoras: "licuadora",
  "tostadores y hervidores": "hervidor",
  "aire acondicionado": "aire",
  "ventiladores y calefactores": "ventilador",
};

/**
 * Devuelve el texto de búsqueda que acota una tarjeta de tipo dentro de su
 * categoría. Es una raíz de palabra que aparece en el nombre de los productos
 * de ese tipo ("televisor" encuentra "Televisor LG…").
 *
 * La búsqueda del backend exige que cada palabra aparezca en el producto, así
 * que mandar el nombre de la tarjeta ("Consolas y gaming") devolvía 0
 * resultados (TAL-244). Los tipos sin una palabra común en sus productos
 * (Celulares, Audífonos y audio, Muebles…) no tienen entrada.
 * @param subtype Nombre del tipo, como lo devuelve getProductSubtype.
 * @returns La raíz de búsqueda, o "" si el tipo no tiene una confiable.
 */
export const getProductSubtypeSearchQuery = (subtype: string): string =>
  SUBTYPE_SEARCH_QUERIES[normalize(subtype.trim())] ?? "";
