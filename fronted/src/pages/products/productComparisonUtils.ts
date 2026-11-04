const compatibleCategoryAliases = [
  ["notebook", "laptop", "computador portatil", "computadora portatil"],
  ["refrigerador", "nevera"],
];

const normalizeCategory = (category: string): string =>
  category
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim()
    .split(/\s+/)
    .map((word) => {
      if (word.length > 4 && word.endsWith("es")) return word.slice(0, -2);
      if (word.length > 3 && word.endsWith("s")) return word.slice(0, -1);
      return word;
    })
    .join(" ");

export const areProductCategoriesCompatible = (first: string, second: string): boolean => {
  const firstCategory = normalizeCategory(first);
  const secondCategory = normalizeCategory(second);

  if (!firstCategory || !secondCategory) return false;
  if (firstCategory === secondCategory) return true;

  return compatibleCategoryAliases.some(
    (aliases) => aliases.includes(firstCategory) && aliases.includes(secondCategory),
  );
};
