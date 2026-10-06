import { useEffect, useRef, useState } from "react";
import type { Product, Page, StoreOffer } from "../../types";
import { Rating, FavoriteButton } from "../common/ui";
import { formatPrice, getMinOffer } from "../../Services/utils/productUtils";

interface Props {
  product: Product;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  onAddToCart: (product: Product, offer: StoreOffer) => void;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string, category: string) => void;
}

export default function ProductCard({
  product,
  navigate,
  isFavorite,
  isComparing,
  onAddToCart,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [justAdded, setJustAdded] = useState(false);
  const addedTimeout = useRef<ReturnType<typeof setTimeout> | null>(null);
  const bestOffer = getMinOffer(product);
  const discountPercent =
    bestOffer?.listPrice && bestOffer.listPrice > bestOffer.price
      ? Math.round(((bestOffer.listPrice - bestOffer.price) / bestOffer.listPrice) * 100)
      : null;

  useEffect(
    () => () => {
      if (addedTimeout.current) clearTimeout(addedTimeout.current);
    },
    [],
  );

  const handleAddToCart = () => {
    if (!bestOffer) return;
    onAddToCart(product, bestOffer);
    setJustAdded(true);
    if (addedTimeout.current) clearTimeout(addedTimeout.current);
    addedTimeout.current = setTimeout(() => setJustAdded(false), 1500);
  };

  return (
    <div
      className={`group relative flex flex-col overflow-hidden rounded-2xl border bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-950 shadow-lg shadow-slate-950/30 transition duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-violet-950/40 ${
        isComparing
          ? "z-[1] scale-[1.02] border-cyan-300 ring-2 ring-cyan-400/30"
          : "border-indigo-300/20 hover:border-violet-300/60"
      }`}
    >
      {/* Image */}
      <div
        className="relative h-36 cursor-pointer overflow-hidden bg-gradient-to-br from-violet-950 via-slate-900 to-cyan-950"
        onClick={() => navigate({ id: "product-detail", productId: product.id })}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm font-medium text-slate-300">
            Sin imagen
          </div>
        )}

        <div className="absolute left-2.5 top-2.5 flex gap-1.5">
          <span
            className={`rounded-full border px-2.5 py-1 text-[10px] font-extrabold uppercase tracking-wide shadow-lg backdrop-blur ${
              bestOffer
                ? "border-emerald-200/30 bg-emerald-400/20 text-emerald-100"
                : "border-slate-300/20 bg-slate-950/70 text-slate-200"
            }`}
          >
            {bestOffer ? "Mejor oferta" : "Sin ofertas"}
          </span>
          {discountPercent !== null && (
            <span className="rounded-full border border-rose-200/30 bg-rose-500/90 px-2.5 py-1 text-[10px] font-extrabold text-white shadow-lg">
              -{discountPercent}%
            </span>
          )}
        </div>

        <div className="absolute right-2.5 top-2.5 rounded-full border border-white/15 bg-slate-950/75 shadow-lg backdrop-blur">
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
      <div className="flex flex-1 flex-col gap-1.5 p-3.5">
        <div>
          <span className="text-[11px] font-bold uppercase tracking-[0.14em] text-violet-300">
            {product.brand}
          </span>
          <h3
            className="mt-0.5 line-clamp-2 cursor-pointer text-sm font-bold leading-snug text-slate-50 transition-colors group-hover:text-cyan-100"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
          >
            {product.name}
          </h3>
        </div>

        {/* Specs preview */}
        <div className="flex flex-wrap gap-1">
          {Object.entries(product.specs)
            .slice(0, 2)
            .map(([k, v]) => (
              <span
                key={k}
                className="rounded-lg border border-cyan-200/15 bg-cyan-300/10 px-2 py-1 text-[10px] font-semibold text-cyan-100"
                title={k}
              >
                {k}: {v}
              </span>
            ))}
        </div>

        <div className="rounded-lg border border-amber-200/10 bg-amber-300/[0.06] px-2 py-1.5 [&_span]:text-slate-200">
          <Rating value={product.rating} count={product.reviewCount} />
        </div>

        {/* Price */}
        <div className="mt-1 border-t border-indigo-200/15 pt-2">
          <div className="mb-0.5 text-[11px] font-bold uppercase tracking-wide text-slate-300">
            Precio
          </div>
          {bestOffer ? (
            <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
              {discountPercent !== null && (
                <span className="text-xs font-medium text-slate-400 line-through">
                  {formatPrice(bestOffer.listPrice!)}
                </span>
              )}
              <span className="text-lg font-extrabold text-emerald-300">
                {formatPrice(bestOffer.price)}
              </span>
              <span className="text-[10px] font-semibold text-slate-400">
                en {bestOffer.storeName}
              </span>
            </div>
          ) : (
            <div className="text-sm font-semibold text-slate-400">No disponible</div>
          )}
        </div>

        {/* Actions */}
        <div className="mt-1 flex gap-2">
          <button
            type="button"
            disabled={!bestOffer}
            onClick={handleAddToCart}
            className={`flex-1 rounded-lg py-2 text-xs font-bold shadow-md transition hover:brightness-110 disabled:cursor-not-allowed disabled:from-slate-700 disabled:to-slate-700 disabled:text-slate-400 disabled:shadow-none ${
              justAdded
                ? "bg-gradient-to-r from-emerald-800 to-emerald-700 text-emerald-100 shadow-emerald-950/30"
                : "bg-gradient-to-r from-violet-600 to-fuchsia-500 text-white shadow-violet-950/30"
            }`}
          >
            {justAdded ? "✓ Agregado" : "Agregar al carrito"}
          </button>
          <button
            type="button"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
            className="flex-1 rounded-lg border border-cyan-200/25 bg-cyan-400/10 py-2 text-xs font-bold text-cyan-100 transition hover:border-cyan-200/50 hover:bg-cyan-400/20"
          >
            Ver producto
          </button>
        </div>
        <button
          type="button"
          onClick={() => onToggleCompare(product.id, product.category)}
          aria-pressed={isComparing}
          className={`w-full rounded-lg border py-2 text-sm font-extrabold transition ${
            isComparing
              ? "border-cyan-200/50 bg-gradient-to-r from-cyan-500/25 to-violet-500/25 text-cyan-100 shadow-[0_0_18px_rgba(34,211,238,0.16)]"
              : "border-violet-200/25 bg-violet-400/10 text-violet-100 hover:border-violet-200/50 hover:bg-violet-400/20"
          }`}
        >
          {isComparing ? "✓ Comparando" : "Comparar"}
        </button>
      </div>
    </div>
  );
}
