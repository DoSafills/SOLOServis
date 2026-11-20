import { useRef, useState } from "react";
import type { Product, Page, StoreOffer } from "../../types";
import { Rating, FavoriteButton } from "../common/ui";
import {
  formatExactUpdateDate,
  formatPrice,
  formatPriceUpdate,
  detectPriceAnomalies,
  getMinOffer,
} from "../../services/utils/productUtils";

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

/**
 * Tarjeta de producto vertical y compacta: foto 4:3 sobre panel claro, marca,
 * nombre, mejor oferta, precio destacado y acciones (cesta, ver, comparar).
 * Reserva las mismas líneas en todas las tarjetas (rating, tienda, precio de
 * lista y verificación) para que la grilla quede pareja. Recibe el producto,
 * los estados de favorito/comparación/cesta y los callbacks de acciones;
 * `compact` reduce tipografía y padding para el catálogo de una tienda.
 */
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
  const bestOfferUpdateDate = formatExactUpdateDate(bestOffer?.lastUpdated);
  const anomalousOfferIds = detectPriceAnomalies(product.offers, []).anomalousOfferIds;
  const bestOfferIsAnomalous = bestOffer !== undefined && anomalousOfferIds.has(bestOffer.offerId);
  const isInCart = cartProductIds.has(product.id);
  const hasRating = product.rating > 0 || product.reviewCount > 0;
  const discountPercent =
    bestOffer?.price !== null &&
    bestOffer?.price !== undefined &&
    bestOffer.listPrice !== null &&
    bestOffer.listPrice !== undefined &&
    bestOffer.listPrice > bestOffer.price
      ? Math.round(((bestOffer.listPrice - bestOffer.price) / bestOffer.listPrice) * 100)
      : null;

  /**
   * Agrega la mejor oferta a la cesta evitando dobles clics con `addingRef`.
   * Devuelve una promesa que se resuelve cuando el callback del padre termina.
   */
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
    <article
      className={`group relative flex h-full flex-col overflow-hidden rounded-2xl border bg-surface shadow-lg shadow-black/10 transition-all duration-300 hover:-translate-y-1 hover:shadow-2xl hover:shadow-black/25 ${
        isComparing ? "border-prime ring-1 ring-prime/30" : "border-bd hover:border-prime/50"
      }`}
    >
      <div
        className="relative aspect-[4/3] w-full shrink-0 cursor-pointer overflow-hidden bg-white"
        onClick={() => navigate({ id: "product-detail", productId: product.id })}
      >
        {product.image ? (
          <img
            src={product.image}
            alt={product.name}
            loading="lazy"
            className={`absolute inset-0 h-full w-full object-contain transition-transform duration-500 group-hover:scale-[1.04] ${
              compact ? "p-2.5" : "p-3"
            }`}
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center bg-surface-2 text-[11px] font-medium text-muted">
            Sin imagen
          </div>
        )}
        <div className="absolute right-1 top-1 z-20 rounded-full bg-white/95 shadow-sm ring-1 ring-black/5">
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
        className={`flex min-w-0 flex-1 flex-col ${
          compact ? "gap-1 px-2 pb-2 pt-1.5" : "gap-1.5 px-2.5 pb-2.5 pt-2"
        }`}
      >
        <div>
          <span className="block truncate text-[9px] font-bold uppercase tracking-[0.12em] text-muted">
            {product.brand}
          </span>
          <h3
            className={`mt-0.5 line-clamp-2 min-h-[2.6em] cursor-pointer font-semibold leading-snug text-text transition-colors group-hover:text-prime ${
              compact ? "text-[11px]" : "text-xs sm:text-[13px]"
            }`}
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
          >
            {product.name}
          </h3>
          {!compact && (
            <div className="mt-0.5 flex min-h-[1.15rem] items-center">
              {hasRating ? (
                <div className="origin-left scale-[0.8] [&_span]:text-muted">
                  <Rating value={product.rating} count={product.reviewCount} />
                </div>
              ) : (
                <span className="text-[10px] text-muted">Sin valoraciones</span>
              )}
            </div>
          )}
          <p className="min-h-[1.1rem] truncate text-[10px] text-muted">
            {bestOffer?.storeName && (
              <>
                Por <span className="font-semibold text-text">{bestOffer.storeName}</span>
              </>
            )}
          </p>
        </div>

        <div className="mt-auto border-t border-bd pt-1.5">
          {bestOffer ? (
            <div className="flex flex-col">
              <div className="flex flex-wrap items-baseline gap-x-1.5">
                <span
                  className={`font-extrabold tracking-tight text-rose-500 ${
                    compact ? "text-sm" : "text-base sm:text-lg"
                  }`}
                >
                  {formatPrice(bestOffer.price)}
                </span>
                {discountPercent !== null && (
                  <span className="rounded bg-rose-600 px-1 py-px text-[9px] font-bold leading-tight text-white">
                    -{discountPercent}%
                  </span>
                )}
              </div>
              <span className="min-h-[0.95rem] text-[10px] font-medium text-muted line-through">
                {discountPercent !== null && formatPrice(bestOffer.listPrice!)}
              </span>
              {bestOfferIsAnomalous && (
                <span
                  className="mt-0.5 w-fit rounded border border-amber-400 bg-amber-950 px-1 py-px text-[8px] font-semibold text-amber-100"
                  title="Menos de la mitad de la mediana de otras ofertas"
                >
                  Precio inusual
                </span>
              )}
              {!compact && (
                <span
                  className="min-h-[0.9rem] truncate text-[9px] text-muted"
                  title={bestOfferUpdateDate}
                >
                  {bestOfferUpdateDate
                    ? `Verificado: ${formatPriceUpdate(bestOffer.lastUpdated)}`
                    : ""}
                </span>
              )}
            </div>
          ) : (
            <div className="min-h-[2.9rem] text-[11px] font-semibold text-muted">No disponible</div>
          )}
        </div>

        <div className="flex flex-col gap-1">
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
            className={`w-full truncate rounded-lg px-2 font-bold shadow-md shadow-black/15 transition-all hover:-translate-y-0.5 hover:shadow-lg active:translate-y-0 ${
              compact ? "py-1 text-[10px]" : "py-1.5 text-[11px] sm:text-xs"
            } ${
              isInCart || adding
                ? "cursor-default bg-emerald-700 text-emerald-50"
                : "bg-prime text-[#181714] hover:bg-prime-dark"
            } disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-300 disabled:shadow-none`}
          >
            {isInCart
              ? "En cesta"
              : adding
                ? "Añadiendo..."
                : compact
                  ? "Agregar"
                  : "Agregar al carro"}
          </button>
          <div className="grid grid-cols-2 gap-1">
            <button
              type="button"
              title="Ver producto"
              aria-label={`Ver ${product.name}`}
              onClick={() => navigate({ id: "product-detail", productId: product.id })}
              className={`truncate rounded-lg border border-bd bg-surface-2 px-1.5 font-semibold text-text transition-colors hover:border-prime/50 hover:bg-prime-muted ${
                compact ? "py-1 text-[9px]" : "py-1.5 text-[10px] sm:text-[11px]"
              }`}
            >
              Ver
            </button>
            <button
              type="button"
              title={isComparing ? "Quitar de comparación" : "Comparar"}
              aria-label={isComparing ? "Quitar producto de comparación" : "Comparar producto"}
              onClick={() => onToggleCompare(product.id, product.category)}
              aria-pressed={isComparing}
              className={`truncate rounded-lg border px-1.5 font-semibold transition-colors ${
                compact ? "py-1 text-[9px]" : "py-1.5 text-[10px] sm:text-[11px]"
              } ${
                isComparing
                  ? "border-prime/60 bg-prime-muted text-prime"
                  : "border-bd bg-transparent text-muted hover:border-prime/50 hover:text-text"
              }`}
            >
              {isComparing ? "Comparando" : "Comparar"}
            </button>
          </div>
        </div>
      </div>
    </article>
  );
}
