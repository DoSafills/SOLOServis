import { useRef, useState } from "react";
import type { Product, Page, StoreOffer } from "../../types";
import { Rating, FavoriteButton } from "../common/ui";
import { formatPrice, getMinOffer } from "../../services/utils/productUtils";

interface Props {
  product: Product;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  cartProductIds: ReadonlySet<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => Promise<boolean>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string, category: string) => void;
  compact?: boolean;
}

export default function ProductCard({
  product,
  navigate,
  isFavorite,
  isComparing,
  cartProductIds,
  onAddToCart,
  onToggleFavorite,
  onToggleCompare,
  compact = false,
}: Props) {
  const [adding, setAdding] = useState(false);
  const addingRef = useRef(false);
  const bestOffer = getMinOffer(product);
  const isInCart = cartProductIds.has(product.id);
  const discountPercent =
    bestOffer?.listPrice && bestOffer.listPrice > bestOffer.price
      ? Math.round(((bestOffer.listPrice - bestOffer.price) / bestOffer.listPrice) * 100)
      : null;

  const addProductToCart = async () => {
    if (!bestOffer || isInCart || addingRef.current) return;
    addingRef.current = true;
    setAdding(true);
    try {
      await onAddToCart(product, bestOffer);
    } finally {
      addingRef.current = false;
      setAdding(false);
    }
  };

  return (
    <div
      className={`group relative isolate flex ${
        compact ? "min-h-[11rem] sm:min-h-[10.5rem]" : "min-h-[14.4rem] sm:min-h-[13.2rem]"
      } overflow-hidden rounded-[1.75rem] border bg-surface shadow-lg shadow-black/20 transition-all duration-300 hover:-translate-y-1 hover:shadow-2xl hover:shadow-black/30 ${
        isComparing ? "z-[1] border-prime ring-1 ring-prime/20" : "border-bd hover:border-prime/50"
      }`}
    >
      <div
        className="theme-media-panel relative w-[58%] shrink-0 cursor-pointer bg-slate-950/60"
        style={{ clipPath: "polygon(0 0, 100% 0, 88% 100%, 0 100%)" }}
        onClick={() => navigate({ id: "product-detail", productId: product.id })}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            className="absolute inset-0 h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-[1.03]"
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center text-sm font-medium text-muted">
            Sin imagen
          </div>
        )}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-gradient-to-r from-transparent to-slate-950/20"
        />
        <div className="absolute right-1 top-1 z-20 rounded-full border border-bd bg-surface/90 backdrop-blur-md">
          <FavoriteButton
            active={isFavorite}
            onClick={(e) => {
              e.stopPropagation();
              onToggleFavorite(product.id);
            }}
          />
        </div>
      </div>

      <div
        className={`flex min-w-0 flex-1 flex-col justify-center bg-surface ${
          compact ? "gap-0.5 px-2 py-1" : "gap-1 px-2 py-1.5"
        }`}
      >
        <div>
          <span className="block truncate text-[8px] font-bold uppercase tracking-[0.12em] text-prime sm:text-[9px]">
            {product.brand}
          </span>
          <h3
            className="mt-0.5 line-clamp-2 cursor-pointer text-[11px] font-bold leading-tight text-text transition-colors group-hover:text-prime sm:text-xs"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
          >
            {product.name}
          </h3>
        </div>

        <div className="scale-[0.85] origin-left [&_span]:text-muted">
          <Rating value={product.rating} count={product.reviewCount} />
        </div>

        <div className="border-t border-bd pt-1">
          {bestOffer ? (
            <div className="flex flex-wrap items-baseline gap-x-1 gap-y-0">
              {discountPercent !== null && (
                <span className="text-[8px] font-medium text-muted line-through">
                  {formatPrice(bestOffer.listPrice!)}
                </span>
              )}
              <span className="text-xs font-bold tracking-tight text-prime sm:text-sm">
                {formatPrice(bestOffer.price)}
              </span>
              {discountPercent !== null && (
                <span
                  className={`rounded-full border px-1.5 py-0.5 text-[8px] font-bold shadow-sm sm:text-[9px] ${
                    discountPercent >= 30
                      ? "border-rose-300/70 bg-rose-500/25 text-rose-200 shadow-rose-500/20"
                      : discountPercent >= 15
                        ? "border-amber-300/70 bg-amber-400/20 text-amber-100 shadow-amber-400/20"
                        : "border-emerald-300/70 bg-emerald-500/20 text-emerald-200 shadow-emerald-500/20"
                  }`}
                >
                  -{discountPercent}%
                </span>
              )}
            </div>
          ) : (
            <div className="text-[9px] font-semibold text-muted">No disponible</div>
          )}
        </div>

        <div className={`flex flex-col ${compact ? "gap-1" : "gap-1.5"}`}>
          <div className={`grid grid-cols-2 ${compact ? "gap-1" : "gap-1.5"}`}>
            <button
              type="button"
              disabled={!bestOffer || isInCart || adding}
              aria-live="polite"
              aria-label={
                isInCart
                  ? "Producto agregado en la cesta"
                  : adding
                    ? "Agregando producto a la cesta"
                    : "Agregar producto al carrito"
              }
              title={
                isInCart
                  ? "Agregado en la cesta"
                  : adding
                    ? "Agregando a la cesta..."
                    : "Agregar al carrito"
              }
              onClick={() => void addProductToCart()}
              className={`truncate rounded-lg border px-1.5 ${
                compact ? "py-1.5" : "py-2"
              } text-[9px] font-semibold transition-colors sm:text-[10px] ${
                isInCart
                  ? "border-emerald-300/20 bg-emerald-900/70 text-emerald-200"
                  : adding
                    ? "border-emerald-300/20 bg-emerald-900/70 text-emerald-200"
                    : "border-bd bg-surface-2 text-text hover:border-prime/40 hover:bg-prime-muted"
              } disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-800/70 disabled:text-slate-400`}
            >
              <span className="sm:hidden">{isInCart ? "En cesta" : adding ? "..." : "Cesta"}</span>
              <span className="hidden sm:inline">
                {isInCart ? "En cesta" : adding ? "Añadiendo" : "Agregar"}
              </span>
            </button>
            <button
              type="button"
              title="Ver producto"
              aria-label={`Ver ${product.name}`}
              onClick={() => navigate({ id: "product-detail", productId: product.id })}
              className={`flex min-w-0 items-center justify-center gap-0.5 truncate rounded-lg bg-prime px-1.5 ${
                compact ? "py-1.5" : "py-2"
              } text-[9px] font-semibold text-[#181714] shadow-lg shadow-black/20 transition-all hover:-translate-y-0.5 hover:bg-prime-dark hover:shadow-xl active:translate-y-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime sm:text-[10px]`}
            >
              Ver
              <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" className="h-4 w-4">
                <path
                  d="M4.5 10h11m-4.5-4.5L15.5 10 11 14.5"
                  stroke="currentColor"
                  strokeWidth="1.7"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </button>
          </div>
          <button
            type="button"
            title={isComparing ? "Quitar de comparación" : "Comparar"}
            aria-label={isComparing ? "Quitar producto de comparación" : "Comparar producto"}
            onClick={() => onToggleCompare(product.id, product.category)}
            aria-pressed={isComparing}
            className={`w-full truncate rounded-lg border px-2 ${
              compact ? "py-1.5" : "py-2"
            } text-[9px] font-semibold transition-colors sm:text-[10px] ${
              isComparing
                ? "border-prime/60 bg-prime-muted text-prime"
                : "border-bd bg-transparent text-muted hover:border-prime/50 hover:text-text"
            }`}
          >
            {isComparing ? "En comparación" : "Comparar"}
          </button>
        </div>
      </div>
    </div>
  );
}
