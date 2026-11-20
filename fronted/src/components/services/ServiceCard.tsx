import { useRef, useState } from "react";
import type { Service, Page } from "../../types";
import { formatPrice, getBillingPeriodText } from "../../services/utils/productUtils";
import { Rating, FavoriteButton } from "../common/ui";

interface Props {
  service: Service;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  isInCart: boolean;
  onAddToCart: (service: Service) => Promise<boolean>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string) => void;
}

/**
 * Tarjeta de servicio con la misma estructura que `ProductCard`: foto 4:3
 * sobre panel claro, proveedor, nombre, categoría, valoración, precio mensual
 * y acciones (cesta, ver, comparar). Reserva las mismas líneas en todas las
 * tarjetas para que la grilla quede pareja. Recibe el servicio y los estados
 * de favorito/comparación/cesta con sus callbacks.
 */
export default function ServiceCard({
  service,
  navigate,
  isFavorite,
  isComparing,
  isInCart,
  onAddToCart,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [adding, setAdding] = useState(false);
  const addingRef = useRef(false);
  const hasRating = service.rating > 0 || service.reviewCount > 0;

  /**
   * Agrega el servicio a la cesta evitando dobles clics con `addingRef`.
   * Devuelve una promesa que se resuelve cuando el callback del padre termina.
   */
  const addServiceToCart = async () => {
    if (addingRef.current || isInCart) return;
    addingRef.current = true;
    setAdding(true);
    try {
      await onAddToCart(service);
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
        onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
      >
        {service.image ? (
          <img
            src={service.image}
            alt={service.name}
            loading="lazy"
            className="absolute inset-0 h-full w-full object-contain p-3 transition-transform duration-500 group-hover:scale-[1.04]"
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
              onToggleFavorite(service.id, "service");
            }}
          />
        </div>
      </div>

      <div className="flex min-w-0 flex-1 flex-col gap-1.5 px-2.5 pb-2.5 pt-2">
        <div>
          <span className="block truncate text-[9px] font-bold uppercase tracking-[0.12em] text-muted">
            {service.provider}
          </span>
          <h3
            className="mt-0.5 line-clamp-2 min-h-[2.6em] cursor-pointer text-xs font-semibold leading-snug text-text transition-colors group-hover:text-prime sm:text-[13px]"
            onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
          >
            {service.name}
          </h3>
          <div className="mt-0.5 flex min-h-[1.15rem] items-center">
            {hasRating ? (
              <div className="origin-left scale-[0.8] [&_span]:text-muted">
                <Rating value={service.rating} count={service.reviewCount} />
              </div>
            ) : (
              <span className="text-[10px] text-muted">Sin valoraciones</span>
            )}
          </div>
          <p className="min-h-[1.1rem] truncate text-[10px] text-muted">{service.category}</p>
        </div>

        <div className="mt-auto border-t border-bd pt-1.5">
          <div className="flex flex-col">
            <div className="flex flex-wrap items-baseline gap-x-1">
              <span className="text-base font-extrabold tracking-tight text-prime sm:text-lg">
                {formatPrice(service.monthlyPrice)}
              </span>
              <span className="text-[9px] font-semibold text-muted">
                {getBillingPeriodText(service.billingPeriod)}
              </span>
            </div>
            <span className="min-h-[0.95rem] text-[10px] text-muted">
              {service.installationCost !== null && service.installationCost > 0
                ? `Instalación ${formatPrice(service.installationCost)}`
                : ""}
            </span>
            <span className="min-h-[0.9rem] truncate text-[9px] text-muted">
              {service.contractMonths ? `${service.contractMonths} meses de contrato` : ""}
            </span>
          </div>
        </div>

        <div className="flex flex-col gap-1">
          <button
            type="button"
            onClick={() => void addServiceToCart()}
            disabled={isInCart || adding}
            aria-live="polite"
            aria-label={
              isInCart
                ? "Servicio agregado en la cesta"
                : adding
                  ? "Agregando servicio a la cesta"
                  : "Agregar servicio al carrito"
            }
            title={
              isInCart
                ? "Agregado en la cesta"
                : adding
                  ? "Agregando a la cesta..."
                  : "Agregar al carrito"
            }
            className={`w-full truncate rounded-lg px-2 py-1.5 text-[11px] font-bold shadow-md shadow-black/15 transition-all hover:-translate-y-0.5 hover:shadow-lg active:translate-y-0 sm:text-xs ${
              isInCart || adding
                ? "cursor-default bg-emerald-700 text-emerald-50"
                : "bg-prime text-[#181714] hover:bg-prime-dark"
            } disabled:cursor-not-allowed disabled:bg-slate-700 disabled:text-slate-300 disabled:shadow-none`}
          >
            {isInCart ? "En cesta" : adding ? "Añadiendo..." : "Agregar al carro"}
          </button>
          <div className="grid grid-cols-2 gap-1">
            <button
              type="button"
              title="Ver servicio"
              aria-label={`Ver servicio ${service.name}`}
              onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
              className="truncate rounded-lg border border-bd bg-surface-2 px-1.5 py-1.5 text-[10px] font-semibold text-text transition-colors hover:border-prime/50 hover:bg-prime-muted sm:text-[11px]"
            >
              Ver
            </button>
            <button
              type="button"
              title={isComparing ? "Quitar de comparación" : "Comparar"}
              aria-label={isComparing ? "Quitar servicio de comparación" : "Comparar servicio"}
              onClick={() => onToggleCompare(service.id)}
              aria-pressed={isComparing}
              className={`truncate rounded-lg border px-1.5 py-1.5 text-[10px] font-semibold transition-colors sm:text-[11px] ${
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
