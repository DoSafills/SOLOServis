const compatibleServiceAliases: string[][] = [
  ["Educación", "Educación Online"],
  ["Internet y Telefonía", "Internet Fibra Óptica"],

  ["Servidores VPS", "Servidores en la nube"],
  ["Almacenamiento en la nube", "Almacenamiento de por vida"],

  ["Seguridad Digital", "Seguridad y contraseñas"],
];

const STOP_WORDS = new Set([
  "de",
  "del",
  "la",
  "el",
  "los",
  "las",
  "y",
  "e",
  "en",
  "para",
  "a",
  "servicio",
  "servicios",
]);

const tokenize = (text: string): string[] =>
  text
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .replace(/[^a-z0-9\s]/g, " ")
    .split(/\s+/)
    .filter((word) => word && !STOP_WORDS.has(word));

const wordForms = (word: string): string[] => {
  const forms = new Set([word]);
  if (word.length > 4 && word.endsWith("es")) forms.add(word.slice(0, -2));
  if (word.length > 3 && word.endsWith("s")) forms.add(word.slice(0, -1));
  return [...forms];
};

const phraseVariants = (text: string): Set<string> => {
  const tokens = tokenize(text);
  if (tokens.length === 0) return new Set();

  const variants = tokens
    .map(wordForms)
    .reduce<string[]>(
      (acc, forms) =>
        acc.flatMap((prefix) => forms.map((form) => (prefix ? `${prefix} ${form}` : form))),
      [""],
    );

  return new Set(variants);
};

const aliasIndex = new Map<string, Set<number>>();
compatibleServiceAliases.forEach((aliases, groupId) => {
  aliases.forEach((alias) => {
    phraseVariants(alias).forEach((variant) => {
      const groups = aliasIndex.get(variant) ?? new Set<number>();
      groups.add(groupId);
      aliasIndex.set(variant, groups);
    });
  });
});

const aliasGroupsOf = (variants: Set<string>): Set<number> => {
  const groups = new Set<number>();
  variants.forEach((variant) => aliasIndex.get(variant)?.forEach((id) => groups.add(id)));
  return groups;
};

export const areServiceCategoriesCompatible = (first: string, second: string): boolean => {
  const firstVariants = phraseVariants(first);
  const secondVariants = phraseVariants(second);

  if (firstVariants.size === 0 || secondVariants.size === 0) return false;

  for (const variant of firstVariants) {
    if (secondVariants.has(variant)) return true;
  }

  const firstGroups = aliasGroupsOf(firstVariants);
  for (const group of aliasGroupsOf(secondVariants)) {
    if (firstGroups.has(group)) return true;
  }

  return false;
};
