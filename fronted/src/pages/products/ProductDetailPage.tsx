import { useEffect, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import {
  formatExactUpdateDate,
  formatPriceUpdate,
  keepLatestOfferPerStore,
} from "../../services/utils/productUtils";
import {
  getProductById,
  getProductReviews,
  type ApiProductReview,
} from "../../services/api/products";
import { getCategoryBySlug, type CategoryAttribute } from "../../services/api/categories";
import { getProductOfferUpdates, type ApiProductOfferUpdate } from "../../services/api/stores";
import {
  Breadcrumb,
  FavoriteButton,
  Rating,
  Skeleton,
  SkeletonGrid,
} from "../../components/common/ui";
import PriceHistory from "../../components/products/PriceHistory";
import PriceOfferComparison from "./PriceOfferComparison";
import { getProductSubtype, getProductSubtypeSearchQuery } from "../../constants/productCatalog";
import { findProductCategoryPath } from "../../constants/categoryTree";

interface Props {
  productId: string;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string, category: string) => void;
  cartProductIds: ReadonlySet<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
}

type ProductSpecification = [name: string, value: string];

const primarySpecificationPatterns = [
  /(capacidad|volumen|carga m[aá]xima|peso m[aá]ximo|soporte)/i,
  /(consumo|potencia|energ[ií]a|eficiencia|voltaje|alimentaci[oó]n)/i,
  /(peso|masa)/i,
  /(dimensi[oó]n|ancho|alto|largo|profundidad|di[aá]metro)/i,
  /(velocidad|programa|rendimiento)/i,
];

/**
 * Selecciona las cuatro especificaciones más útiles para el resumen del producto.
 *
 * @param specs Especificaciones disponibles del producto.
 * @returns Las características prioritarias y, si faltan, las primeras adicionales.
 */
const getPrimarySpecifications = (specs: Record<string, string>): ProductSpecification[] => {
  const entries = Object.entries(specs) as ProductSpecification[];
  const primary: ProductSpecification[] = [];

  for (const pattern of primarySpecificationPatterns) {
    const match = entries.find(
      ([name]) => pattern.test(name) && !primary.some(([key]) => key === name),
    );
    if (match) primary.push(match);
    if (primary.length === 4) break;
  }

  for (const entry of entries) {
    if (primary.length === 4) break;
    if (!primary.some(([name]) => name === entry[0])) primary.push(entry);
  }

  return primary;
};

/**
 * Agrupa las especificaciones del producto en secciones de la ficha.
 *
 * @param specs Especificaciones que se quieren organizar.
 * @returns Los grupos con título y características asignadas.
 */
const groupProductSpecifications = (specs: Record<string, string>) => {
  const groups = [
    { title: "Capacidad y carga", test: /(capacidad|volumen|carga|soporte)/i },
    {
      title: "Consumo y energía",
      test: /(consumo|energ[ií]a|eficiencia|potencia|tensi[oó]n|voltaje|alimentaci[oó]n|watt|kwh)/i,
    },
    {
      title: "Peso y dimensiones",
      test: /(peso|masa|dimensi[oó]n|ancho|alto|largo|profundidad|di[aá]metro|superficie)/i,
    },
    {
      title: "Rendimiento",
      test: /(velocidad|programa|rpm|temperatura|rendimiento|presi[oó]n|inclinaci[oó]n)/i,
    },
    { title: "Otros", test: /.*/ },
  ];
  const entries = Object.entries(specs) as ProductSpecification[];
  const assigned = new Set<string>();

  return groups
    .map(({ title, test }) => {
      const items = entries.filter(([name]) => {
        if (assigned.has(name) || !test.test(name)) return false;
        assigned.add(name);
        return true;
      });
      return { title, items };
    })
    .filter((group) => group.items.length > 0);
};

type ProductAttributes = Record<string, string | number | boolean>;

/**
 * Convierte una clave técnica de atributo (capacidad_litros) en una etiqueta
 * legible (Capacidad litros). Solo se usa cuando la categoría no entrega el
 * label de ese atributo.
 * @param key Clave del atributo.
 * @returns La etiqueta con la primera letra en mayúscula.
 */
const formatAttributeKey = (key: string): string =>
  key
    .replace(/[_-]+/g, " ")
    .trim()
    .replace(/^./, (char) => char.toLocaleUpperCase("es"));

