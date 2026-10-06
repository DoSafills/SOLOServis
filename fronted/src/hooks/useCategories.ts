import { getCategories } from "../services/api/products";
import { useFetch } from "./useFetch";

/** Árbol de categorías de productos: las raíz tienen parentId null. */
export function useCategories() {
  const { data: categories = [] } = useFetch("categories", getCategories);

  return {
    categories,
    rootCategories: categories.filter((category) => category.parentId === null),
    subcategoriesOf: (parentId: number) =>
      categories.filter((category) => category.parentId === parentId),
  };
}
