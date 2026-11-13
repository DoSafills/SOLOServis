import { useEffect, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import {
  getProductById,
  getProductReviews,
  type ApiProductReview,
} from "../../services/api/products";
import { Badge, Breadcrumb, FavoriteButton, Rating } from "../../components/common/ui";
import PriceHistory from "../../components/products/PriceHistory";
import PriceOfferComparison from "./PriceOfferComparison";
import {
  getProductCategoryGroup,
  getProductSubtype,
  getProductSubtypeSearchQuery,
} from "../../constants/productCatalog";

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
  const [loadAttempt, setLoadAttempt] = useState(0);
  const requestKey = `${productId}:${loadAttempt}`;
  const loading = loadedRequestKey !== requestKey;
  const product = productState?.requestKey === requestKey ? productState.value : null;
  const reviews = reviewState?.requestKey === requestKey ? reviewState.items : [];
  const currentProductError = productError?.requestKey === requestKey ? productError.message : null;
  const currentReviewsError = reviewsError?.requestKey === requestKey ? reviewsError.message : null;
  const reviewsLoading = reviewState?.requestKey !== requestKey && currentReviewsError === null;

  useEffect(() => {
    let cancelled = false;
    const currentRequestKey = `${productId}:${loadAttempt}`;

    getProductById(productId)
      .then((result) => {
        if (!cancelled) {
          setSelectedImage(0);
          setProductState({ requestKey: currentRequestKey, value: result });
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
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando producto...</p>
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

  const primarySpecifications = getPrimarySpecifications(product.specs);
  const specificationGroups = groupProductSpecifications(product.specs);
  const productImages = [...new Set([product.image, ...product.images].filter(Boolean))];
  const isHomeProduct = getProductCategoryGroup(product.category) === "home";
  const productSubcategory = product.subcategory.trim() || getProductSubtype(product);
  const hasDistinctSubcategory =
    productSubcategory.toLocaleLowerCase("es") !== product.category.toLocaleLowerCase("es");

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Productos",
            onClick: () => navigate({ id: "search-products", query: "" }),
          },
          {
            label: isHomeProduct ? "Hogar" : product.category,
            onClick: () =>
              isHomeProduct
                ? navigate({ id: "search-products", query: "", productGroup: "home" })
                : navigate({
                    id: "search-products",
                    query: "",
                    category: product.category,
                  }),
          },
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
              background: "linear-gradient(135deg, #343B40, #191D20)",
              border: "1px solid #343B40",
              boxShadow: "0 18px 40px rgba(0, 0, 0, 0.25)",
            }}
            className="rounded-2xl overflow-hidden h-[420px] md:h-[520px] mb-3"
          >
            <img
              src={productImages[selectedImage] ?? product.image}
              alt={product.name}
              className="w-full h-full object-contain p-4 transition-all duration-200"
              style={{
                background:
                  "radial-gradient(circle at top, rgba(209,168,90,0.12), transparent 40%)",
              }}
            />
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
                  border: `2px solid ${i === selectedImage ? "#D1A85A" : "#343B40"}`,
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
          </div>

          <div
            style={{
              background: "linear-gradient(180deg, #22282C, #191D20)",
              border: "1px solid #343B40",
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
          background: "linear-gradient(180deg, #22282C, #191D20)",
          border: "1px solid #343B40",
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
      />

      {/* Reviews */}
      <section
        style={{ background: "#191D20", border: "1px solid #343B40" }}
        className="rounded-2xl p-6 mt-6"
      >
        <div className="flex items-center justify-between gap-4 mb-5 flex-wrap">
          <h2 className="text-lg font-bold text-text">Reseñas y calificaciones</h2>
          <Rating value={product.rating} count={product.reviewCount} />
        </div>
        {reviewsLoading ? (
          <p role="status" className="text-sm text-muted">
            Cargando reseñas desde Product API...
          </p>
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
                style={{ background: "#22282C" }}
                className="rounded-xl p-4"
              >
                <div className="flex items-center justify-between gap-3 flex-wrap mb-2">
                  <div className="flex items-center gap-2">
                    <Rating value={review.rating} />
                    {review.authorVerified && <Badge variant="available">Usuario verificado</Badge>}
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
