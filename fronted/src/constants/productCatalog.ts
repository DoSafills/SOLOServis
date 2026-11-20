import type { Product } from "../types";
import type { CategoryIconName } from "../components/common/CategoryIcon";

export const PRODUCT_TYPE_ICONS = {
  appliances: "refrigerator",
  technology: "bolt",
  computing: "laptop",
  phones: "phone",
  gaming: "gamepad",
  home: "house",
} as const;

export const CATEGORY_SECTION_ICONS = {
  home: "house",
  technology: "bolt",
  other: "grid",
} as const;

export const SERVICE_CATEGORY_ICONS = {
  internet: "wifi",
  security: "shield",
  education: "book",
  technicians: "wrench",
  other: "grid",
} as const;

export const PRODUCT_TYPES = [
  {
    id: "appliances",
    name: "Electrodomésticos",
    icon: PRODUCT_TYPE_ICONS.appliances,
    color: "#FB923C",
    pattern:
      /electrodomest|refriger|nevera|freezer|congelador|microonda|lavador|secador|lavavajilla|horno|cocina|aspirador|cafetera|licuadora|freidora|batidora|tostador|hervidor|plancha|ventilador|calefactor|aire acondicionado/,
  },
  {
    id: "technology",
    name: "Tecnología",
    icon: PRODUCT_TYPE_ICONS.technology,
    color: "#E8001B",
    pattern:
      /televisor|television|\btv\b|reloj|smartwatch|audif|auricular|headphone|parlante|speaker|camara|proyector|audio|microfono/,
  },
  {
    id: "computing",
    name: "Computación",
    icon: PRODUCT_TYPE_ICONS.computing,
    color: "#818CF8",
    pattern: /notebook|laptop|computador|computadora|desktop|impresora|monitor|tablet/,
  },
  {
    id: "phones",
    name: "Celulares",
    icon: PRODUCT_TYPE_ICONS.phones,
    color: "#F472B6",
    pattern: /celular|smartphone|iphone|telefono movil|telefono celular/,
  },
  {
    id: "gaming",
    name: "Gaming",
    icon: PRODUCT_TYPE_ICONS.gaming,
    color: "#A78BFA",
    pattern: /gaming|gamer|consola|playstation|xbox|nintendo|videojuego|gamepad/,
  },
  {
    id: "home",
    name: "Hogar y muebles",
    icon: PRODUCT_TYPE_ICONS.home,
    color: "#34D399",
    pattern:
      /electrodomest|refriger|nevera|freezer|congelador|microonda|lavador|secador|lavavajilla|horno|cocina|aspirador|cafetera|licuadora|freidora|batidora|tostador|hervidor|plancha|ventilador|calefactor|aire acondicionado|mueble|mesa|silla|sofa|cama|escritorio|colchon|decoracion|alfombra|ropero|estante/,
  },
] as const;

export type ProductTypeId = (typeof PRODUCT_TYPES)[number]["id"];

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

export type ProductCategoryGroup = "home" | "technology" | "other";

/**
 * Ubica una categoría raíz en una de las secciones de la página de categorías.
 * @param name Nombre de la categoría.
 * @returns "home", "technology" u "other".
 */
export const getProductCategoryGroup = (name: string): ProductCategoryGroup => {
  const normalized = normalize(name);
  if (
    /hogar|mueble|mesa|silla|sofa|cama|electrodom|refriger|lavado|lavadora|secadora|cocina|microonda|horno|aspirador|freidora|cafetera|licuadora|batidora|lavavajilla|tostador|hervidor|ventilador|calefactor/.test(
      normalized,
    )
  ) {
    return "home";
  }
  if (
    /tecnolog|electron|televisor|television|\btv\b|videojuego|computacion|computador|notebook|telefon|audio|gaming|consola|celular|smartphone/.test(
      normalized,
    )
  ) {
    return "technology";
  }
  return "other";
};

/**
 * Indica si un producto pertenece a un tipo amplio (Electrodomésticos,
 * Tecnología…) según su categoría, nombre y modelo.
 * @param product Producto a revisar.
 * @param typeId Id del tipo en PRODUCT_TYPES; si no existe, acepta todo.
 * @returns true si el producto calza con el tipo.
 */
export const productMatchesType = (product: Product, typeId: string): boolean => {
  const type = PRODUCT_TYPES.find((candidate) => candidate.id === typeId);
  if (!type) return true;

  return type.pattern.test(
    normalize(`${product.category} ${product.subcategory} ${product.name} ${product.model}`),
  );
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

/**
 * Indica si un producto es del tipo indicado.
 * @param product Producto a revisar.
 * @param subtype Nombre del tipo, como lo devuelve getProductSubtype.
 * @returns true si getProductSubtype(product) es ese tipo.
 */
export const productMatchesSubtype = (product: Product, subtype: string): boolean =>
  getProductSubtype(product) === subtype;

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
export const getProductSubtypeSearchQuery = (subtype: string): string => {
  const queries: Record<string, string> = {
    Televisores: "televisor",
    Notebooks: "notebook",
    "Consolas y gaming": "consola",
    Monitores: "monitor",
    Cámaras: "camara",
    "Relojes inteligentes": "smartwatch",
    "Freidoras de aire": "freidora",
    Refrigeradores: "refrigerador",
    Microondas: "microondas",
    Lavadoras: "lavadora",
    Secadoras: "secadora",
    Lavavajillas: "lavavajilla",
    "Hornos y cocinas": "horno",
    Aspiradoras: "aspiradora",
    Cafeteras: "cafetera",
    Licuadoras: "licuadora",
    "Tostadores y hervidores": "hervidor",
    "Aire acondicionado": "aire",
    "Ventiladores y calefactores": "ventilador",
  };
  return queries[subtype] ?? "";
};

/**
 * Indica si un producto aparecería al buscar el texto en el backend: cada
 * palabra debe estar en el nombre, el modelo o la marca, sin distinguir
 * tildes ni mayúsculas (igual que la Search API; el SKU no llega al frontend,
 * así que esta comprobación es igual o más estricta).
 * @param product Producto a revisar.
 * @param query Texto de búsqueda; vacío calza con todo.
 * @returns true si todas las palabras aparecen en el producto.
 */
export const productMatchesSearchQuery = (product: Product, query: string): boolean => {
  const searchable = normalize(`${product.name} ${product.model} ${product.brand}`);
  return normalize(query)
    .split(/\s+/)
    .filter(Boolean)
    .every((word) => searchable.includes(word));
};

/**
 * Elige el texto de búsqueda de una tarjeta de tipo: la raíz de
 * getProductSubtypeSearchQuery solo si todos los productos de la tarjeta la
 * contienen. Si alguno no la tiene, devuelve "" y la tarjeta filtra solo por
 * categoría, para que nunca muestre 0 resultados teniendo productos (TAL-244).
 * @param subtype Nombre del tipo de la tarjeta.
 * @param products Productos que cuenta la tarjeta.
 * @returns El texto de búsqueda o "".
 */
export const getSubtypeQueryForProducts = (subtype: string, products: Product[]): string => {
  const query = getProductSubtypeSearchQuery(subtype);
  if (!query || products.length === 0) return "";
  return products.every((product) => productMatchesSearchQuery(product, query)) ? query : "";
};
