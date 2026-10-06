import { useEffect, useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import { getProductById } from "../../services/api/api";
import { Breadcrumb, FavoriteButton, Rating } from "../../components/common/ui";
import PriceHistory from "../../components/products/PriceHistory";
import PriceOfferComparison from "./PriceOfferComparison";

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
  const [product, setProduct] = useState<Product | null>(null);
  const [loading, setLoading] = useState(true);
  const [selectedImage, setSelectedImage] = useState(0);

  useEffect(() => {
    let cancelled = false;

    getProductById(productId)
      .then((result) => {
        if (!cancelled) {
          setSelectedImage(0);
          setProduct(result);
        }
      })
      .catch(() => {
        if (!cancelled) {
          setProduct(null);
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [productId]);

  if (loading)
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando producto...</p>
      </div>
    );

  if (!product)
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Producto no encontrado.</p>
      </div>
    );

  const primarySpecifications = getPrimarySpecifications(product.specs);
  const specificationGroups = groupProductSpecifications(product.specs);

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
            label: product.category,
            onClick: () => navigate({ id: "search-products", query: product.category }),
          },
          { label: product.name },
        ]}
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        <div>
          <div
            style={{
              background: "linear-gradient(135deg, rgba(15, 23, 42, 0.98), rgba(30, 41, 59, 0.86))",
              border: "1px solid rgba(148, 163, 184, 0.25)",
              boxShadow: "0 18px 40px rgba(15, 23, 42, 0.12)",
            }}
            className="rounded-2xl overflow-hidden h-[420px] md:h-[520px] mb-3"
          >
            <img
              src={product.images[selectedImage] ?? product.image}
              alt={product.name}
              className="w-full h-full object-contain p-4 transition-all duration-200"
              style={{
                background:
                  "radial-gradient(circle at top, rgba(139,92,246,0.14), transparent 40%)",
              }}
            />
          </div>

          <div className="flex gap-2 overflow-x-auto pb-1">
            {product.images.map((img, i) => (
              <button
                key={i}
                onClick={() => setSelectedImage(i)}
                style={{
                  border: `2px solid ${i === selectedImage ? "#8b5cf6" : "rgba(148,163,184,0.25)"}`,
                  boxShadow: i === selectedImage ? "0 0 0 2px rgba(139,92,246,0.12)" : "none",
                }}
                className="w-16 h-16 rounded-xl overflow-hidden transition-all shrink-0 bg-slate-900"
              >
                <img src={img} alt="" className="w-full h-full object-contain p-1" />
              </button>
            ))}
          </div>
        </div>

        <div className="flex flex-col gap-4">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="inline-flex items-center gap-2 rounded-full border border-violet-400/30 bg-violet-500/10 px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-violet-200">
                {product.brand} · {product.category}
              </span>
              <FavoriteButton active={isFavorite} onClick={() => onToggleFavorite(product.id)} />
            </div>

            <h1 className="text-2xl md:text-3xl font-black text-slate-900 leading-tight tracking-[-0.04em]">
              {product.name}
            </h1>

            <div className="mt-3">
              <Rating value={product.rating} count={product.reviewCount} />
            </div>
          </div>

          <p className="text-sm text-slate-600 leading-relaxed">{product.description}</p>

          <div
            style={{
              background: "linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(15, 23, 42, 0.88))",
              border: "1px solid rgba(148, 163, 184, 0.24)",
              boxShadow: "0 14px 30px rgba(15, 23, 42, 0.08)",
            }}
            className="rounded-xl p-4"
          >
            <h3 className="text-sm font-semibold text-white mb-3">Características principales</h3>

            {primarySpecifications.length > 0 ? (
              <dl className="grid grid-cols-2 xl:grid-cols-4 gap-x-4 gap-y-3">
                {primarySpecifications.map(([name, value]) => (
                  <div
                    key={name}
                    className="min-w-0 rounded-xl border border-violet-400/20 bg-violet-500/5 pl-3 py-2"
                  >
                    <dt className="text-[10px] uppercase tracking-[0.14em] text-slate-400">
                      {name}
                    </dt>
                    <dd className="mt-2 text-sm font-semibold text-slate-100 break-words">
                      {value}
                    </dd>
                  </div>
                ))}
              </dl>
            ) : (
              <p className="text-sm text-slate-300">No hay características informadas.</p>
            )}
          </div>

          <div className="flex gap-3">
            <button
              onClick={() => onToggleCompare(product.id, product.category)}
              style={
                isComparing
                  ? {
                      background: "linear-gradient(135deg, #ff7a59 0%, #ff5f7b 32%, #8b5cf6 100%)",
                      color: "#fff",
                    }
                  : {
                      background: "rgba(15, 23, 42, 0.8)",
                      border: "1px solid rgba(148, 163, 184, 0.25)",
                      color: "#e2e8f0",
                    }
              }
              className="flex-1 py-3 rounded-2xl text-sm font-semibold transition-all hover:opacity-95"
            >
              {isComparing ? "✓ Agregado al comparador" : "Agregar al comparador"}
            </button>
          </div>
        </div>
      </div>

      <PriceOfferComparison
        product={product}
        cartProductIds={cartProductIds}
        onAddToCart={onAddToCart}
      />

      <PriceHistory
        history={product.priceHistory}
        offerHistory={product.offerPriceHistory}
        currentOfferPrice={product.offerPrice}
      />

      <section
        style={{
          background: "linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(15, 23, 42, 0.88))",
          border: "1px solid rgba(148, 163, 184, 0.24)",
        }}
        className="rounded-xl p-5 sm:p-6 mt-6"
      >
        <h2 className="text-lg font-semibold text-white">Especificaciones técnicas</h2>

        {specificationGroups.length > 0 ? (
          <div className="mt-4 space-y-5">
            {specificationGroups.map((group) => (
              <section key={group.title}>
                <h3 className="text-xs font-semibold uppercase tracking-[0.16em] text-violet-200">
                  {group.title}
                </h3>
                <dl className="mt-2 grid grid-cols-1 md:grid-cols-2 gap-x-8">
                  {group.items.map(([name, value]) => (
                    <div
                      key={name}
                      className="flex items-start justify-between gap-4 border-b border-slate-700/80 py-3"
                    >
                      <dt className="min-w-0 text-sm text-slate-300">{name}</dt>
                      <dd className="max-w-[60%] text-right text-sm font-semibold text-white break-words">
                        {value}
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            ))}
          </div>
        ) : (
          <p className="mt-4 text-sm text-slate-300">
            No hay especificaciones técnicas informadas.
          </p>
        )}
      </section>
    </div>
  );
}
