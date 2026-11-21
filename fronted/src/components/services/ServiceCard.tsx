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
 * Tarjeta de servicio horizontal: imagen a la izquierda y datos del proveedor
 * y la mensualidad a la derecha. Recibe el servicio, los estados de favorito,
 * comparación y cesta, y los callbacks de acciones.
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

  /**
   * Agrega el servicio a la cesta evitando dobles clics con `addingRef`.
   * Devuelve una promesa que se resuelve al terminar el callback del padre.
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
    <div
      className={`group relative isolate flex min-h-[14.4rem] overflow-hidden rounded-[1.75rem] border bg-surface shadow-lg shadow-black/20 transition-all duration-300 hover:-translate-y-1 hover:shadow-2xl hover:shadow-black/30 sm:min-h-[13.2rem] ${
        isComparing ? "z-[1] border-prime ring-1 ring-prime/20" : "border-bd hover:border-prime/50"
      }`}
    >
      <div
        className="relative w-[58%] shrink-0 cursor-pointer bg-white"
        style={{ clipPath: "polygon(0 0, 100% 0, 88% 100%, 0 100%)" }}
        onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
      >
        {service.image ? (
          <img
            src={service.image}
            alt={service.name}
            className="absolute inset-0 h-full w-full object-contain object-center p-2 transition-transform duration-500 group-hover:scale-[1.03] sm:p-3"
          />
        ) : (
          <div className="absolute inset-0 flex items-center justify-center text-sm font-medium text-slate-500">
            Sin imagen
          </div>
        )}
        <div className="absolute right-1 top-1 z-20 rounded-full border border-bd bg-surface/90 backdrop-blur-md">
          <FavoriteButton
            active={isFavorite}
            onClick={(e) => {
              e.stopPropagation();
              onToggleFavorite(service.id, "service");
            }}
          />
        </div>
      </div>

      <div className="flex min-w-0 flex-1 flex-col justify-center gap-1 bg-surface px-2 py-1.5">
        <div className="min-w-0">
          <span className="block truncate text-[8px] font-bold uppercase tracking-[0.12em] text-prime sm:text-[9px]">
            {service.provider}
          </span>
          <h3
            className="mt-0.5 line-clamp-2 cursor-pointer text-[11px] font-bold leading-tight text-text transition-colors group-hover:text-prime sm:text-xs"
            onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
          >
            {service.name}
          </h3>
        </div>

        <div className="scale-[0.85] origin-left [&_span]:text-muted">
          <Rating value={service.rating} count={service.reviewCount} />
        </div>

        <div className="border-t border-bd pt-1">
          <div className="flex min-w-0 flex-wrap items-baseline gap-x-1 gap-y-0">
            <span className="text-xs font-bold tracking-tight text-prime sm:text-sm">
              {formatPrice(service.monthlyPrice)}
            </span>
            <span className="text-[8px] font-semibold text-muted">
              {getBillingPeriodText(service.billingPeriod)}
            </span>
          </div>
        </div>

        <div className="flex flex-col gap-1.5">
          <div className="grid grid-cols-2 gap-1.5">
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
              className={`truncate rounded-lg border px-1.5 py-2 text-[9px] font-semibold transition-colors sm:text-[10px] ${
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
              title="Ver servicio"
              aria-label={`Ver servicio ${service.name}`}
              onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
              className="flex min-w-0 items-center justify-center gap-0.5 truncate rounded-lg bg-prime px-1.5 py-2 text-[9px] font-semibold text-[#181714] shadow-lg shadow-black/20 transition-all hover:-translate-y-0.5 hover:bg-prime-dark hover:shadow-xl active:translate-y-0 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime sm:text-[10px]"
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
            aria-label={isComparing ? "Quitar servicio de comparación" : "Comparar servicio"}
            onClick={() => onToggleCompare(service.id)}
            aria-pressed={isComparing}
            className={`w-full truncate rounded-lg border px-2 py-2 text-[9px] font-semibold transition-colors sm:text-[10px] ${
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
