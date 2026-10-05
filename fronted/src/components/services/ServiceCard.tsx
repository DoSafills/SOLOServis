import type { Service, Page } from "../../types";
import {
  formatServicePrice,
  getServiceBillingPeriodLabel,
} from "../../Services/utils/productUtils";
import { Rating, FavoriteButton } from "../common/ui";

interface Props {
  service: Service;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string) => void;
}

export default function ServiceCard({
  service,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const openService = () => navigate({ id: "service-detail", serviceId: service.id });
  const hasReferentialPrice = service.description.includes(
    "Precio mensual referencial; vigencia no confirmada.",
  );
  const contractPeriod =
    service.contractPeriod?.trim() ||
    (service.contractMonths ? `${service.contractMonths} meses` : "No informado");

  return (
    <div className="group relative flex flex-col overflow-hidden rounded-2xl border border-indigo-300/20 bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-950 shadow-lg shadow-slate-950/30 transition duration-300 hover:-translate-y-1 hover:border-violet-300/60 hover:shadow-xl hover:shadow-violet-950/40">
      {/* Image */}
      <div
        className="relative h-36 cursor-pointer overflow-hidden bg-gradient-to-br from-violet-950 via-slate-900 to-cyan-950"
        onClick={openService}
      >
        {service.image ? (
          <img
            src={service.image}
            alt={service.name}
            className="h-full w-full object-cover transition-transform duration-500 group-hover:scale-105"
          />
        ) : (
          <div className="flex h-full w-full items-center justify-center text-sm font-medium text-slate-300">
            Sin imagen
          </div>
        )}

        <div className="absolute left-2.5 top-2.5 flex gap-1.5">
          <span className="rounded-full border border-emerald-200/30 bg-emerald-400/20 px-2.5 py-1 text-[10px] font-extrabold uppercase tracking-wide text-emerald-100 shadow-lg backdrop-blur">
            {service.category}
          </span>
        </div>

        <div className="absolute right-2.5 top-2.5 rounded-full border border-white/15 bg-slate-950/75 shadow-lg backdrop-blur">
          <FavoriteButton
            active={isFavorite}
            onClick={(e) => {
              e.stopPropagation();
              onToggleFavorite(service.id, "service");
            }}
          />
        </div>
      </div>

      {/* Content */}
      <div className="flex flex-1 flex-col gap-1.5 p-3.5">
        <div>
          <span className="text-[11px] font-bold uppercase tracking-[0.14em] text-violet-300">
            {service.provider}
          </span>
          <h3
            className="mt-0.5 line-clamp-2 cursor-pointer text-sm font-bold leading-snug text-slate-50 transition-colors group-hover:text-cyan-100"
            onClick={openService}
          >
            {service.name}
          </h3>
        </div>

        {/* Specs preview */}
        <div className="flex flex-wrap gap-1">
          {Object.entries(service.specs)
            .slice(0, 2)
            .map(([key, value]) => (
              <span
                key={key}
                className="rounded-lg border border-cyan-200/15 bg-cyan-300/10 px-2 py-1 text-[10px] font-semibold text-cyan-100"
                title={key}
              >
                {key}: {value}
              </span>
            ))}
        </div>

        <div className="rounded-lg border border-amber-200/10 bg-amber-300/[0.06] px-2 py-1.5 [&_span]:text-slate-200">
          <Rating value={service.rating} count={service.reviewCount} />
        </div>

        {/* Price */}
        <div className="mt-1 border-t border-indigo-200/15 pt-2">
          <div className="mb-0.5 text-[11px] font-bold uppercase tracking-wide text-slate-300">
            Precio
          </div>
          <div className="flex flex-wrap items-baseline gap-x-2 gap-y-0.5">
            <span className="text-lg font-extrabold text-emerald-300">
              {formatServicePrice(service.monthlyPrice, service.currency)}
            </span>
            {getServiceBillingPeriodLabel(service.billingPeriod) && (
              <span className="text-[10px] font-semibold text-slate-400">
                {getServiceBillingPeriodLabel(service.billingPeriod)}
              </span>
            )}
          </div>
          {hasReferentialPrice && (
            <div className="mt-0.5 text-[10px] font-medium text-amber-200">
              Precio referencial; verificar vigencia
            </div>
          )}
          <div className="mt-1 flex flex-wrap gap-x-3 gap-y-0.5 text-[10px] text-slate-300">
            <span>
              Instalación:{" "}
              {service.installationCost === null
                ? "No informado"
                : service.installationCost === 0
                  ? "Gratis"
                  : formatServicePrice(service.installationCost, service.currency)}
            </span>
            <span>Contrato: {contractPeriod}</span>
          </div>
        </div>

        {/* Actions */}
        <div className="mt-1 flex gap-2">
          <button
            type="button"
            onClick={openService}
            className="flex-1 rounded-lg bg-gradient-to-r from-violet-600 to-fuchsia-500 py-2 text-xs font-bold text-white shadow-md shadow-violet-950/30 transition hover:brightness-110"
          >
            Ver servicio
          </button>
          <button
            type="button"
            onClick={() => onToggleCompare(service.id)}
            aria-pressed={isComparing}
            className={`flex-1 rounded-lg border py-2 text-xs font-bold transition ${
              isComparing
                ? "border-cyan-200/50 bg-gradient-to-r from-cyan-500/25 to-violet-500/25 text-cyan-100 shadow-[0_0_18px_rgba(34,211,238,0.16)]"
                : "border-violet-200/25 bg-violet-400/10 text-violet-100 hover:border-violet-200/50 hover:bg-violet-400/20"
            }`}
          >
            {isComparing ? "✓ Comparando" : "Comparar"}
          </button>
        </div>
      </div>
    </div>
  );
}
