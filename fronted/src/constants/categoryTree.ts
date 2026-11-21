import type { CategoryIconName } from "../components/common/CategoryIcon";

/** Hoja del árbol (NV3): categoría con la que se filtran los productos. */
export interface ProductCategoryLeaf {
  slug: string;
  name: string;
  icon: CategoryIconName;
}

/** Grupo intermedio (NV2): agrupa hojas y no filtra productos directamente. */
export interface ProductCategoryGroup extends ProductCategoryLeaf {
  children: ProductCategoryLeaf[];
}

/** Sección raíz (NV1): Hogar o Tecnología. */
export interface ProductCategorySection extends ProductCategoryLeaf {
  children: ProductCategoryGroup[];
}

/**
 * Espejo del árbol de categorías de producto del backend (migración 033):
 * exactamente tres niveles NV1 > NV2 > NV3. Se declara por slug y nombre,
 * nunca por id, para que no dependa de los ids de cada entorno. Si el
 * backend agrega o renombra una categoría, este archivo debe cambiar con él.
 */
export const PRODUCT_CATEGORY_TREE: ProductCategorySection[] = [
  {
    slug: "hogar",
    name: "Hogar",
    icon: "house",
    children: [
      {
        slug: "electrodomesticos",
        name: "Electrodomésticos",
        icon: "refrigerator",
        children: [
          { slug: "refrigeradores", name: "Refrigeradores", icon: "refrigerator" },
          { slug: "lavadoras", name: "Lavadoras", icon: "washer" },
          { slug: "cocina", name: "Cocina", icon: "kitchen" },
          { slug: "climatizacion", name: "Climatización", icon: "airConditioning" },
          { slug: "limpieza-del-hogar", name: "Limpieza del hogar", icon: "vacuum" },
          { slug: "cuidado-personal", name: "Cuidado personal", icon: "grid" },
        ],
      },
      {
        slug: "muebles",
        name: "Muebles",
        icon: "furniture",
        children: [
          { slug: "muebles-y-decoracion", name: "Muebles y decoración", icon: "furniture" },
        ],
      },
    ],
  },
  {
    slug: "tecnologia",
    name: "Tecnología",
    icon: "bolt",
    children: [
      {
        slug: "computacion",
        name: "Computación",
        icon: "laptop",
        children: [
          { slug: "notebooks", name: "Notebooks", icon: "laptop" },
          {
            slug: "computadores-de-escritorio",
            name: "Computadores de escritorio",
            icon: "laptop",
          },
          {
            slug: "accesorios-de-computacion",
            name: "Accesorios de computación",
            icon: "laptop",
          },
        ],
      },
      {
        slug: "pantallas",
        name: "Pantallas",
        icon: "television",
        children: [{ slug: "televisores", name: "Televisores", icon: "television" }],
      },
      {
        slug: "sonido",
        name: "Sonido",
        icon: "headphones",
        children: [{ slug: "audio", name: "Audio", icon: "headphones" }],
      },
      {
        slug: "telefonia",
        name: "Telefonía",
        icon: "phone",
        children: [{ slug: "celulares", name: "Celulares", icon: "phone" }],
      },
      {
        slug: "videojuegos-y-consolas",
        name: "Videojuegos y consolas",
        icon: "gamepad",
        children: [{ slug: "gaming", name: "Gaming", icon: "gamepad" }],
      },
    ],
  },
];

/**
 * Todos los grupos (NV2) del árbol, en el orden declarado.
 * @returns Una lista plana de los grupos intermedios.
 */
export const getProductCategoryGroups = (): ProductCategoryGroup[] =>
  PRODUCT_CATEGORY_TREE.flatMap((section) => section.children);

/**
 * Busca una categoría por slug en cualquier nivel.
 * @param slug Slug a buscar.
 * @returns La ruta NV1 > NV2 > NV3 que termina en la categoría, o null.
 */
export const findProductCategoryPath = (slug: string): ProductCategoryLeaf[] | null => {
  for (const section of PRODUCT_CATEGORY_TREE) {
    if (section.slug === slug) return [section];
    for (const group of section.children) {
      if (group.slug === slug) return [section, group];
      for (const leaf of group.children) {
        if (leaf.slug === slug) return [section, group, leaf];
      }
    }
  }
  return null;
};

/**
 * Indica si un texto es el slug de una categoría del árbol.
 * @param value Texto a revisar.
 * @returns true si corresponde a un slug NV1, NV2 o NV3.
 */
export const isProductCategorySlug = (value: string): boolean =>
  findProductCategoryPath(value) !== null;

/**
 * Devuelve el nombre visible de una categoría del árbol.
 * @param slug Slug de la categoría.
 * @returns El nombre, o null si el slug no pertenece al árbol.
 */
export const getProductCategorySlugName = (slug: string): string | null =>
  findProductCategoryPath(slug)?.at(-1)?.name ?? null;

/**
 * Hojas NV3 que cuelgan de una categoría, incluida ella misma si ya es hoja.
 * @param slug Slug de NV1, NV2 o NV3.
 * @returns Los slugs de las hojas bajo esa categoría (vacío si no existe).
 */
export const getProductCategoryLeafSlugs = (slug: string): string[] => {
  const section = PRODUCT_CATEGORY_TREE.find((item) => item.slug === slug);
  if (section) return section.children.flatMap((group) => group.children.map((leaf) => leaf.slug));
  const group = getProductCategoryGroups().find((item) => item.slug === slug);
  if (group) return group.children.map((leaf) => leaf.slug);
  return isProductCategorySlug(slug) ? [slug] : [];
};
