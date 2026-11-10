import type { Product } from "../types";

export const PRODUCT_TYPES = [
  {
    id: "appliances",
    name: "Electrodomésticos",
    icon: "🏠",
    color: "#FB923C",
    pattern:
      /electrodomest|refriger|nevera|freezer|congelador|microonda|lavador|secador|lavavajilla|horno|cocina|aspirador|cafetera|licuadora|freidora|aire acondicionado/,
  },
  {
    id: "technology",
    name: "Tecnología",
    icon: "⚡",
    color: "#E8001B",
    pattern:
      /televisor|television|\btv\b|reloj|smartwatch|audif|auricular|headphone|parlante|speaker|camara|proyector|audio|microfono/,
  },
  {
    id: "computing",
    name: "Computación",
    icon: "💻",
    color: "#818CF8",
    pattern: /notebook|laptop|computador|computadora|desktop|impresora|monitor|tablet/,
  },
  {
    id: "phones",
    name: "Celulares",
    icon: "📱",
    color: "#F472B6",
    pattern: /celular|smartphone|iphone|telefono movil|telefono celular/,
  },
  {
    id: "gaming",
    name: "Gaming",
    icon: "🎮",
    color: "#A78BFA",
    pattern: /gaming|gamer|consola|playstation|xbox|nintendo|videojuego|gamepad/,
  },
  {
    id: "home",
    name: "Hogar y muebles",
    icon: "🛋️",
    color: "#34D399",
    pattern: /mueble|mesa|silla|sofa|cama|escritorio|colchon|decoracion|alfombra|ropero|estante/,
  },
] as const;

export type ProductTypeId = (typeof PRODUCT_TYPES)[number]["id"];

const PRODUCT_SUBTYPES = [
  { name: "Televisores", pattern: /televisor|television|\btv\b/ },
  { name: "Refrigeradores", pattern: /refriger|nevera|freezer|congelador/ },
  { name: "Microondas", pattern: /microonda/ },
  { name: "Lavadoras y secadoras", pattern: /lavador|secador/ },
  { name: "Notebooks", pattern: /notebook|laptop/ },
  { name: "Celulares", pattern: /celular|smartphone|iphone|telefono movil|telefono celular/ },
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

const normalize = (value: string): string =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase();

export const productMatchesType = (product: Product, typeId: string): boolean => {
  const type = PRODUCT_TYPES.find((candidate) => candidate.id === typeId);
  if (!type) return true;

  return type.pattern.test(normalize(`${product.category} ${product.name}`));
};

export const getProductSubtype = (product: Product): string => {
  const searchableText = normalize(`${product.category} ${product.name}`);
  const subtype = PRODUCT_SUBTYPES.find((candidate) => candidate.pattern.test(searchableText));

  return subtype?.name ?? (product.category.trim() || "Otros productos");
};

export const productMatchesSubtype = (product: Product, subtype: string): boolean =>
  getProductSubtype(product) === subtype;