/**
 * Arma las especificaciones visibles del producto desde los atributos que
 * devuelve Product API, con la etiqueta y la unidad que define la categoría.
 * Product API entrega los datos en `attributes` y deja `specs` vacío, así que
 * esta conversión es la que llena las tablas de características de la ficha.
 * @param attributes Atributos tipados del producto.
 * @param definitions Definiciones de la categoría, para label y unidad.
 * @returns Un mapa nombre → valor listo para las tablas de la ficha.
 */
const buildSpecifications = (
  attributes: ProductAttributes | undefined,
  definitions: CategoryAttribute[],
): Record<string, string> => {
  const definitionsByKey = new Map(definitions.map((definition) => [definition.key, definition]));
  const specifications: Record<string, string> = {};

  for (const [key, rawValue] of Object.entries(attributes ?? {})) {
    const definition = definitionsByKey.get(key);
    const label = definition
      ? `${definition.label}${definition.unit ? ` (${definition.unit})` : ""}`
      : formatAttributeKey(key);
    specifications[label] =
      typeof rawValue === "boolean" ? (rawValue ? "Sí" : "No") : String(rawValue);
  }

  return specifications;
};

/**
 * Muestra la ficha de un producto, sus características, historial y ofertas por tienda.
 *
 * @param productId Identificador del producto que se quiere mostrar.
 * @param navigate Callback para cambiar de página.
 * @param isFavorite Indica si el producto está en favoritos.
 * @param isComparing Indica si el producto está en el comparador.
 * @param onToggleFavorite Callback para alternar el favorito.
 * @param onToggleCompare Callback para alternar la comparación.
 * @param cartProductIds Productos que ya están en el carrito.
 * @param onAddToCart Callback para agregar una oferta al carrito.
 * @returns La ficha completa del producto con una oferta reciente por tienda.
 */
