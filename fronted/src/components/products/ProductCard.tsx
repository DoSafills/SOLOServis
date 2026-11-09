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
  const hasOfferWithoutPrice = product.offers.some(
    (offer) => offer.price === null || !Number.isFinite(offer.price),
  );
  const isInCart = cartProductIds.has(product.id);
  const discountPercent =
    bestOffer?.listPrice != null &&
    bestOffer.price !== null &&
    bestOffer.listPrice > bestOffer.price
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
      className={`group relative flex flex-col overflow-hidden rounded-2xl border bg-slate-900/70 transition-colors duration-200 ${
        isComparing
          ? "z-[1] border-cyan-300/80 ring-1 ring-cyan-300/20"
          : "border-slate-700/70 hover:border-cyan-300/60"
      }`}
    >
      {/* Image */}
      <div
        className="relative h-40 cursor-pointer overflow-hidden bg-slate-950/60"
        onClick={() => navigate({ id: "product-detail", productId: product.id })}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            className="h-full w-full object-contain p-4"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm font-medium text-slate-300">
            Sin imagen
          </div>
        )}

        <div className="absolute left-3 top-3 flex gap-1.5">
          <span
            className={`rounded-full border px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide backdrop-blur ${
              bestOffer
                ? "border-emerald-300/20 bg-slate-950/80 text-emerald-200"
                : "border-slate-700 bg-slate-950/80 text-slate-300"
            }`}
          >
            {bestOffer ? "Mejor oferta" : "Sin ofertas"}
          </span>
          {discountPercent !== null && (
            <span className="rounded-full border border-rose-300/20 bg-slate-950/85 px-2.5 py-1 text-[10px] font-semibold text-rose-200">
              -{discountPercent}%
            </span>
          )}
        </div>

        <div className="absolute right-3 top-3 rounded-full border border-slate-700/80 bg-slate-950/80 backdrop-blur">
          <FavoriteButton
            active={isFavorite}
            onClick={(e) => {
              e.stopPropagation();
              onToggleFavorite(product.id);
            }}
          />
        </div>
      </div>

      {/* Content */}
      <div className="flex flex-1 flex-col gap-3 p-4">
        <div>
          <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-cyan-300">
            {product.brand}
          </span>
          <h3
            className="mt-1 line-clamp-2 cursor-pointer text-base font-semibold leading-snug text-slate-100 transition-colors group-hover:text-white"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
          >
            {product.name}
          </h3>
        </div>

        {/* Specs preview */}
        <div className="flex min-h-6 flex-wrap gap-1.5">
          {Object.entries(product.specs)
            .slice(0, 2)
            .map(([k, v]) => (
              <span
                key={k}
                className="rounded-md border border-slate-700/80 bg-slate-950/50 px-2 py-1 text-[10px] font-medium text-slate-300"
                title={k}
              >
                {k}: {v}
              </span>
            ))}
        </div>

        <div className="[&_span]:text-slate-300">
          <Rating value={product.rating} count={product.reviewCount} />
        </div>

        {/* Price */}
        <div className="mt-auto rounded-xl border border-slate-800 bg-slate-950/40 p-3">
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-slate-400">
            Precio
          </div>
          {bestOffer ? (
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
              {discountPercent !== null && (
                <span className="text-xs font-medium text-slate-400 line-through">
                  {formatPrice(bestOffer.listPrice!)}
                </span>
              )}
              <span className="text-xl font-bold tracking-tight text-cyan-200">
                {formatPrice(bestOffer.price)}
              </span>
              <span className="text-[10px] font-semibold text-slate-400">
                en {bestOffer.storeName}
              </span>
            </div>
          ) : (
            <div className="text-sm font-semibold text-slate-400">
              {hasOfferWithoutPrice || product.offers.length === 0
                ? "Sin precio"
                : "No disponible"}
            </div>
          )}
        </div>

        {/* Actions */}
        <div className="flex gap-2">
          <button
            type="button"
            disabled={!bestOffer && !isInCart}
            aria-disabled={(!bestOffer && !isInCart) || isInCart || adding}
            aria-live="polite"
            onClick={() => void addProductToCart()}
            className={`flex-1 rounded-lg py-2.5 text-xs font-semibold text-white transition ${
              isInCart
                ? "bg-emerald-900/70 text-emerald-200"
                : adding
                  ? "bg-emerald-900/70 text-emerald-200"
                  : "bg-gradient-to-r from-cyan-600 to-violet-600 hover:brightness-110"
            } disabled:cursor-not-allowed disabled:from-slate-700 disabled:to-slate-700 disabled:text-slate-400 disabled:shadow-none`}
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
            className="flex-1 rounded-lg border border-slate-700 bg-slate-800/50 py-2.5 text-xs font-semibold text-slate-200 transition-colors hover:border-slate-500 hover:bg-slate-800"
          >
            Ver producto
          </button>
        </div>
        <button
          type="button"
          onClick={() => onToggleCompare(product.id, product.category)}
          aria-pressed={isComparing}
          className={`w-full rounded-lg border py-2 text-xs font-semibold transition-colors ${
            isComparing
              ? "border-cyan-300/50 bg-cyan-300/10 text-cyan-100"
              : "border-slate-700 bg-transparent text-slate-300 hover:border-slate-500 hover:text-white"
          }`}
        >
          {isComparing ? "✓ Comparando" : "Comparar"}
        </button>
      </div>
    </div>
  );
}
