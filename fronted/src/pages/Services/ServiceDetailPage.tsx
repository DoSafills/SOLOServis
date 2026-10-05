import { useEffect, useState } from "react";
import type { Page, Service } from "../../types";
import { getServiceById } from "../../services/api/api";
import {
  formatServicePrice,
  getServiceBillingPeriodLabel,
} from "../../Services/utils/productUtils";
import { Badge, Breadcrumb, FavoriteButton, Rating } from "../../components/common/ui";
import PriceHistory from "../../components/products/PriceHistory";

interface Props {
  serviceId: string;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  isInCart: boolean;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string, category: string) => void;
  onAddToCart: (service: Service) => void;
}

export default function ServiceDetailPage({
  serviceId,
  navigate,
  isFavorite,
  isComparing,
  isInCart,
  onToggleFavorite,
  onToggleCompare,
  onAddToCart,
}: Props) {
  const [result, setResult] = useState<{ serviceId: string; service: Service | null } | null>(null);

  useEffect(() => {
    let cancelled = false;

    getServiceById(serviceId)
      .then((service) => {
        if (!cancelled) {
          setResult({ serviceId, service });
        }
      })
      .catch((error) => {
        console.error(error);
        if (!cancelled) {
          setResult({ serviceId, service: null });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [serviceId]);

  const loading = result?.serviceId !== serviceId;
  const service = result?.serviceId === serviceId ? result.service : null;
  const contractPeriod = service
    ? service.contractPeriod?.trim() ||
      (service.contractMonths ? `${service.contractMonths} meses` : "No informado")
    : "";

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando servicio...</p>
      </div>
    );
  }

  if (!service) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Servicio no encontrado.</p>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Servicios",
            onClick: () => navigate({ id: "search-services", query: "" }),
          },
          {
            label: service.category,
            onClick: () =>
              navigate({
                id: "search-services",
                query: service.category,
              }),
          },
          { label: service.name },
        ]}
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        <div
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="flex h-64 items-center justify-center overflow-hidden rounded-2xl"
        >
          {service.image ? (
            <img src={service.image} alt={service.name} className="h-full w-full object-cover" />
          ) : (
            <span className="text-sm font-medium text-slate-300">Sin imagen</span>
          )}
        </div>

        <div className="flex flex-col gap-4">
          <div className="flex items-start justify-between">
            <div>
              <Badge variant="available">
                {service.category} · {service.subcategory}
              </Badge>

              <h1 className="text-2xl font-bold text-text mt-2 leading-snug">{service.name}</h1>

              <div className="text-sm font-semibold text-prime mt-1">{service.provider}</div>

              <div className="mt-2">
                <Rating value={service.rating} count={service.reviewCount} />
              </div>
            </div>

            <FavoriteButton
              active={isFavorite}
              onClick={() => onToggleFavorite(service.id, "service")}
            />
          </div>

          <p className="text-sm text-muted leading-relaxed">{service.description}</p>

          <div
            style={{ background: "#111111", border: "1px solid #2A2A2A" }}
            className="rounded-2xl p-4"
          >
            <h3 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3">
              Características
            </h3>

            <div className="grid grid-cols-2 gap-2">
              {Object.entries(service.specs).map(([key, value]) => (
                <div key={key} style={{ background: "#1A1A1A" }} className="rounded-xl px-3 py-2">
                  <div className="text-xs text-muted">{key}</div>
                  <div className="text-sm font-semibold text-text">{value}</div>
                </div>
              ))}
            </div>
          </div>

          <div
            style={{
              background: "rgba(232,0,27,0.08)",
              border: "1px solid rgba(232,0,27,0.25)",
            }}
            className="rounded-2xl p-4"
          >
            <div className="text-xs text-prime font-semibold mb-1">Precio</div>

            <div className="price text-3xl font-bold text-prime">
              {formatServicePrice(service.monthlyPrice, service.currency)}
              {getServiceBillingPeriodLabel(service.billingPeriod) && (
                <span className="text-sm font-normal text-muted">
                  {" "}
                  {getServiceBillingPeriodLabel(service.billingPeriod)}
                </span>
              )}
            </div>

            <div className="flex gap-4 mt-2 text-xs text-muted">
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

          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <button
              type="button"
              onClick={() => onAddToCart(service)}
              disabled={isInCart}
              className={`rounded-2xl border py-3 text-sm font-semibold transition-all ${
                isInCart
                  ? "cursor-default border-emerald-300/30 bg-emerald-400/10 text-emerald-200"
                  : "border-emerald-300/30 bg-emerald-400/10 text-emerald-100 hover:border-emerald-300/60 hover:bg-emerald-400/20"
              }`}
            >
              {isInCart ? "✓ En la cesta" : "Agregar a la cesta"}
            </button>
            <button
              type="button"
              onClick={() => onToggleCompare(service.id, service.category)}
              style={
                isComparing
                  ? { background: "#E8001B", color: "#0A0A0A" }
                  : {
                      background: "#1A1A1A",
                      border: "1px solid #2A2A2A",
                      color: "#94A3B8",
                    }
              }
              className="rounded-2xl py-3 text-sm font-semibold transition-all hover:border-prime hover:text-prime"
            >
              {isComparing ? "✓ Agregado al comparador" : "Agregar al comparador"}
            </button>
          </div>
        </div>
      </div>

      <section
        style={{ background: "#111111", border: "1px solid #2A2A2A" }}
        className="rounded-2xl p-6 mb-6"
      >
        <h2 className="text-lg font-bold text-text mb-4">Beneficios incluidos</h2>

        <div className="flex flex-wrap gap-2">
          {service.benefits.map((benefit) => (
            <div
              key={benefit}
              style={{
                background: "rgba(232,0,27,0.1)",
                border: "1px solid rgba(232,0,27,0.2)",
              }}
              className="flex items-center gap-2 px-3 py-1.5 rounded-xl"
            >
              <svg
                width="12"
                height="12"
                viewBox="0 0 24 24"
                fill="none"
                stroke="#E8001B"
                strokeWidth="3"
              >
                <path d="M20 6 9 17l-5-5" />
              </svg>

              <span className="text-sm text-muted-2">{benefit}</span>
            </div>
          ))}
        </div>

        <div className="mt-4 text-sm text-muted">
          <span className="font-semibold text-muted-2">Cobertura: </span>
          {service.coverage}
        </div>
      </section>

      <PriceHistory history={service.priceHistory} offerHistory={[]} />
    </div>
  );
}
