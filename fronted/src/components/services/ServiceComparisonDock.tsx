import { useEffect, useState } from "react";
import type { Page, Service } from "../../types";
import { getServiceById } from "../../services/api/api";

interface Props {
  serviceIds: string[];
  onClear: () => void;
  onRemove: (service: Service) => void;
  onNavigate: (page: Page) => void;
}

export default function ServiceComparisonDock({
  serviceIds,
  onClear,
  onRemove,
  onNavigate,
}: Props) {
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(true);
  const serviceKey = serviceIds.join(",");

  useEffect(() => {
    let cancelled = false;

    async function loadServices() {
      setLoading(true);
      setError(null);
      try {
        const ids = serviceKey ? serviceKey.split(",") : [];
        const results = await Promise.all(ids.map((id) => getServiceById(id)));
        if (cancelled) return;
        const availableServices = results.filter((service): service is Service => service !== null);
        setServices(availableServices);
        if (availableServices.length !== ids.length) {
          setError("No se pudo encontrar uno o más servicios seleccionados.");
        }
      } catch (loadError) {
        if (cancelled) return;
        setError(
          loadError instanceof Error
            ? loadError.message
            : "No se pudieron cargar los servicios seleccionados.",
        );
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadServices();
    return () => {
      cancelled = true;
    };
  }, [serviceKey]);

  return (
    <section
      aria-label="Servicios seleccionados para comparar"
      className="w-[min(22rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border border-white/15 bg-slate-950/95 text-white shadow-[0_20px_60px_rgba(0,0,0,0.5)] backdrop-blur-xl"
    >
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((open) => !open)}
        className="flex w-full items-center justify-between gap-3 bg-gradient-to-r from-cyan-600 via-violet-600 to-fuchsia-500 px-4 py-3 text-left"
      >
        <span className="flex min-w-0 items-center gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-white/20 text-lg">
            ⚖
          </span>
          <span className="min-w-0">
            <span className="block text-[10px] font-bold uppercase tracking-[0.16em] text-white/75">
              {expanded ? "¿Qué estás comparando?" : "Comparador"}
            </span>
            <span className="block truncate text-sm font-extrabold">
              {serviceIds.length} servicio{serviceIds.length === 1 ? "" : "s"} seleccionado
              {serviceIds.length === 1 ? "" : "s"}
            </span>
          </span>
        </span>
        <span aria-hidden="true" className="text-lg font-bold">
          {expanded ? "−" : "+"}
        </span>
      </button>

      {expanded && (
        <div className="space-y-3 p-3">
          {loading ? (
            <p className="px-1 py-2 text-xs text-slate-300">Cargando servicios...</p>
          ) : (
            <ul className="space-y-2">
              {services.map((service) => (
                <li
                  key={service.id}
                  className="flex items-center gap-2 rounded-xl border border-white/10 bg-white/[0.06] p-2"
                >
                  <button
                    type="button"
                    onClick={() => onNavigate({ id: "service-detail", serviceId: service.id })}
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    title={`Ver ${service.name}`}
                  >
                    {service.image ? (
                      <img
                        src={service.image}
                        alt=""
                        className="h-10 w-10 shrink-0 rounded-lg object-cover"
                      />
                    ) : (
                      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-cyan-500/20 text-xs text-cyan-200">
                        {service.provider.slice(0, 2).toUpperCase() || "SE"}
                      </span>
                    )}
                    <span className="min-w-0">
                      <span className="block truncate text-xs font-bold text-white">
                        {service.name}
                      </span>
                      <span className="block truncate text-[10px] text-slate-300">
                        {service.provider} · {service.category}
                      </span>
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={() => onRemove(service)}
                    aria-label={`Quitar ${service.name} de la comparación`}
                    title="Quitar de la comparación"
                    className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-slate-300 transition hover:bg-rose-500/20 hover:text-rose-200"
                  >
                    ×
                  </button>
                </li>
              ))}
            </ul>
          )}

          {error && (
            <p
              role="alert"
              className="rounded-lg bg-rose-500/10 px-2 py-1.5 text-[11px] text-rose-200"
            >
              {error}
            </p>
          )}

          <div className="flex gap-2">
            {serviceIds.length >= 2 && (
              <button
                type="button"
                onClick={() => onNavigate({ id: "service-comparison", serviceIds })}
                className="flex-1 rounded-xl bg-gradient-to-r from-cyan-500 via-violet-500 to-fuchsia-400 px-3 py-2 text-xs font-extrabold text-white shadow-lg shadow-violet-950/40 transition hover:brightness-110"
              >
                Comparar servicios
              </button>
            )}
            <button
              type="button"
              onClick={onClear}
              className="rounded-xl border border-white/15 px-3 py-2 text-xs font-semibold text-slate-200 transition hover:border-rose-300/50 hover:bg-rose-500/10 hover:text-rose-100"
            >
              Limpiar
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
