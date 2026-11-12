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
      className={`group relative flex flex-col overflow-hidden rounded-2xl border bg-slate-900/70 transition-colors duration-200 ${
        isComparing
          ? "z-[1] border-cyan-300/80 ring-1 ring-cyan-300/20"
          : "border-slate-700/70 hover:border-cyan-300/60"
      }`}
    >
      <div
        className="relative h-40 cursor-pointer overflow-hidden bg-slate-950/60"
        onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
      >
        {service.image ? (
          <img
            src={service.image}
            alt={service.name}
            className="h-full w-full object-contain p-4"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm font-medium text-slate-300">
            Sin imagen
          </div>
        )}
        <div className="absolute left-2.5 top-2.5">
          <span className="rounded-full border border-slate-700 bg-slate-950/80 px-2.5 py-1 text-[10px] font-semibold uppercase tracking-wide text-slate-200 backdrop-blur">
            {service.category}
          </span>
        </div>
        <div className="absolute right-3 top-3 rounded-full border border-slate-700/80 bg-slate-950/80 backdrop-blur">
          <FavoriteButton
            active={isFavorite}
            onClick={(e) => {
              e.stopPropagation();
              onToggleFavorite(service.id, "service");
            }}
          />
        </div>
        <div className="absolute bottom-2 left-3">
          <span className="rounded-md border border-slate-700/70 bg-slate-950/80 px-2 py-1 text-xs font-medium text-slate-200 backdrop-blur-sm">
            {service.provider}
          </span>
        </div>
      </div>

      <div className="flex flex-1 flex-col gap-3 p-4">
        <div>
          <span className="text-[10px] font-semibold uppercase tracking-[0.14em] text-cyan-300">
            {service.subcategory}
          </span>
          <h3
            className="mt-1 line-clamp-2 cursor-pointer text-base font-semibold leading-snug text-slate-100 transition-colors group-hover:text-white"
            onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
          >
            {service.name}
          </h3>
        </div>

        <div className="flex min-h-6 flex-wrap gap-1.5">
          {Object.entries(service.specs)
            .slice(0, 2)
            .map(([k, v]) => (
              <span
                key={k}
                title={k}
                className="rounded-md border border-slate-700/80 bg-slate-950/50 px-2 py-1 text-[10px] font-medium text-slate-300"
              >
                {k}: {v}
              </span>
            ))}
        </div>

        <div className="[&_span]:text-slate-300">
          <Rating value={service.rating} count={service.reviewCount} />
        </div>

        <div className="mt-auto rounded-xl border border-slate-800 bg-slate-950/40 p-3">
          <div className="mb-1 text-[10px] font-semibold uppercase tracking-wide text-slate-400">
            Precio del servicio
          </div>
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="text-xl font-bold tracking-tight text-cyan-200">
              {formatPrice(service.monthlyPrice)}
            </span>
            <span className="text-[10px] font-semibold text-slate-400">
              {getBillingPeriodText(service.billingPeriod)}
            </span>
          </div>
          <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[10px] font-medium text-slate-400">
            <span>
              Instalación:{" "}
              {service.installationCost === null || service.installationCost === 0
                ? "Gratis"
                : formatPrice(service.installationCost)}
            </span>
            <span>
              Contrato:{" "}
              {service.contractMonths ? `${service.contractMonths} meses` : "Sin permanencia"}
            </span>
          </div>
        </div>

        <div className="flex gap-2">
          <button
            type="button"
            onClick={() => void addServiceToCart()}
            aria-live="polite"
            aria-disabled={isInCart || adding}
            className={`flex-1 rounded-lg py-2.5 text-xs font-semibold transition ${
              isInCart
                ? "bg-emerald-900/70 text-emerald-200"
                : adding
                  ? "bg-emerald-900/70 text-emerald-200"
                  : "bg-gradient-to-r from-cyan-600 to-violet-600 text-white hover:brightness-110"
            } ${isInCart || adding ? "cursor-default" : ""}`}
          >
            {isInCart
              ? "Agregado en la cesta"
              : adding
                ? "Agregando al carrito..."
                : "Agregar al carrito"}
          </button>
          <button
            type="button"
            onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
            className="flex-1 rounded-lg border border-slate-700 bg-slate-800/50 py-2.5 text-xs font-semibold text-slate-200 transition-colors hover:border-slate-500 hover:bg-slate-800"
          >
            Ver servicio
          </button>
        </div>
        <button
          type="button"
          onClick={() => onToggleCompare(service.id)}
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