export default function ProductDetailPage({
  productId,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
  cartProductIds,
  onAddToCart,
}: Props) {
  const [productState, setProductState] = useState<{
    requestKey: string;
    value: Product | null;
  } | null>(null);
  const [offerUpdateState, setOfferUpdateState] = useState<{
    requestKey: string;
    updates: ApiProductOfferUpdate[];
  } | null>(null);
  const [loadedRequestKey, setLoadedRequestKey] = useState("");
  const [selectedImage, setSelectedImage] = useState(0);
  const [reviewState, setReviewState] = useState<{
    requestKey: string;
    items: ApiProductReview[];
  } | null>(null);
  const [productError, setProductError] = useState<{
    requestKey: string;
    message: string;
  } | null>(null);
  const [reviewsError, setReviewsError] = useState<{
    requestKey: string;
    message: string;
  } | null>(null);
  const [categoryDefinitions, setCategoryDefinitions] = useState<{
    slug: string;
    attributes: CategoryAttribute[];
  }>({ slug: "", attributes: [] });
  const [loadAttempt, setLoadAttempt] = useState(0);
  const requestKey = `${productId}:${loadAttempt}`;
  const loading = loadedRequestKey !== requestKey;
  const storedProduct = productState?.requestKey === requestKey ? productState.value : null;
  const offerUpdates = offerUpdateState?.requestKey === requestKey ? offerUpdateState.updates : [];
  const product = storedProduct
    ? {
        ...storedProduct,
        offers: keepLatestOfferPerStore(
          storedProduct.offers.map((offer) => {
            const update = offerUpdates.find((item) => item.offerId === offer.offerId);
            return update ? { ...offer, lastUpdated: update.lastUpdated } : offer;
          }),
        ),
      }
    : null;
  const latestOfferUpdate = product?.offers.reduce<string | undefined>((latest, offer) => {
    const updatedAt = offer.lastUpdated;
    if (!updatedAt || !formatExactUpdateDate(updatedAt)) return latest;
    return !latest || Date.parse(updatedAt) > Date.parse(latest) ? updatedAt : latest;
  }, undefined);
  const reviews = reviewState?.requestKey === requestKey ? reviewState.items : [];
  const currentProductError = productError?.requestKey === requestKey ? productError.message : null;
  const currentReviewsError = reviewsError?.requestKey === requestKey ? reviewsError.message : null;
  const reviewsLoading = reviewState?.requestKey !== requestKey && currentReviewsError === null;

  useEffect(() => {
    let cancelled = false;
    const currentRequestKey = `${productId}:${loadAttempt}`;

    getProductOfferUpdates(productId)
      .then((updates) => {
        if (!cancelled && updates) {
          setOfferUpdateState({ requestKey: currentRequestKey, updates });
        }
      })
      .catch((error: unknown) => {
        console.warn("No se pudieron cargar las fechas de verificación de ofertas.", error);
      });

    getProductById(productId)
      .then((result) => {
        if (!cancelled) {
          setSelectedImage(0);
          setProductState({ requestKey: currentRequestKey, value: result });
          setCategoryDefinitions({ slug: result?.categorySlug ?? "", attributes: [] });
          const slug = result?.categorySlug;
          if (slug) {
            // Las etiquetas de los atributos viven en la definición de la
            // categoría; si falla, quedan las claves técnicas del producto.
            getCategoryBySlug(slug)
              .then((category) => {
                if (!cancelled) {
                  setCategoryDefinitions({ slug, attributes: category.attributes });
                }
              })
              .catch(() => undefined);
          }
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setProductState({ requestKey: currentRequestKey, value: null });
          setProductError({
            requestKey: currentRequestKey,
            message: error instanceof Error ? error.message : "Product API no disponible.",
          });
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoadedRequestKey(currentRequestKey);
        }
      });

    // Si fallan las reseñas, la ficha del producto se muestra igual.
    getProductReviews(productId)
      .then((result) => {
        if (!cancelled) {
          setReviewState({ requestKey: currentRequestKey, items: result });
        }
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          setReviewsError({
            requestKey: currentRequestKey,
            message:
              error instanceof Error ? error.message : "Product API no disponible para reseñas.",
          });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [productId, loadAttempt]);

  if (loading)
    return (
      <div role="status" aria-label="Cargando producto" className="mx-auto max-w-7xl px-4 py-8">
        <Skeleton className="mb-6 h-4 w-44" />
        <div className="grid gap-8 lg:grid-cols-2">
          <Skeleton className="min-h-80 rounded-2xl sm:min-h-[28rem]" />
          <div className="space-y-4">
            <Skeleton className="h-8 w-4/5" />
            <Skeleton className="h-5 w-2/5" />
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-10 w-1/2" />
            <Skeleton className="h-14 w-full rounded-xl" />
          </div>
        </div>
        <Skeleton className="mt-10 h-48 w-full rounded-2xl" />
        <div className="mt-8 rounded-2xl border border-bd bg-surface p-6">
          <Skeleton className="mb-5 h-6 w-56" />
          <SkeletonGrid count={2} />
        </div>
      </div>
    );

  if (!product)
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        {currentProductError ? (
          <>
            <p role="alert" className="text-warn">
              {currentProductError}
            </p>
            <button
              type="button"
              onClick={() => setLoadAttempt((attempt) => attempt + 1)}
              className="mt-4 rounded-xl border border-prime px-4 py-2 text-sm font-semibold text-prime"
            >
              Reintentar Product API
            </button>
          </>
        ) : (
          <p className="text-muted">Product API no encontró este producto.</p>
        )}
      </div>
    );

  const specifications =
    Object.keys(product.specs).length > 0
      ? product.specs
      : buildSpecifications(
          product.attributes,
          categoryDefinitions.slug === product.categorySlug ? categoryDefinitions.attributes : [],
        );
  const primarySpecifications = getPrimarySpecifications(specifications);
  const specificationGroups = groupProductSpecifications(specifications);
  const productImages = [...new Set([product.image, ...product.images].filter(Boolean))];
  // Ruta NV (Hogar > Electrodomésticos > Refrigeradores) desde el slug que
  // entrega el backend; si el producto aún no tiene slug, se cae al nombre legacy.
  const categoryPath = product.categorySlug
    ? (findProductCategoryPath(product.categorySlug) ?? [])
    : [];
  const productSubcategory = product.subcategory.trim() || getProductSubtype(product);
  const lastCategoryPathName = categoryPath.at(-1)?.name ?? "";
  // La ruta NV ya termina en la subcategoría, así que solo se agrega un tramo
  // extra cuando la subcategoría aporta un nombre distinto.
  const hasDistinctSubcategory =
    productSubcategory.toLocaleLowerCase("es") !== product.category.toLocaleLowerCase("es") &&
    productSubcategory.toLocaleLowerCase("es") !== lastCategoryPathName.toLocaleLowerCase("es");

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Productos",
            onClick: () => navigate({ id: "search-products", query: "" }),
          },
          ...(categoryPath.length > 0
            ? categoryPath.map((node, index) => ({
                label: node.name,
                ...(index < categoryPath.length - 1
                  ? {
                      onClick: () =>
                        navigate({
                          id: "search-products",
                          query: "",
                          category: node.slug,
                          categoryContext: node.name,
                          resetFilters: true,
                        }),
                    }
                  : {}),
              }))
            : [
                {
                  label: product.category,
                  onClick: () =>
                    navigate({
                      id: "search-products",
                      query: "",
                      category: product.category,
                    }),
                },
              ]),
          ...(hasDistinctSubcategory
            ? [
                {
                  label: productSubcategory,
                  onClick: () =>
                    navigate({
                      id: "search-products",
                      query: getProductSubtypeSearchQuery(productSubcategory),
                      categoryContext: productSubcategory,
                      categoryId: product.categoryId,
                    }),
                },
              ]
            : []),
          { label: product.name },
        ]}
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        <div>
          <div
            style={{
              background: "linear-gradient(135deg, var(--color-surface-3), var(--color-surface))",
              border: "1px solid var(--color-bd)",
              boxShadow: "0 18px 40px rgba(0, 0, 0, 0.25)",
            }}
            className="rounded-2xl overflow-hidden h-[420px] md:h-[520px] mb-3"
          >
            {productImages.length > 0 ? (
              <img
                src={productImages[selectedImage] ?? productImages[0]}
                alt={product.name}
                className="w-full h-full object-contain p-4 transition-all duration-200"
                style={{
                  background:
                    "radial-gradient(circle at top, rgba(209,168,90,0.12), transparent 40%)",
                }}
              />
            ) : (
              <div className="flex h-full items-center justify-center text-sm text-muted">
                Sin imagen
              </div>
            )}
          </div>

          <div className="flex gap-2 overflow-x-auto pb-1">
            {productImages.map((img, i) => (
              <button
                key={i}
                onClick={() => setSelectedImage(i)}
                type="button"
                aria-label={`Ver imagen ${i + 1} de ${product.name}`}
                aria-pressed={i === selectedImage}
                style={{
                  border: `2px solid ${i === selectedImage ? "#D1A85A" : "var(--color-bd)"}`,
                  boxShadow: i === selectedImage ? "0 0 0 2px rgba(209,168,90,0.12)" : "none",
                }}
                className="w-16 h-16 rounded-xl overflow-hidden transition-all shrink-0 bg-surface"
              >
                <img src={img} alt="" className="w-full h-full object-contain p-1" />
              </button>
            ))}
          </div>
        </div>

        <div className="flex flex-col gap-4">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="inline-flex items-center gap-2 rounded-full border border-prime/40 bg-prime-muted px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-prime">
                {product.brand} · {product.category}
              </span>
              <FavoriteButton active={isFavorite} onClick={() => onToggleFavorite(product.id)} />
            </div>

            <h1 className="text-2xl md:text-3xl font-black text-text leading-tight tracking-[-0.04em]">
              {product.name}
            </h1>

            <div className="mt-3">
              <Rating value={product.rating} count={product.reviewCount} />
            </div>
            <p
              className="mt-2 text-xs text-muted"
              title={
                formatExactUpdateDate(latestOfferUpdate) ?? "Fecha de actualización no disponible"
              }
            >
              {latestOfferUpdate
                ? `Última verificación de precios: ${formatPriceUpdate(latestOfferUpdate)}`
                : "Fecha de actualización no disponible"}
            </p>
          </div>

          <div
            style={{
              background: "linear-gradient(180deg, var(--color-surface-2), var(--color-surface))",
              border: "1px solid var(--color-bd)",
              boxShadow: "0 14px 30px rgba(0, 0, 0, 0.2)",
            }}
            className="rounded-xl p-4"
          >
            <h3 className="text-sm font-semibold text-text mb-3">Características principales</h3>

            {primarySpecifications.length > 0 ? (
              <dl className="grid grid-cols-2 xl:grid-cols-4 gap-x-4 gap-y-3">
                {primarySpecifications.map(([name, value]) => (
                  <div
                    key={name}
                    className="min-w-0 rounded-xl border border-bd bg-surface-2 pl-3 py-2"
                  >
                    <dt className="text-[10px] uppercase tracking-[0.14em] text-muted">{name}</dt>
                    <dd className="mt-2 text-sm font-semibold text-text break-words">{value}</dd>
                  </div>
                ))}
              </dl>
            ) : (
              <p className="text-sm text-muted">No hay características informadas.</p>
            )}
          </div>

          <p className="text-sm leading-relaxed text-muted">
            {product.description || "Este producto aún no tiene una descripción disponible."}
          </p>
        </div>
      </div>

      <section
        style={{
          background: "linear-gradient(180deg, var(--color-surface-2), var(--color-surface))",
          border: "1px solid var(--color-bd)",
        }}
        className="rounded-xl p-5 sm:p-6 mb-6"
      >
        <h2 className="text-lg font-semibold text-text">Especificaciones técnicas</h2>

        {specificationGroups.length > 0 ? (
          <div className="mt-4 space-y-5">
            {specificationGroups.map((group) => (
              <section key={group.title}>
                <h3 className="text-xs font-semibold uppercase tracking-[0.16em] text-prime">
                  {group.title}
                </h3>
                <dl className="mt-2 grid grid-cols-1 gap-x-8 md:grid-cols-2">
                  {group.items.map(([name, value]) => (
                    <div
                      key={name}
                      className="flex items-start justify-between gap-4 border-b border-bd py-3"
                    >
                      <dt className="min-w-0 text-sm text-muted">{name}</dt>
                      <dd className="max-w-[60%] break-words text-right text-sm font-semibold text-text">
                        {value}
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            ))}
          </div>
        ) : (
          <p className="mt-4 text-sm text-muted">No hay especificaciones técnicas informadas.</p>
        )}
      </section>

      <PriceOfferComparison
        product={product}
        cartProductIds={cartProductIds}
        onAddToCart={onAddToCart}
        navigate={navigate}
        isComparing={isComparing}
        onToggleCompare={onToggleCompare}
      />

      <PriceHistory
        history={product.priceHistory}
        offerHistory={product.offerPriceHistory}
        currentOfferPrice={product.offerPrice}
        offers={product.offers}
      />

      {/* Reviews */}
      <section
        style={{ background: "var(--color-surface)", border: "1px solid var(--color-bd)" }}
        className="rounded-2xl p-6 mt-6"
      >
        <div className="flex items-center justify-between gap-4 mb-5 flex-wrap">
          <h2 className="text-lg font-bold text-text">Reseñas y calificaciones</h2>
          <Rating value={product.rating} count={product.reviewCount} />
        </div>
        {reviewsLoading ? (
          <SkeletonGrid count={2} />
        ) : currentReviewsError ? (
          <div
            className="mb-4 rounded-xl border border-rose-300/20 bg-rose-950/30 p-3 text-sm text-rose-200"
            role="alert"
          >
            {currentReviewsError}
            <button
              type="button"
              onClick={() => setLoadAttempt((attempt) => attempt + 1)}
              className="ml-3 font-semibold underline"
            >
              Reintentar
            </button>
          </div>
        ) : reviews.length === 0 ? (
          <p className="text-sm text-muted">Este producto aún no tiene reseñas.</p>
        ) : (
          <div className="flex flex-col gap-3">
            {reviews.map((review, i) => (
              <article
                key={`${review.author}-${review.createdAt}-${i}`}
                style={{ background: "var(--color-surface-2)" }}
                className="rounded-xl p-4"
              >
                <div className="flex items-center justify-between gap-3 flex-wrap mb-2">
                  <div className="flex items-center gap-2">
                    <Rating value={review.rating} />
                  </div>
                  <span className="text-xs text-muted">
                    {review.author}
                    {review.createdAt &&
                      ` · ${new Date(review.createdAt).toLocaleDateString("es-CL")}`}
                  </span>
                </div>
                {review.title && (
                  <h3 className="text-sm font-semibold text-text mb-1">{review.title}</h3>
                )}
                {review.content && <p className="text-sm text-muted">{review.content}</p>}
              </article>
            ))}
          </div>
        )}
      </section>
    </div>
  );
}
