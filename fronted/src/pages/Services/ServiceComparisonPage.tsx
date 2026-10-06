import { Fragment, useEffect, useRef, useState } from "react";
import type { Page, Service } from "../../types";
import { getServiceById } from "../../services/api/api";
import {
  formatPrice,
  getBillingPeriodName,
  getBillingPeriodText,
} from "../../services/utils/productUtils";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  serviceIds: string[];
  navigate: (page: Page) => void;
  cartServiceIds: ReadonlySet<string>;
  onAddToCart: (service: Service) => Promise<boolean>;
}

export default function ServiceComparisonPage({
  serviceIds,
  navigate,
  cartServiceIds,
  onAddToCart,
}: Props) {
  const requestKey = JSON.stringify(serviceIds);
  const [result, setResult] = useState<{ requestKey: string; services: Service[] } | null>(null);
  const [addingServiceIdsState, setAddingServiceIdsState] = useState<Set<string>>(() => new Set());
  const addingServiceIds = useRef(new Set<string>());

  const addServiceToCart = async (service: Service) => {
    if (cartServiceIds.has(service.id) || addingServiceIds.current.has(service.id)) return;
    addingServiceIds.current.add(service.id);
    setAddingServiceIdsState((previous) => new Set(previous).add(service.id));
    try {
      await onAddToCart(service);
    } finally {
      addingServiceIds.current.delete(service.id);
      setAddingServiceIdsState((previous) => {
        const next = new Set(previous);
        next.delete(service.id);
        return next;
      });
    }
  };

  useEffect(() => {
    let cancelled = false;
    const currentRequestKey = JSON.stringify(serviceIds);

    Promise.all(serviceIds.map((id) => getServiceById(id)))
      .then((services) => {
        if (!cancelled) {
          setResult({
            requestKey: currentRequestKey,
            services: services.filter((service): service is Service => service !== null),
          });
        }
      })
      .catch((error) => {
        console.error(error);
        if (!cancelled) {
          setResult({ requestKey: currentRequestKey, services: [] });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [serviceIds]);

  const loading = result?.requestKey !== requestKey;
  const selected = result?.requestKey === requestKey ? result.services : [];

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="text-slate-300">Cargando comparación...</p>
      </div>
    );
  }

  if (selected.length < 2) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="mb-4 text-slate-300">Selecciona al menos 2 servicios para comparar.</p>

        <button
          onClick={() => navigate({ id: "search-services", query: "" })}
          className="rounded-xl bg-gradient-to-r from-cyan-500 via-violet-500 to-fuchsia-400 px-5 py-2 text-sm font-semibold text-white shadow-md shadow-violet-950/40 transition hover:brightness-110"
        >
          Buscar servicios
        </button>
      </div>
    );
  }

  const getBillingPeriod = (service: Service) => service.billingPeriod ?? "monthly";
  const getLowestPrice = (service: Service) =>
    Math.min(
      ...selected
        .filter((candidate) => getBillingPeriod(candidate) === getBillingPeriod(service))
        .map((candidate) => candidate.monthlyPrice),
    );
  const getTotalPrice = (service: Service) =>
    service.monthlyPrice + (service.installationCost ?? 0);
  const getLowestTotalPrice = (service: Service) =>
    Math.min(
      ...selected
        .filter((candidate) => getBillingPeriod(candidate) === getBillingPeriod(service))
        .map(getTotalPrice),
    );
  const highestRating = Math.max(...selected.map((s) => s.rating));

  const compareRows: {
    key: string;
    label: string;
    getValue: (s: Service) => string;
  }[] = [
    {
      key: "monthlyPrice",
      label: "Precio del servicio",
      getValue: (s) => `${formatPrice(s.monthlyPrice)} ${getBillingPeriodText(s.billingPeriod)}`,
    },
    {
      key: "totalPrice",
      label: "Instalación + precio del período",
      getValue: (s) =>
        `${formatPrice(getTotalPrice(s))} ${getBillingPeriodText(s.billingPeriod)}`,
    },
    {
      key: "installation",
      label: "Instalación",
      getValue: (s) =>
        s.installationCost === 0
          ? "Gratis"
          : s.installationCost
            ? formatPrice(s.installationCost)
            : "Sin costo",
    },
    {
      key: "contract",
      label: "Permanencia",
      getValue: (s) => (s.contractMonths ? `${s.contractMonths} meses` : "Sin permanencia"),
    },
    {
      key: "rating",
      label: "Valoración",
      getValue: (s) => `★ ${s.rating.toFixed(1)} (${s.reviewCount.toLocaleString("es-CL")})`,
    },
    {
      key: "coverage",
      label: "Cobertura",
      getValue: (s) => s.coverage,
    },
  ];

  const comparisonGroups = [
    {
      title: "Precio y valoración",
      rows: compareRows.filter((row) => row.key === "monthlyPrice" || row.key === "rating"),
    },
    {
      title: "Precio total",
      rows: compareRows.filter((row) => row.key === "totalPrice"),
    },
    {
      title: "Condiciones del servicio",
      rows: compareRows.filter((row) => row.key === "installation" || row.key === "contract"),
    },
    {
      title: "Cobertura",
      rows: compareRows.filter((row) => row.key === "coverage"),
    },
  ];
  const specKeys = [...new Set(selected.flatMap((s) => Object.keys(s.specs)))];

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Servicios",
            onClick: () => navigate({ id: "search-services", query: "" }),
          },
          { label: "Comparación" },
        ]}
      />

      <h1 className="mb-2 text-2xl font-bold text-slate-100">Comparación de servicios</h1>

      <p className="mb-8 text-sm text-slate-300">Comparando {selected.length} servicios</p>

      <div className="rounded-[28px] border border-slate-600/80 bg-gradient-to-br from-slate-900/95 via-slate-950/90 to-indigo-950/45 p-3 shadow-[0_24px_70px_rgba(2,6,23,0.55)] backdrop-blur-sm sm:p-5">
        <p className="mb-4 flex flex-wrap items-center gap-2 rounded-xl border border-slate-700/70 bg-slate-950/45 px-3 py-2 text-[11px] text-slate-200">
          <span className="inline-flex items-center gap-1 rounded-full border border-cyan-200/50 bg-cyan-300 px-2 py-1 font-extrabold text-slate-950">
            ✦ Mejor
          </span>
          El brillo cian destaca los servicios con mejor precio en su período y valoración.
        </p>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] border-separate border-spacing-1.5">
            <thead>
              <tr>
                <th className="w-44 rounded-xl border border-slate-700 bg-slate-800/90 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-200">
                  Característica
                </th>

                {selected.map((service) => (
                  <th
                    key={service.id}
                    className="rounded-xl border border-slate-700 bg-gradient-to-br from-slate-800/95 to-slate-900/95 p-4 text-center"
                  >
                    <div className="flex flex-col items-center gap-3">
                      {service.image ? (
                        <img
                          src={service.image}
                          alt={service.name}
                          className="h-16 w-20 rounded-2xl object-cover"
                        />
                      ) : (
                        <div className="flex h-16 w-20 items-center justify-center rounded-2xl bg-slate-700/80 text-[10px] text-slate-300">
                          Sin imagen
                        </div>
                      )}

                      <div>
                        <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-cyan-300">
                          {service.provider}
                        </div>

                        <div className="mt-1 max-w-[180px] text-sm font-semibold leading-tight text-slate-100">
                          {service.name}
                        </div>
                      </div>

                      <button
                        onClick={() =>
                          navigate({
                            id: "service-detail",
                            serviceId: service.id,
                          })
                        }
                        className="rounded-lg border border-slate-600 bg-slate-700/80 px-3 py-1 text-[11px] font-medium text-slate-200 transition-colors hover:border-cyan-400 hover:text-cyan-200"
                      >
                        Ver detalle
                      </button>
                      <button
                        type="button"
                        onClick={() => void addServiceToCart(service)}
                        aria-live="polite"
                        aria-disabled={Boolean(
                          service.offerId &&
                          (cartServiceIds.has(service.id) || addingServiceIdsState.has(service.id)),
                        )}
                        className={`rounded-lg px-3 py-1.5 text-[11px] font-semibold shadow-md transition ${
                          cartServiceIds.has(service.id)
                            ? "bg-emerald-700 text-emerald-100 shadow-emerald-950/30"
                            : addingServiceIdsState.has(service.id)
                              ? "bg-emerald-800 text-emerald-100 shadow-emerald-950/30"
                              : "bg-gradient-to-r from-[#ff9878] to-[#fb7185] text-white shadow-rose-500/15 hover:brightness-105"
                        } ${
                          service.offerId &&
                          (cartServiceIds.has(service.id) || addingServiceIdsState.has(service.id))
                            ? "cursor-default"
                            : ""
                        }`}
                      >
                        {cartServiceIds.has(service.id)
                          ? "Agregado en la cesta"
                          : addingServiceIdsState.has(service.id)
                            ? "Agregando a la cesta..."
                            : "Agregar al carrito"}
                      </button>
                    </div>
                  </th>
                ))}
              </tr>
            </thead>

            <tbody>
              {comparisonGroups.map((group) => (
                <Fragment key={group.title}>
                  <tr>
                    <th
                      colSpan={selected.length + 1}
                      className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[11px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                    >
                      {group.title}
                    </th>
                  </tr>
                  {group.rows.map((row) => (
                    <tr key={row.key}>
                      <th
                        scope="row"
                        className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
                      >
                        {row.label}
                      </th>
                      {selected.map((service) => {
                        const isBest =
                          (row.key === "monthlyPrice" &&
                            service.monthlyPrice === getLowestPrice(service)) ||
                          (row.key === "totalPrice" &&
                            getTotalPrice(service) === getLowestTotalPrice(service)) ||
                          (row.key === "rating" &&
                            service.rating > 0 &&
                            service.rating === highestRating);
                        const value = row.getValue(service);
                        return (
                          <td key={service.id} className="p-1.5 text-center align-middle">
                            <div
                              className={`flex min-h-16 flex-col items-center justify-center rounded-xl border px-3 py-2.5 transition duration-200 hover:-translate-y-0.5 ${
                                isBest
                                  ? "border-cyan-200 bg-gradient-to-br from-cyan-300/25 via-sky-500/15 to-violet-500/20 text-white shadow-[0_0_24px_rgba(34,211,238,0.24),inset_0_0_0_1px_rgba(165,243,252,0.2)] ring-1 ring-cyan-200/30"
                                  : "border-slate-700/70 bg-slate-950/40 text-slate-100"
                              }`}
                            >
                              <span className="block text-sm font-bold leading-snug">{value}</span>
                              {isBest && (
                                <span className="mt-1.5 inline-flex items-center gap-1 rounded-full border border-cyan-100/70 bg-cyan-100 px-2.5 py-0.5 text-[9px] font-black uppercase tracking-[0.14em] text-slate-950 shadow-[0_0_14px_rgba(103,232,249,0.45)]">
                                  ✦
                                  {row.key === "rating"
                                    ? "Mejor valoración"
                                    : row.key === "totalPrice"
                                      ? "Menor precio total"
                                      : "Mejor"}
                                </span>
                              )}
                            </div>
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </Fragment>
              ))}

              {specKeys.length > 0 && (
                <>
                  <tr>
                    <th
                      colSpan={selected.length + 1}
                      className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[11px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                    >
                      Especificaciones del servicio
                    </th>
                  </tr>
                  {specKeys.map((key) => (
                    <tr key={key}>
                      <th
                        scope="row"
                        className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
                      >
                        {key}
                      </th>
                      {selected.map((service) => {
                        const value = service.specs[key]?.trim();
                        return (
                          <td key={service.id} className="p-1.5 text-center align-middle">
                            <div className="flex min-h-16 flex-col items-center justify-center rounded-xl border border-slate-700/70 bg-slate-950/40 px-3 py-2.5 text-slate-100">
                              <span className="block text-sm font-bold leading-snug">
                                {value || "No informado"}
                              </span>
                            </div>
                          </td>
                        );
                      })}
                    </tr>
                  ))}
                </>
              )}

              <tr>
                <th
                  colSpan={selected.length + 1}
                  className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[11px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                >
                  Beneficios incluidos
                </th>
              </tr>
              <tr>
                <th
                  scope="row"
                  className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200 align-top"
                >
                  Beneficios
                </th>
                {selected.map((service) => (
                  <td key={service.id} className="p-1.5 align-top">
                    <div className="min-h-16 rounded-xl border border-slate-700/70 bg-slate-950/40 p-3">
                      <ul className="space-y-1">
                        {service.benefits.length > 0 ? (
                          service.benefits.map((benefit) => (
                            <li
                              key={benefit}
                              className="flex items-center gap-1.5 text-xs text-slate-200"
                            >
                              <svg
                                width="10"
                                height="10"
                                viewBox="0 0 24 24"
                                fill="none"
                                stroke="#67e8f9"
                                strokeWidth="3"
                              >
                                <path d="M20 6 9 17l-5-5" />
                              </svg>
                              {benefit}
                            </li>
                          ))
                        ) : (
                          <li className="text-sm text-slate-400">No informado</li>
                        )}
                      </ul>
                    </div>
                  </td>
                ))}
              </tr>
            </tbody>
          </table>
        </div>
      </div>
    </div>
  );
}
