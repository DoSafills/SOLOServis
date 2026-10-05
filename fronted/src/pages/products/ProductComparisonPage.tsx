import { Fragment, useEffect, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import { getProductById } from "../../services/api/api";
import { formatPrice, getMinOffer } from "../../Services/utils/productUtils";
import { Breadcrumb } from "../../components/common/ui";
import { areProductCategoriesCompatible } from "./productComparisonUtils";

interface Props {
  productIds: string[];
  navigate: (page: Page) => void;
  onAddToCart: (product: Product, offer: StoreOffer) => void;
}

interface ComparedOffer {
  offer: StoreOffer;
}

type SpecCellState = "best" | "equal" | "default";

const getAvailableOffers = (product: Product): ComparedOffer[] =>
  product.offers
    .filter((offer) => offer.available)
    .sort((first, second) => first.price - second.price)
    .map((offer) => ({ offer }));

const specificationGroups = [
  { title: "Capacidad y carga", pattern: /(capacidad|volumen|carga|soporte)/i },
  {
    title: "Consumo y energía",
    pattern:
      /(consumo|energ[ií]a|eficiencia|potencia|tensi[oó]n|voltaje|alimentaci[oó]n|watt|kwh)/i,
  },
  {
    title: "Peso y dimensiones",
    pattern: /(peso|masa|dimensi[oó]n|ancho|alto|largo|profundidad|di[aá]metro|superficie)/i,
  },
  {
    title: "Pantalla e imagen",
    pattern: /(pantalla|pulgad|resoluci[oó]n|frecuencia|panel|brillo|imagen|vram)/i,
  },
  {
    title: "Rendimiento",
    pattern:
      /(velocidad|programa|rpm|temperatura|rendimiento|presi[oó]n|procesador|n[uú]cleos|cuda|bus de memoria)/i,
  },
  {
    title: "Funciones y conectividad",
    pattern:
      /(^ia$|inteligencia|modelo.*ia|smart|wifi|bluetooth|conect|sistema operativo|asistente|voz|aplicaci[oó]n)/i,
  },
  { title: "Materiales y acabado", pattern: /(material|acabado|color)/i },
  { title: "Otros", pattern: /.*/ },
];

const getSpecificationGroups = (products: Product[]) => {
  const names = [...new Set(products.flatMap((product) => Object.keys(product.specs)))].sort(
    (a, b) => a.localeCompare(b, "es"),
  );

  const assigned = new Set<string>();

  return specificationGroups
    .map(({ title, pattern }) => ({
      title,
      names: names.filter((name) => {
        if (assigned.has(name) || !pattern.test(name)) return false;
        assigned.add(name);
        return true;
      }),
    }))
    .filter((group) => group.names.length > 0);
};

const normalizeComparableText = (value: string | undefined): string =>
  (value ?? "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim();

const parseComparableNumber = (value: string | undefined): number | null => {
  const numericToken = normalizeComparableText(value).match(/[+-]?\d+(?:[.,]\d+)?/);
  if (!numericToken) return null;

  const parsed = Number(numericToken[0].replace(",", "."));
  return Number.isFinite(parsed) ? parsed : null;
};

const getComparableUnit = (value: string): string =>
  normalizeComparableText(value)
    .replace(/[+-]?\d+(?:[.,]\d+)?/g, "#")
    .replace(/\b(maxim[oa]s?|minim[oa]s?|soportad[oa]s?|hasta|aprox(?:imad[oa]mente)?)\b/g, "")
    .replace(/[^a-z#]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();

const getEnergyEfficiencyRank = (value: string | undefined): number | null => {
  const normalized = normalizeComparableText(value);
  const grade = normalized.match(/(?:^|[^a-z])(?:clase\s+)?([a-g])(\+{1,3})?(?=$|[^a-z])/);
  if (!grade) return null;

  const baseRank = 26 - (grade[1].charCodeAt(0) - "a".charCodeAt(0));
  return baseRank + (grade[2]?.length ?? 0);
};

const isEnergyEfficiencySpec = (specName: string): boolean =>
  /(eficiencia|energy.*(?:efficiency|class|rating)|(?:clase|class|etiqueta).*energetic)/i.test(
    normalizeComparableText(specName),
  );

const isEnergyConsumptionSpec = (specName: string): boolean => {
  const name = normalizeComparableText(specName);
  return /(consumo.*(energia|energetic|electric|kwh|kilowatt)|(?:energy|electricity|electrical).*(consumption|usage|use)|kwh)/i.test(
    name,
  );
};

const isLowerBetterSpec = (specName: string): boolean => {
  const name = normalizeComparableText(specName);
  const higherIsBetter =
    /(maxim|soport|carga|capacidad|memoria|vram|ram|almacenamiento|rendimiento|velocidad|nucleos|cuda|frecuencia|resolucion|brillo)/i;
  if (higherIsBetter.test(name)) return false;

  return /(consumo|potencia|voltaje|tension|peso|masa|ancho|alto|largo|profundidad|temperatura|watt|kwh|latencia|ruido)/i.test(
    name,
  );
};

const getSpecCellStates = (
  specName: string,
  values: Array<string | undefined>,
): SpecCellState[] => {
  const entries = values.map((value) => ({
    value: value ?? "",
    normalized: normalizeComparableText(value),
    numeric: parseComparableNumber(value),
    unit: getComparableUnit(value ?? ""),
  }));

  const populated = entries.filter((entry) => entry.normalized.length > 0);

  if (populated.length === 0) {
    return values.map(() => "default");
  }

  if (isEnergyEfficiencySpec(specName)) {
    const energyRanks = entries.map((entry) =>
      entry.normalized ? getEnergyEfficiencyRank(entry.value) : null,
    );
    const populatedRanks = energyRanks.filter((rank) => rank !== null);
    if (populatedRanks.length === populated.length) {
      const bestRank = Math.max(...populatedRanks);
      return energyRanks.map((rank) => (rank === bestRank ? "best" : "default"));
    }
  }

  const textValues = populated.map((entry) => entry.normalized);
  const allSimilarText = new Set(textValues).size <= 1;
  if (allSimilarText) {
    return values.map((value) =>
      value && normalizeComparableText(value).length > 0 ? "equal" : "default",
    );
  }

  const numericValues = populated.map((entry) => entry.numeric);
  const units = new Set(populated.map((entry) => entry.unit));
  if (numericValues.every((num) => num !== null) && units.size === 1) {
    const comparableNumbers = numericValues as number[];
    const bestValue = isLowerBetterSpec(specName)
      ? Math.min(...comparableNumbers)
      : Math.max(...comparableNumbers);
    const range = Math.max(...comparableNumbers) - Math.min(...comparableNumbers);
    const isSimilar = range / Math.max(Math.abs(bestValue), 1) <= 0.05;

    if (isSimilar && !isEnergyConsumptionSpec(specName)) {
      return values.map((_, index) => (entries[index].normalized ? "equal" : "default"));
    }

    return values.map((_, index) => {
      const entry = entries[index];
      return entry.numeric === bestValue ? "best" : "default";
    });
  }

  return values.map(() => "default");
};

const getCellTone = (state: SpecCellState) => {
  switch (state) {
    case "best":
      return "border-cyan-200 bg-gradient-to-br from-cyan-300/25 via-sky-500/15 to-violet-500/20 text-white shadow-[0_0_24px_rgba(34,211,238,0.24),inset_0_0_0_1px_rgba(165,243,252,0.2)] ring-1 ring-cyan-200/30";
    case "equal":
      return "border-slate-500/70 bg-slate-700/35 text-slate-100";
    default:
      return "border-slate-700/70 bg-slate-950/40 text-slate-100";
  }
};

export default function ProductComparisonPage({ productIds, navigate, onAddToCart }: Props) {
  const [selected, setSelected] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadProducts() {
      if (productIds.length === 0) {
        setSelected([]);
        setLoading(false);
        return;
      }

      setLoading(true);
      setError(null);
      try {
        const results = await Promise.all(productIds.map((id) => getProductById(id)));
        if (cancelled) return;
        const valid = results.filter((p: Product | null): p is Product => p !== null);
        setSelected(valid);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Error al cargar los productos");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadProducts();

    return () => {
      cancelled = true;
    };
  }, [productIds]);

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="text-slate-300">Cargando productos...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="mb-4 text-rose-300">{error}</p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "linear-gradient(135deg, #ff9878 0%, #fb7185 100%)", color: "#fff" }}
          className="rounded-xl px-5 py-2 text-sm font-semibold shadow-md shadow-rose-500/15 transition hover:brightness-105"
        >
          Buscar productos
        </button>
      </div>
    );
  }

  if (selected.length < 2) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="mb-4 text-slate-300">Selecciona al menos 2 productos para comparar.</p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "linear-gradient(135deg, #ff9878 0%, #fb7185 100%)", color: "#fff" }}
          className="rounded-xl px-5 py-2 text-sm font-semibold shadow-md shadow-rose-500/15 transition hover:brightness-105"
        >
          Buscar productos
        </button>
      </div>
    );
  }

  const baseCategory = selected[0].category;
  const incompatibleProduct = selected.find(
    (product) => !areProductCategoriesCompatible(baseCategory, product.category),
  );

  if (incompatibleProduct) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p role="alert" className="mb-4 text-amber-300">
          No se pueden comparar productos de categorías diferentes: {baseCategory} y{" "}
          {incompatibleProduct.category}.
        </p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "linear-gradient(135deg, #ff9878 0%, #fb7185 100%)", color: "#fff" }}
          className="rounded-xl px-5 py-2 text-sm font-semibold shadow-md shadow-rose-500/15 transition hover:brightness-105"
        >
          Buscar productos compatibles
        </button>
      </div>
    );
  }

  const productOffers = selected.map((product) => getAvailableOffers(product));
  const cartOffers = selected.map(
    (product, index) => productOffers[index][0]?.offer ?? getMinOffer(product),
  );
  const specificationRows = getSpecificationGroups(selected);
  const bestRating = Math.max(...selected.map((product) => product.rating));
  const comparableProductPrices = productOffers.flatMap((result) =>
    result.map(({ offer }) => offer.price),
  );
  const lowestProductPrice =
    comparableProductPrices.length > 0 ? Math.min(...comparableProductPrices) : null;

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Productos", onClick: () => navigate({ id: "search-products", query: "" }) },
          { label: "Comparación" },
        ]}
      />

      <h1 className="mb-2 text-2xl font-bold text-slate-100">Comparación de productos</h1>
      <p className="mb-8 text-sm text-slate-300">Comparando {selected.length} productos</p>

      <div className="rounded-[28px] border border-slate-600/80 bg-gradient-to-br from-slate-900/95 via-slate-950/90 to-indigo-950/45 p-3 shadow-[0_24px_70px_rgba(2,6,23,0.55)] backdrop-blur-sm sm:p-5">
        <p className="mb-4 flex flex-wrap items-center gap-2 rounded-xl border border-slate-700/70 bg-slate-950/45 px-3 py-2 text-[11px] text-slate-200">
          <span className="inline-flex items-center gap-1 rounded-full border border-cyan-200/50 bg-cyan-300 px-2 py-1 font-extrabold text-slate-950">
            ✦ Mejor
          </span>
          El brillo cian identifica el mejor valor comparable en cada característica.
        </p>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] border-separate border-spacing-1.5">
            <thead>
              <tr>
                <th className="w-44 rounded-xl border border-slate-700 bg-slate-800/90 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-200">
                  Característica
                </th>
                {selected.map((product) => (
                  <th
                    key={product.id}
                    className="rounded-xl border border-slate-700 bg-gradient-to-br from-slate-800/95 to-slate-900/95 p-4 text-center"
                  >
                    <div className="flex flex-col items-center gap-3">
                      {product.image ? (
                        <img
                          src={product.image}
                          alt={product.name}
                          className="h-16 w-20 rounded-2xl object-contain"
                        />
                      ) : (
                        <div className="flex h-16 w-20 items-center justify-center rounded-2xl bg-slate-700/80 text-[10px] text-slate-300">
                          Sin imagen
                        </div>
                      )}

                      <div>
                        <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-cyan-300">
                          {product.brand}
                        </div>
                        <div className="mt-1 max-w-[180px] text-sm font-semibold leading-tight text-slate-100">
                          {product.name}
                        </div>
                      </div>

                      <button
                        onClick={() => navigate({ id: "product-detail", productId: product.id })}
                        className="rounded-lg border border-slate-600 bg-slate-700/80 px-3 py-1 text-[11px] font-medium text-slate-200 transition-colors hover:border-cyan-400 hover:text-cyan-200"
                      >
                        Ver detalle
                      </button>

                      <button
                        type="button"
                        disabled={!cartOffers[selected.indexOf(product)]}
                        onClick={() => {
                          const offer = cartOffers[selected.indexOf(product)];
                          if (offer) onAddToCart(product, offer);
                        }}
                        style={{
                          background: "linear-gradient(135deg, #ff9878 0%, #fb7185 100%)",
                          color: "#fff",
                        }}
                        className="rounded-lg px-3 py-1.5 text-[11px] font-semibold shadow-md shadow-rose-500/15 transition hover:brightness-105 disabled:cursor-not-allowed disabled:opacity-40"
                      >
                        Agregar al carrito
                      </button>
                    </div>
                  </th>
                ))}
              </tr>
            </thead>

            <tbody>
              <tr>
                <th scope="row" className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200">
                  Precio del producto
                </th>
                {productOffers.map((result, index) => (
                  <td
                    key={selected[index].id}
                    className={`rounded-xl border p-2 text-center ${lowestProductPrice !== null && result.some(({ offer }) => offer.price === lowestProductPrice) ? getCellTone("best") : getCellTone("default")}`}
                  >
                    {result.length > 0 ? (
                      result.map(({ offer }) => (
                        <div key={offer.storeId} className="mb-1 rounded-lg px-2 py-2 last:mb-0">
                          <div className="text-base font-bold text-white">
                            {formatPrice(offer.price)}
                          </div>
                          <div className="text-xs text-slate-300">{offer.storeName}</div>
                          {offer.price === lowestProductPrice && (
                            <span className="mt-1 inline-flex rounded-full bg-cyan-200 px-2 py-0.5 text-[9px] font-extrabold uppercase tracking-wider text-slate-950">
                              Mejor precio
                            </span>
                          )}
                        </div>
                      ))
                    ) : (
                      <span className="text-sm text-slate-300">No disponible</span>
                    )}
                  </td>
                ))}
              </tr>

              <tr>
                <th scope="row" className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200">
                  Valoración
                </th>
                {selected.map((product) => {
                  const isBestRating = product.rating > 0 && product.rating === bestRating;
                  return (
                    <td
                      key={product.id}
                      className={`rounded-xl border p-3 text-center ${getCellTone(isBestRating ? "best" : "default")}`}
                    >
                      <span
                        className={`text-base font-extrabold ${isBestRating ? "text-amber-200" : "text-slate-100"}`}
                      >
                        ★ {product.rating.toFixed(1)}
                      </span>
                      <div className="text-xs text-slate-300">
                        ({product.reviewCount.toLocaleString("es-CL")})
                      </div>
                      {isBestRating && (
                        <span className="mt-1 inline-flex rounded-full bg-amber-200 px-2 py-0.5 text-[9px] font-extrabold uppercase tracking-wider text-slate-950">
                          Mejor valoración
                        </span>
                      )}
                    </td>
                  );
                })}
              </tr>

              {specificationRows.map((group) => (
                <Fragment key={group.title}>
                  <tr>
                    <th
                      colSpan={selected.length + 1}
                      className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[11px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                    >
                      {group.title}
                    </th>
                  </tr>

                  {group.names.map((name) => {
                    const states = getSpecCellStates(
                      name,
                      selected.map((product) => product.specs[name]),
                    );

                    return (
                      <tr key={name}>
                        <th
                          scope="row"
                          className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
                        >
                          {name}
                        </th>

                        {selected.map((product, productIndex) => {
                          const rawValue = product.specs[name]?.trim();
                          const state = states[productIndex] ?? "default";
                          const displayValue = rawValue && rawValue.length > 0 ? rawValue : "No informado";

                          return (
                            <td
                              key={`${product.id}-${name}`}
                              className="p-1.5 text-center align-middle"
                            >
                              <div
                                className={`flex min-h-16 flex-col items-center justify-center rounded-xl border px-3 py-2.5 transition duration-200 hover:-translate-y-0.5 ${getCellTone(state)}`}
                              >
                                <span className="block text-sm font-bold leading-snug">{displayValue}</span>
                                {state === "best" && (
                                  <span className="mt-1.5 inline-flex items-center gap-1 rounded-full border border-cyan-100/70 bg-cyan-100 px-2.5 py-0.5 text-[9px] font-black uppercase tracking-[0.14em] text-slate-950 shadow-[0_0_14px_rgba(103,232,249,0.45)]">
                                    ✦ Mejor
                                  </span>
                                )}
                              </div>
                            </td>
                          );
                        })}
                      </tr>
                    );
                  })}
                </Fragment>
              ))}

            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
