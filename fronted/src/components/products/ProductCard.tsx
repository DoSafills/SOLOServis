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
      className={`group relative isolate flex min-h-[24rem] flex-col overflow-hidden rounded-[1.75rem] border bg-surface shadow-lg shadow-black/20 transition-all duration-300 hover:-translate-y-1 hover:shadow-2xl hover:shadow-black/30 sm:min-h-[22rem] ${
        isComparing ? "z-[1] border-prime ring-1 ring-prime/20" : "border-bd hover:border-prime/50"
      }`}
    >
      <div
        className="absolute inset-y-0 left-0 w-[58%] cursor-pointer bg-slate-950/60"
        onClick={() => navigate({ id: "product-detail", productId: product.id })}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            className="h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-[1.03]"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm font-medium text-muted">
            Sin imagen
          </div>
        )}
        <div
          aria-hidden="true"
          className="pointer-events-none absolute inset-0 bg-gradient-to-r from-transparent via-transparent to-slate-950/25"
        />
      </div>

      <div
        aria-hidden="true"
        className="absolute inset-y-0 right-0 w-[66%]"
        style={{
          background: "linear-gradient(135deg, #343B40 0%, #22282C 42%, #191D20 100%)",
          clipPath: "polygon(18% 0, 100% 0, 100% 100%, 0 100%)",
        }}
      />

      <div className="absolute right-3 top-3 z-20 rounded-full border border-bd bg-surface/90 backdrop-blur-md">
        <FavoriteButton
          active={isFavorite}
          onClick={(e) => {
            e.stopPropagation();
            onToggleFavorite(product.id);
          }}
        />
      </div>

      <div className="relative z-10 ml-[40%] flex w-[60%] flex-1 flex-col justify-center gap-2 p-3 pl-10 sm:gap-3 sm:p-4 sm:pl-12">
        <div>
          <span className="text-[10px] font-bold uppercase tracking-[0.16em] text-prime">
            {product.brand}
          </span>
          <h3
            className="mt-1 line-clamp-2 cursor-pointer text-base font-bold leading-snug text-text transition-colors group-hover:text-prime"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
          >
            {product.name}
          </h3>
        </div>

        <div className="[&_span]:text-muted">
          <Rating value={product.rating} count={product.reviewCount} />
        </div>

        <div className="border-t border-bd pt-2">
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-[0.14em] text-muted">
            Precio
          </div>
          {bestOffer ? (
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
              {discountPercent !== null && (
                <span className="text-xs font-medium text-muted line-through">
                  {formatPrice(bestOffer.listPrice!)}
                </span>
              )}
              <span className="text-xl font-bold tracking-tight text-prime">
                {formatPrice(bestOffer.price)}
              </span>
              {discountPercent !== null && (
                <span className="rounded-full border border-prime/40 bg-prime-muted px-2 py-0.5 text-[10px] font-bold text-prime">
                  -{discountPercent}%
                </span>
              )}
            </div>
          ) : (
            <div className="text-sm font-semibold text-muted">No disponible</div>
          )}
        </div>

        {/* Actions */}
        <div className="flex flex-col gap-2">
          <button
            type="button"
            disabled={!bestOffer || isInCart || adding}
            aria-live="polite"
            onClick={() => void addProductToCart()}
            className={`w-full rounded-xl border px-2 py-2.5 text-[11px] font-semibold transition-colors ${
              isInCart
                ? "border-emerald-300/20 bg-emerald-900/70 text-emerald-200"
                : adding
                  ? "border-emerald-300/20 bg-emerald-900/70 text-emerald-200"
                  : "border-bd bg-surface-2 text-text hover:border-prime/40 hover:bg-prime-muted"
            } disabled:cursor-not-allowed disabled:border-slate-700 disabled:bg-slate-800/70 disabled:text-slate-400`}
          >
            {isInCart
              ? "Agregado en la cesta"
              : adding
                ? "Agregando a la cesta..."
                : "Agregar al carrito"}
          </button>
          <button
            type="button"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
            className="flex w-full items-center justify-center gap-1.5 rounded-xl bg-prime px-2 py-2.5 text-[11px] font-semibold text-[#181714] shadow-lg shadow-black/20 transition-all hover:-translate-y-0.5 hover:bg-prime-dark hover:shadow-xl active:translate-y-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime"
          >
            Ver producto
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
          onClick={() => onToggleCompare(product.id, product.category)}
          aria-pressed={isComparing}
          className={`w-full rounded-xl border py-2 text-[11px] font-semibold transition-colors ${
            isComparing
              ? "border-prime/60 bg-prime-muted text-prime"
              : "border-bd bg-transparent text-muted hover:border-prime/50 hover:text-text"
          }`}
        >
          {isComparing ? "✓ Comparando" : "Comparar"}
        </button>
      </div>
    </div>
  );
}
