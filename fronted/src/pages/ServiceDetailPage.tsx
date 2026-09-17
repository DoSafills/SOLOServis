import { useEffect, useState } from "react";
import type { Page, Service } from "../types";
import { getServiceById, formatPrice } from "../Services/api/frontend-src/api";
import { Badge, Breadcrumb, FavoriteButton, Rating } from "../components/ui";
import PriceHistory from "../components/PriceHistory";

interface Props {
  serviceId: string;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

export default function ServiceDetailPage({
  serviceId,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [service, setService] = useState<Service | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadService() {
      setLoading(true);
      setError(null);
      try {
        const result = await getServiceById(serviceId);
        if (cancelled) return;
        setService(result);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Error al cargar el servicio");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadService();

    return () => {
      cancelled = true;
    };
  }, [serviceId]);

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando servicio…</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn">{error}</p>
      </div>
    );
  }

  if (!service)
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Servicio no encontrado.</p>
      </div>
    );

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Servicios", onClick: () => navigate({ id: "search-services", query: "" }) },
          {
            label: service.category,
            onClick: () => navigate({ id: "search-services", query: service.category }),
          },
          { label: service.name },
        ]}
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        {/* Image */}
        <div
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="rounded-2xl overflow-hidden h-64"
        >
          <img src={service.image} alt={service.name} className="w-full h-full object-cover" />
        </div>

        {/* Info */}
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
            <FavoriteButton active={isFavorite} onClick={() => onToggleFavorite(service.id)} />
          </div>

          <p className="text-sm text-muted leading-relaxed">{service.description}</p>

          {/* Key specs */}
          <div
            style={{ background: "#111111", border: "1px solid #2A2A2A" }}
            className="rounded-2xl p-4"
          >
            <h3 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3">
              Características
            </h3>
            <div className="grid grid-cols-2 gap-2">
              {Object.entries(service.specs).map(([k, v]) => (
                <div key={k} style={{ background: "#1A1A1A" }} className="rounded-xl px-3 py-2">
                  <div className="text-xs text-muted">{k}</div>
                  <div className="text-sm font-semibold text-text">{v}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Price */}
          <div
            style={{ background: "rgba(232,0,27,0.08)", border: "1px solid rgba(232,0,27,0.25)" }}
            className="rounded-2xl p-4"
          >
            <div className="text-xs text-prime font-semibold mb-1">Precio mensual</div>
            <div className="price text-3xl font-bold text-prime">
              {formatPrice(service.monthlyPrice)}
              <span className="text-sm font-normal text-muted"> /mes</span>
            </div>
            <div className="flex gap-4 mt-2 text-xs text-muted">
              <span>
                Instalación:{" "}
                {service.installationCost === 0
                  ? "Gratis"
                  : service.installationCost
                    ? formatPrice(service.installationCost)
                    : "Sin costo"}
              </span>
              <span>
                Contrato:{" "}
                {service.contractMonths ? `${service.contractMonths} meses` : "Sin permanencia"}
              </span>
            </div>
          </div>

          <button
            onClick={() => onToggleCompare(service.id)}
            style={
              isComparing
                ? { background: "#E8001B", color: "#0A0A0A" }
                : { background: "#1A1A1A", border: "1px solid #2A2A2A", color: "#94A3B8" }
            }
            className="py-3 rounded-2xl text-sm font-semibold transition-all hover:border-prime hover:text-prime"
          >
            {isComparing ? "✓ Agregado al comparador" : "Agregar al comparador"}
          </button>
        </div>
      </div>

      {/* Benefits */}
      <section
        style={{ background: "#111111", border: "1px solid #2A2A2A" }}
        className="rounded-2xl p-6 mb-6"
      >
        <h2 className="text-lg font-bold text-text mb-4">Beneficios incluidos</h2>
        <div className="flex flex-wrap gap-2">
          {service.benefits.map((b) => (
            <div
              key={b}
              style={{ background: "rgba(232,0,27,0.1)", border: "1px solid rgba(232,0,27,0.2)" }}
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
              <span className="text-sm text-muted-2">{b}</span>
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
