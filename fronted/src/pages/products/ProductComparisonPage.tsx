import { Fragment, useEffect, useRef, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import { getProductById } from "../../services/api/api";
import { getProductOfferUpdates } from "../../services/api/stores";
import { compareProducts, type ApiProductAttributeRow } from "../../services/api/personalization";
import {
  formatExactUpdateDate,
  formatPrice,
  formatPriceUpdate,
  getMinOffer,
  validateOfferComparison,
} from "../../services/utils/productUtils";
import { Breadcrumb, Skeleton } from "../../components/common/ui";
import {
  areProductCategoriesCompatible,
  compareBeneficialFeature,
  getBeneficialFeatureLabel,
  getBestSpecLabel,
} from "./productComparisonUtils";
import ComparisonAssistantCard from "./ComparisonAssistantCard";

interface Props {
  productIds: string[];
  navigate: (page: Page) => void;
  cartProductIds: ReadonlySet<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
  onRemoveProduct: (productId: string, category: string) => void;
}

interface ComparedOffer {
  offer: StoreOffer;
}

type SpecCellState = "best" | "benefit" | "equal" | "default";

interface SpecCellComparison {
  states: SpecCellState[];
  unitsNotComparable: boolean;
}

/**
 * Selecciona la mejor oferta disponible de un producto para la matriz de comparación.
 *
 * @param product Producto cuyas ofertas se quieren ordenar.
 * @param validOffers Ofertas con precio válido según la comparación global.
 * @param currencyCompatible Indica si las monedas permiten ordenar por precio.
 * @returns Una oferta representativa del producto o una lista vacía si no hay disponibles.
 */
const getAvailableOffers = (
  product: Product,
  validOffers: ReadonlySet<StoreOffer>,
  currencyCompatible: boolean,
): ComparedOffer[] => {
  const offers = product.offers.filter((offer) => offer.available);
  if (!currencyCompatible) return offers.slice(0, 1).map((offer) => ({ offer }));

  return offers
    .sort((first, second) => {
      const firstIsValid = validOffers.has(first);
      const secondIsValid = validOffers.has(second);
      if (firstIsValid && secondIsValid) return first.price! - second.price!;
      return Number(secondIsValid) - Number(firstIsValid);
    })
    .slice(0, 1)
    .map((offer) => ({ offer }));
};

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

/**
 * Agrupa las especificaciones comunes de los productos en secciones legibles.
 *
 * @param products Productos que se están comparando.
 * @returns Los grupos con nombre y especificaciones asignadas, conservando el orden visual.
 */
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

/**
 * Normaliza un valor para comparar textos sin distinguir tildes o mayúsculas.
 *
 * @param value Texto que se quiere normalizar.
 * @returns El texto en minúsculas, sin tildes ni espacios sobrantes.
 */
const normalizeComparableText = (value: string | undefined): string =>
  (value ?? "")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLowerCase()
    .trim();

/**
 * Extrae el primer número de un valor para comparar magnitudes.
 *
 * @param value Texto que puede contener una cantidad y su unidad.
 * @returns El número encontrado o `null` cuando no hay uno válido.
 */
const parseComparableNumber = (value: string | undefined): number | null => {
  const numericToken = normalizeComparableText(value).match(/[+-]?\d+(?:[.,]\d+)?/);
  if (!numericToken) return null;

  const parsed = Number(numericToken[0].replace(",", "."));
  return Number.isFinite(parsed) ? parsed : null;
};

/**
 * Obtiene una firma de unidad reemplazando las cantidades por un marcador común.
 *
 * @param value Texto que contiene cantidad y unidad.
 * @returns La unidad normalizada para detectar comparaciones incompatibles.
 */
const getComparableUnit = (value: string): string =>
  normalizeComparableText(value)
    .replace(/[+-]?\d+(?:[.,]\d+)?/g, "#")
    .replace(/\b(maxim[oa]s?|minim[oa]s?|soportad[oa]s?|hasta|aprox(?:imad[oa]mente)?)\b/g, "")
    .replace(/[^a-z#]+/g, " ")
    .replace(/\s+/g, " ")
    .trim();

/**
 * Convierte una clase energética A–G y sus signos «+» en una puntuación ordenable.
 *
 * @param value Texto de la eficiencia energética.
 * @returns Una puntuación donde un valor mayor es mejor, o `null` si no se reconoce.
 */
const getEnergyEfficiencyRank = (value: string | undefined): number | null => {
  const normalized = normalizeComparableText(value);
  const grade = normalized.match(/(?:^|[^a-z])(?:clase\s+)?([a-g])(\+{1,3})?(?=$|[^a-z])/);
  if (!grade) return null;

  const baseRank = 26 - (grade[1].charCodeAt(0) - "a".charCodeAt(0));
  return baseRank + (grade[2]?.length ?? 0);
};

/**
 * Comprueba si una especificación representa eficiencia energética.
 *
 * @param specName Nombre de la característica.
 * @returns `true` cuando el valor debe interpretarse como clase energética.
 */
const isEnergyEfficiencySpec = (specName: string): boolean =>
  /(eficiencia|energy.*(?:efficiency|class|rating)|(?:clase|class|etiqueta).*energetic)/i.test(
    normalizeComparableText(specName),
  );

/**
 * Comprueba si una especificación mide consumo eléctrico.
 *
 * @param specName Nombre de la característica.
 * @returns `true` cuando un valor menor representa un mejor consumo.
 */
const isEnergyConsumptionSpec = (specName: string): boolean => {
  const name = normalizeComparableText(specName);
  return /(consumo.*(energia|energetic|electric|kwh|kilowatt)|(?:energy|electricity|electrical).*(consumption|usage|use)|kwh)/i.test(
    name,
  );
};

/**
 * Decide si un valor numérico menor representa una mejor característica.
 *
 * @param specName Nombre de la característica.
 * @returns `true` para consumo, dimensiones o ruido; `false` para capacidad y rendimiento.
 */
const isLowerBetterSpec = (specName: string): boolean => {
  const name = normalizeComparableText(specName);
  const higherIsBetter =
    /(maxim|soport|carga|capacidad|memoria|vram|ram|almacenamiento|rendimiento|velocidad|nucleos|cuda|frecuencia|resolucion|brillo)/i;
  if (higherIsBetter.test(name)) return false;

  return /(consumo|potencia|voltaje|tension|peso|masa|ancho|alto|largo|profundidad|temperatura|watt|kwh|latencia|ruido)/i.test(
    name,
  );
};

/**
 * Determina qué valores destacan al comparar una característica entre productos.
 *
 * @param specName Nombre de la característica.
 * @param values Valor de cada producto; los datos faltantes se representan como `undefined`.
 * @returns El estado visual por producto y si las unidades impiden comparar.
 */
const getSpecCellStates = (
  specName: string,
  values: Array<string | undefined>,
): SpecCellComparison => {
  const entries = values.map((value) => ({
    value: value ?? "",
    normalized: normalizeComparableText(value),
    numeric: parseComparableNumber(value),
    unit: getComparableUnit(value ?? ""),
  }));

  const populated = entries.filter((entry) => entry.normalized.length > 0);

  if (populated.length === 0) {
    return { states: values.map(() => "default"), unitsNotComparable: false };
  }

  const beneficialFeature = compareBeneficialFeature(specName, values);
  if (beneficialFeature) {
    return {
      states: values.map((_, index) => (beneficialFeature[index] ? "benefit" : "default")),
      unitsNotComparable: false,
    };
  }

  if (isEnergyEfficiencySpec(specName)) {
    const energyRanks = entries.map((entry) =>
      entry.normalized ? getEnergyEfficiencyRank(entry.value) : null,
    );
    const populatedRanks = energyRanks.filter((rank) => rank !== null);
    if (populatedRanks.length === populated.length) {
      const bestRank = Math.max(...populatedRanks);
      return {
        states: energyRanks.map((rank) => (rank === bestRank ? "best" : "default")),
        unitsNotComparable: false,
      };
    }
  }

  const textValues = populated.map((entry) => entry.normalized);
  const allSimilarText = new Set(textValues).size <= 1;
  if (allSimilarText) {
    return {
      states: values.map((value) =>
        value && normalizeComparableText(value).length > 0 ? "equal" : "default",
      ),
      unitsNotComparable: false,
    };
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
      return {
        states: values.map((_, index) => (entries[index].normalized ? "equal" : "default")),
        unitsNotComparable: false,
      };
    }

    return {
      states: values.map((_, index) => {
        const entry = entries[index];
        return entry.numeric === bestValue ? "best" : "default";
      }),
      unitsNotComparable: false,
    };
  }

  return {
    states: values.map(() => "default"),
    unitsNotComparable: numericValues.every((num) => num !== null) && units.size > 1,
  };
};

/**
 * Devuelve las clases visuales para un valor comparado.
 *
 * @param state Resultado de comparar la característica.
 * @returns Clases de borde, fondo y texto compatibles con ambos temas.
 */
const getCellTone = (state: SpecCellState) => {
  switch (state) {
    case "best":
      return "border-prime/70 bg-prime-muted text-text shadow-[0_0_24px_rgba(209,168,90,0.16)] ring-1 ring-prime/30";
    case "benefit":
      return "border-success/50 bg-success/10 text-text ring-1 ring-success/20";
    case "equal":
      return "border-bd bg-surface-2 text-text";
    default:
      return "border-bd bg-surface text-text";
  }
};

/**
 * Carga y compara dos o más productos, destacando precios y características beneficiosas.
 *
 * @param productIds Identificadores de los productos seleccionados.
 * @param navigate Callback para cambiar de página.
 * @param cartProductIds Productos que ya están en el carrito.
 * @param onAddToCart Callback para agregar una oferta al carrito.
 * @param onRemoveProduct Callback para quitar un producto de la comparación.
 * @returns La matriz de comparación con beneficios y advertencias por característica.
 */
export default function ProductComparisonPage({
  productIds,
  navigate,
  cartProductIds,
  onAddToCart,
  onRemoveProduct,
}: Props) {
  const [selected, setSelected] = useState<Product[]>([]);
  const [comparisonCategory, setComparisonCategory] = useState<string>();
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [hasMissingProducts, setHasMissingProducts] = useState(false);
  const [attributeRows, setAttributeRows] = useState<ApiProductAttributeRow[]>([]);
  const [loadAttempt, setLoadAttempt] = useState(0);
  const [addingProductIds, setAddingProductIds] = useState<Set<string>>(() => new Set());
  const addingProductIdsRef = useRef(new Set<string>());

  /**
   * Agrega un producto al carrito evitando solicitudes duplicadas.
   *
   * @param product Producto que se quiere agregar.
   * @param offer Oferta disponible seleccionada para ese producto.
   * @returns Una promesa que termina al finalizar el callback de carrito.
   */
  const addProductToCart = async (product: Product, offer: StoreOffer) => {
    if (cartProductIds.has(product.id) || addingProductIdsRef.current.has(product.id)) return;
    addingProductIdsRef.current.add(product.id);
    setAddingProductIds((previous) => new Set(previous).add(product.id));
    try {
      await onAddToCart(product, offer);
    } finally {
      addingProductIdsRef.current.delete(product.id);
      setAddingProductIds((previous) => {
        const next = new Set(previous);
        next.delete(product.id);
        return next;
      });
    }
  };

  useEffect(() => {
    let cancelled = false;

    /**
     * Carga los productos, su matriz de atributos y las fechas de verificación de ofertas.
     *
     * @returns Una promesa que actualiza el estado local cuando termina la carga.
     */
    async function loadProducts() {
      if (productIds.length < 2) {
        setSelected([]);
        setAttributeRows([]);
        setHasMissingProducts(false);
        setError(null);
        setLoading(false);
        return;
      }

      setLoading(true);
      setError(null);
      setHasMissingProducts(false);
      try {
        const results = await Promise.all(productIds.map((id) => getProductById(id)));
        if (cancelled) return;

        const valid = results.filter((product): product is Product => product !== null);
        if (valid.length !== results.length) {
          setHasMissingProducts(true);
          setSelected([]);
          setAttributeRows([]);
          setComparisonCategory(undefined);
          return;
        }

        const [matrix, offerUpdates] = await Promise.all([
          compareProducts(productIds),
          Promise.all(
            productIds.map(async (id) => {
              try {
                return await getProductOfferUpdates(id);
              } catch (requestError) {
                console.warn(
                  "No se pudieron cargar las fechas de verificación de ofertas.",
                  requestError,
                );
                return undefined;
              }
            }),
          ),
        ]);
        if (cancelled) return;
        const validWithOfferUpdates = valid.map((product) => {
          const productIndex = productIds.indexOf(product.id);
          const updatesByOfferId = new Map<number, string>();
          for (const update of offerUpdates[productIndex] ?? []) {
            updatesByOfferId.set(update.offerId, update.lastUpdated);
          }

          return {
            ...product,
            offers: product.offers.map((offer) => {
              const lastUpdated = updatesByOfferId.get(offer.offerId);
              return lastUpdated === undefined ? offer : { ...offer, lastUpdated };
            }),
          };
        });
        setSelected(validWithOfferUpdates);
        setAttributeRows(matrix.attributes ?? []);
        setComparisonCategory(validWithOfferUpdates[0]?.category);
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
  }, [productIds, loadAttempt]);

  if (loading) {
    return (
      <div role="status" aria-label="Cargando comparación" className="mx-auto max-w-7xl px-4 py-8">
        <Skeleton className="mb-6 h-4 w-44" />
        <Skeleton className="mb-6 h-8 w-64" />
        <div className="grid gap-4 sm:grid-cols-2">
          {[0, 1, 2].map((item) => (
            <div key={item} className="rounded-2xl border border-bd bg-surface p-5">
              <Skeleton className="mb-4 h-40 w-full" />
              <Skeleton className="mb-3 h-6 w-3/4" />
              <Skeleton className="mb-6 h-4 w-1/2" />
              <Skeleton className="h-10 w-full" />
            </div>
          ))}
        </div>
        <Skeleton className="mt-6 h-64 w-full rounded-2xl" />
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="mb-4 text-danger">{error}</p>
        <button
          onClick={() => setLoadAttempt((attempt) => attempt + 1)}
          style={{ background: "linear-gradient(135deg, #ff9878 0%, #fb7185 100%)", color: "#fff" }}
          className="rounded-xl px-5 py-2 text-sm font-semibold shadow-md shadow-rose-500/15 transition hover:brightness-105"
        >
          Reintentar
        </button>
      </div>
    );
  }

  if (selected.length < 2) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="mb-4 text-muted" role={hasMissingProducts ? "alert" : undefined}>
          {hasMissingProducts
            ? "Uno o más productos seleccionados no existen o ya no están disponibles. Busca y selecciona otros productos."
            : "Selecciona al menos 2 productos para comparar."}
        </p>
        <button
          onClick={() =>
            navigate({
              id: "search-products",
              query: "",
              ...(comparisonCategory ? { category: comparisonCategory } : {}),
            })
          }
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
        <p role="alert" className="mb-4 text-warn">
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

  const offerComparison = validateOfferComparison(selected.flatMap((product) => product.offers));
  const validOffers = new Set(offerComparison.validPriceOffers);
  const productOffers = selected.map((product) =>
    getAvailableOffers(product, validOffers, offerComparison.currencyCompatible),
  );
  const cartOffers = selected.map((product) => getMinOffer(product));
  const specificationRows = getSpecificationGroups(selected);
  const bestRating = Math.max(...selected.map((product) => product.rating));
  const comparableProductPrices = offerComparison.canCompare
    ? offerComparison.availableOffers.flatMap((offer) =>
        offer.price === null ? [] : [offer.price],
      )
    : [];
  const lowestProductPrice =
    comparableProductPrices.length > 0 ? Math.min(...comparableProductPrices) : null;
  const assistantProductIds = selected.map((product) => product.id);

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Productos", onClick: () => navigate({ id: "search-products", query: "" }) },
          { label: "Comparación" },
        ]}
      />

      <h1 className="mb-2 text-2xl font-bold text-text">Comparación de productos</h1>
      <p className="mb-8 text-sm text-muted">Comparando {selected.length} productos</p>
      <div className="rounded-[28px] border border-bd bg-surface p-3 shadow-[0_24px_70px_rgba(0,0,0,0.16)] backdrop-blur-sm sm:p-5">
        <p className="mb-4 flex flex-wrap items-center gap-2 rounded-xl border border-bd bg-surface-2 px-3 py-2 text-[11px] text-muted">
          <span className="inline-flex items-center gap-1 rounded-full border border-prime/50 bg-prime px-2 py-1 font-extrabold text-[#181714]">
            ✦ Mejor
          </span>
          <span className="inline-flex rounded-full border border-success/50 bg-success/10 px-2 py-1 font-semibold text-success">
            Beneficio
          </span>
          El dorado identifica el mejor valor comparable; el verde resalta funciones beneficiosas
          incluidas.
        </p>
        {!offerComparison.canCompare && (
          <p role="status" className="mb-4 text-sm text-warn">
            {offerComparison.hasMixedCurrencies
              ? "Monedas distintas: los precios no son comparables."
              : !offerComparison.currencyCompatible && offerComparison.validPriceOffers.length > 0
                ? "Moneda no compatible: los precios no son comparables."
                : "Se requieren al menos dos ofertas válidas para comparar precios."}
          </p>
        )}
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] border-separate border-spacing-1.5">
            <thead>
              <tr>
                <th className="w-44 rounded-xl border border-bd bg-surface-2 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.18em] text-text">
                  Característica
                </th>
                {selected.map((product, index) => {
                  const cartOffer = cartOffers[index];
                  const offerInCart = cartProductIds.has(product.id);
                  const offerIsAdding = addingProductIds.has(product.id);
                  return (
                    <th
                      key={product.id}
                      className="rounded-xl border border-bd bg-surface-2 p-4 text-center"
                    >
                      <div className="flex flex-col items-center gap-3">
                        {product.image ? (
                          <img
                            src={product.image}
                            alt={product.name}
                            className="h-16 w-20 rounded-2xl object-contain"
                          />
                        ) : (
                          <div className="flex h-16 w-20 items-center justify-center rounded-2xl bg-surface-3 text-[10px] text-muted">
                            Sin imagen
                          </div>
                        )}

                        <div>
                          <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-prime">
                            {product.brand}
                          </div>
                          <div className="mt-1 max-w-[180px] text-sm font-semibold leading-tight text-text">
                            {product.name}
                          </div>
                        </div>

                        <button
                          onClick={() => navigate({ id: "product-detail", productId: product.id })}
                          className="rounded-lg border border-bd bg-surface-2 px-3 py-1 text-[11px] font-medium text-text transition-colors hover:border-prime hover:text-prime"
                        >
                          Ver detalle
                        </button>

                        <button
                          type="button"
                          disabled={!cartOffer && !offerInCart}
                          onClick={() => cartOffer && void addProductToCart(product, cartOffer)}
                          aria-live="polite"
                          aria-disabled={offerInCart || offerIsAdding}
                          className={`rounded-lg px-3 py-1.5 text-[11px] font-semibold shadow-md transition ${
                            offerInCart
                              ? "border border-success/50 bg-success/15 text-success shadow-success/10"
                              : offerIsAdding
                                ? "border border-success/50 bg-success/15 text-success shadow-success/10"
                                : "bg-prime text-[#181714] shadow-black/15 hover:bg-prime-dark"
                          } disabled:cursor-not-allowed disabled:opacity-40`}
                        >
                          {offerInCart
                            ? "Agregado en la cesta"
                            : offerIsAdding
                              ? "Agregando a la cesta..."
                              : "Agregar al carrito"}
                        </button>
                        <button
                          type="button"
                          onClick={() => {
                            setSelected((products) =>
                              products.filter(
                                (selectedProduct) => selectedProduct.id !== product.id,
                              ),
                            );
                            onRemoveProduct(product.id, product.category);
                          }}
                          className="rounded-lg border border-danger/40 bg-danger/10 px-3 py-1.5 text-[11px] font-semibold text-danger transition hover:border-danger hover:bg-danger/20"
                        >
                          Quitar producto
                        </button>
                      </div>
                    </th>
                  );
                })}
              </tr>
            </thead>

            <tbody>
              <tr>
                <th
                  scope="row"
                  className="rounded-xl border border-bd bg-surface-2 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-text"
                >
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
                          <div
                            className={`font-bold text-text ${
                              offer.price === lowestProductPrice
                                ? "text-xl sm:text-2xl"
                                : "text-base"
                            }`}
                          >
                            {!validOffers.has(offer)
                              ? "Precio no disponible"
                              : offerComparison.currencyCompatible
                                ? formatPrice(offer.price)
                                : "No comparable"}
                          </div>
                          <div className="text-xs text-muted">{offer.storeName}</div>
                          <div
                            className="text-[10px] text-muted"
                            title={
                              formatExactUpdateDate(offer.lastUpdated) ??
                              "Fecha de actualización no disponible"
                            }
                          >
                            {formatExactUpdateDate(offer.lastUpdated)
                              ? `Última verificación: ${formatPriceUpdate(offer.lastUpdated)}`
                              : "Fecha de actualización no disponible"}
                          </div>
                          {offer.price === lowestProductPrice && (
                            <span className="mt-1 inline-flex rounded-full bg-prime px-2 py-0.5 text-[9px] font-extrabold uppercase tracking-wider text-[#181714]">
                              Mejor precio
                            </span>
                          )}
                        </div>
                      ))
                    ) : (
                      <span className="text-sm text-muted">No disponible</span>
                    )}
                  </td>
                ))}
              </tr>

              <tr>
                <th
                  scope="row"
                  className="rounded-xl border border-bd bg-surface-2 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-text"
                >
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
                        className={`text-base font-extrabold ${isBestRating ? "text-prime" : "text-text"}`}
                      >
                        ★ {product.rating.toFixed(1)}
                      </span>
                      <div className="text-xs text-muted">
                        ({product.reviewCount.toLocaleString("es-CL")})
                      </div>
                      {isBestRating && (
                        <span className="mt-1 inline-flex rounded-full bg-prime px-2 py-0.5 text-[9px] font-extrabold uppercase tracking-wider text-[#181714]">
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
                      className="rounded-xl border border-bd bg-surface-2 px-4 py-3 text-left text-[11px] font-extrabold uppercase tracking-[0.18em] text-prime"
                    >
                      {group.title}
                    </th>
                  </tr>

                  {group.names.map((name) => {
                    const comparison = getSpecCellStates(
                      name,
                      selected.map((product) => product.specs[name]),
                    );

                    return (
                      <tr key={name}>
                        <th
                          scope="row"
                          className="rounded-xl border border-bd bg-surface-2 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-text"
                        >
                          {name}
                          {comparison.unitsNotComparable && (
                            <span className="mt-1 block normal-case tracking-normal text-warn">
                              Unidades no comparables
                            </span>
                          )}
                        </th>

                        {selected.map((product, productIndex) => {
                          const rawValue = product.specs[name]?.trim();
                          const state = comparison.states[productIndex] ?? "default";
                          const displayValue =
                            rawValue && rawValue.length > 0 ? rawValue : "No informado";

                          return (
                            <td
                              key={`${product.id}-${name}`}
                              className="p-1.5 text-center align-middle"
                            >
                              <div
                                className={`flex min-h-16 flex-col items-center justify-center rounded-xl border px-3 py-2.5 transition duration-200 hover:-translate-y-0.5 ${getCellTone(state)}`}
                              >
                                <span className="block text-sm font-bold leading-snug">
                                  {displayValue}
                                </span>
                                {state === "best" && (
                                  <span className="mt-1.5 inline-flex items-center gap-1 rounded-full border border-prime/70 bg-prime px-2.5 py-0.5 text-[9px] font-black uppercase tracking-[0.14em] text-[#181714]">
                                    ✦ {getBestSpecLabel(name)}
                                  </span>
                                )}
                                {state === "benefit" && (
                                  <span className="mt-1.5 inline-flex rounded-full border border-success/50 bg-success/10 px-2.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.12em] text-success">
                                    {getBeneficialFeatureLabel(name)}
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

              {attributeRows.length > 0 && (
                <>
                  <tr>
                    <th
                      colSpan={selected.length + 1}
                      className="rounded-xl border border-bd bg-surface-2 px-4 py-3 text-left text-[11px] font-extrabold uppercase tracking-[0.18em] text-prime"
                    >
                      Atributos comparables de {selected[0].category}
                    </th>
                  </tr>
                  {attributeRows.map((attribute) => {
                    const values = attribute.values.map((value) =>
                      value === null || value === undefined ? undefined : String(value),
                    );
                    const comparison = getSpecCellStates(attribute.label, values);

                    return (
                      <tr key={attribute.key}>
                        <th
                          scope="row"
                          className="rounded-xl border border-bd bg-surface-2 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-text"
                        >
                          {attribute.label}
                          {attribute.unit ? ` (${attribute.unit})` : ""}
                          {comparison.unitsNotComparable && (
                            <span className="mt-1 block normal-case tracking-normal text-warn">
                              Unidades no comparables
                            </span>
                          )}
                        </th>
                        {selected.map((product, index) => {
                          const value = attribute.values[index];
                          const displayValue =
                            value === null || value === undefined
                              ? "No informado"
                              : typeof value === "boolean"
                                ? value
                                  ? "Sí"
                                  : "No"
                                : String(value);
                          const state =
                            comparison.states[index] ?? (attribute.differs ? "default" : "equal");

                          return (
                            <td
                              key={`${product.id}-${attribute.key}`}
                              className={`rounded-xl border p-3 text-center ${getCellTone(state)}`}
                            >
                              <span className="block text-sm font-bold leading-snug">
                                {displayValue}
                              </span>
                              {state === "best" && (
                                <span className="mt-1.5 inline-flex items-center gap-1 rounded-full border border-prime/70 bg-prime px-2.5 py-0.5 text-[9px] font-black uppercase tracking-[0.14em] text-[#181714]">
                                  ✦ {getBestSpecLabel(attribute.label)}
                                </span>
                              )}
                              {state === "benefit" && (
                                <span className="mt-1.5 inline-flex rounded-full border border-success/50 bg-success/10 px-2.5 py-0.5 text-[9px] font-bold uppercase tracking-[0.12em] text-success">
                                  {getBeneficialFeatureLabel(attribute.label)}
                                </span>
                              )}
                            </td>
                          );
                        })}
                      </tr>
                    );
                  })}
                </>
              )}
            </tbody>
          </table>
        </div>
      </div>

      <ComparisonAssistantCard
        key={assistantProductIds.join(",")}
        productIds={assistantProductIds}
      />
    </div>
  );
}
