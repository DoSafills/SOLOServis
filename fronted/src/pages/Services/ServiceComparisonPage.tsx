import { Fragment, useEffect, useState } from "react";
import type { Page, Service } from "../../types";
import { getServiceById } from "../../services/api/api";
import {
  formatServicePrice,
  getServiceBillingPeriodLabel,
} from "../../Services/utils/productUtils";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  serviceIds: string[];
  navigate: (page: Page) => void;
  serviceCart: Set<string>;
  onAddToCart: (service: Service) => void;
}

const normalizeComparableText = (value: string): string =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase()
    .trim();

const getSpecBestIndices = (services: Service[], name: string): Set<number> => {
  const entries = services.map((service) => {
    const value = service.specs[name] ?? "";
    const numericMatch = normalizeComparableText(value).match(/[+-]?\d+(?:[.,]\d+)?/);
    const numeric = numericMatch ? Number(numericMatch[0].replace(",", ".")) : null;
    const unit = normalizeComparableText(value)
      .replace(/[+-]?\d+(?:[.,]\d+)?/g, "#")
      .replace(/[^a-z#]+/g, " ")
      .replace(/\s+/g, " ")
      .trim();
    return { value: normalizeComparableText(value), numeric, unit };
  });
  const populated = entries.filter((entry) => entry.value);
  if (populated.length < 2) return new Set();

  const units = new Set(populated.map((entry) => entry.unit));
  if (units.size !== 1 || populated.some((entry) => entry.numeric === null)) return new Set();

  const values = populated.map((entry) => entry.numeric as number);
  const bestValue = Math.max(...values);
  return new Set(entries.flatMap((entry, index) => (entry.numeric === bestValue ? [index] : [])));
};

const bestCellClass =
  "border-cyan-200/80 bg-gradient-to-br from-cyan-300/20 via-sky-500/10 to-violet-500/15 text-white shadow-[0_0_22px_rgba(34,211,238,0.12)] ring-1 ring-cyan-200/20";
const defaultCellClass = "border-white/[0.07] bg-slate-950/35 text-slate-100";

const specificationGroups = [
  {
    title: "Conectividad y velocidad",
    pattern: /(velocidad|conectividad|wifi|wi-fi|red|fibra|datos)/i,
  },
  {
    title: "Plan y condiciones",
    pattern: /(plan|contrato|permanencia|duraci[oó]n|dispositivo|l[ií]mite|usuario)/i,
  },
  {
    title: "Seguridad y privacidad",
    pattern: /(seguridad|privacidad|protecci[oó]n|amenaza|vpn|antivirus|cifrado)/i,
  },
  {
    title: "Educación y contenido",
    pattern: /(curso|idioma|contenido|clase|certificaci[oó]n|aprendizaje|cat[aá]logo)/i,
  },
  { title: "Otros", pattern: /.*/ },
];

export default function ServiceComparisonPage({
  serviceIds,
  navigate,
  serviceCart,
  onAddToCart,
}: Props) {
  const requestKey = JSON.stringify(serviceIds);
  const [result, setResult] = useState<{ requestKey: string; services: Service[] } | null>(null);

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
      <div className="mx-auto max-w-7xl px-4 py-24 text-center">
        <div className="mx-auto flex max-w-sm flex-col items-center rounded-3xl border border-white/10 bg-slate-950/50 p-8">
          <span className="mb-4 flex h-12 w-12 items-center justify-center rounded-2xl bg-cyan-300/10 text-2xl text-cyan-200">
            ⚖
          </span>
          <p className="text-sm font-semibold text-slate-200">Preparando tu comparación</p>
          <p className="mt-1 text-xs text-slate-400">
            Estamos reuniendo los datos de cada servicio.
          </p>
        </div>
      </div>
    );
  }

  if (selected.length < 2) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-24 text-center">
        <div className="mx-auto max-w-lg rounded-3xl border border-white/10 bg-gradient-to-br from-slate-900 to-indigo-950/60 p-8">
          <span className="text-3xl" aria-hidden="true">
            ⚖
          </span>
          <p className="mb-4 mt-3 text-slate-300">Selecciona al menos 2 servicios para comparar.</p>

          <button
            onClick={() => navigate({ id: "search-services", query: "" })}
            className="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-5 py-2.5 text-sm font-bold text-white shadow-lg shadow-violet-950/30 transition hover:brightness-110"
          >
            Buscar servicios
          </button>
        </div>
      </div>
    );
  }

  const selectedCategory = selected[0].category.trim();
  if (
    !selectedCategory ||
    selected.some(
      (service) =>
        service.category.trim().toLocaleLowerCase() !== selectedCategory.toLocaleLowerCase(),
    )
  ) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-24 text-center">
        <div className="mx-auto max-w-lg rounded-3xl border border-amber-200/15 bg-gradient-to-br from-slate-900 to-amber-950/20 p-8">
          <span className="text-3xl" aria-hidden="true">
            ⚠
          </span>
          <p className="mb-4 mt-3 text-slate-300">
            Solo puedes comparar servicios que pertenezcan a la misma categoría.
          </p>
          <button
            type="button"
            onClick={() => navigate({ id: "search-services", query: "" })}
            className="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-5 py-2.5 text-sm font-bold text-white shadow-lg shadow-violet-950/30 transition hover:brightness-110"
          >
            Volver a los servicios
          </button>
        </div>
      </div>
    );
  }

  const hasComparablePrices = selected.every(
    (service) =>
      service.currency === selected[0].currency &&
      service.billingPeriod === selected[0].billingPeriod,
  );
  const lowestPrice = hasComparablePrices
    ? Math.min(...selected.map((service) => service.monthlyPrice))
    : null;
  const highestRating = Math.max(...selected.map((s) => s.rating));
  const bestPriceIndices = hasComparablePrices
    ? new Set(
        selected.flatMap((service, index) => (service.monthlyPrice === lowestPrice ? [index] : [])),
      )
    : new Set<number>();
  const bestRatingIndices = new Set(
    selected.flatMap((service, index) => (service.rating === highestRating ? [index] : [])),
  );

  const compareRows: {
    key: string;
    label: string;
    getValue: (s: Service) => string;
  }[] = [
    {
      key: "monthlyPrice",
      label: "Precio",
      getValue: (s) =>
        `${formatServicePrice(s.monthlyPrice, s.currency)} ${getServiceBillingPeriodLabel(s.billingPeriod)}`.trim(),
    },
    {
      key: "installation",
      label: "Instalación",
      getValue: (s) =>
        s.installationCost === null
          ? "No informado"
          : s.installationCost === 0
            ? "Gratis"
            : formatServicePrice(s.installationCost, s.currency),
    },
    {
      key: "contract",
      label: "Permanencia",
      getValue: (s) =>
        s.contractPeriod?.trim() ||
        (s.contractMonths ? `${s.contractMonths} meses` : "No informado"),
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

  const specNames = [...new Set(selected.flatMap((service) => Object.keys(service.specs)))].sort(
    (first, second) => first.localeCompare(second, "es"),
  );
  const assignedSpecs = new Set<string>();
  const specGroups = specificationGroups
    .map(({ title, pattern }) => ({
      title,
      names: specNames.filter((name) => {
        if (assignedSpecs.has(name) || !pattern.test(name)) return false;
        assignedSpecs.add(name);
        return true;
      }),
    }))
    .filter((group) => group.names.length > 0);

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
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

      <section className="relative mb-7 overflow-hidden rounded-[28px] border border-slate-600/80 bg-gradient-to-br from-slate-900/95 via-slate-950/90 to-indigo-950/45 px-5 py-6 shadow-[0_24px_70px_rgba(2,6,23,0.55)] sm:px-8 sm:py-8">
        <div
          aria-hidden="true"
          className="pointer-events-none absolute -right-16 -top-24 h-64 w-64 rounded-full bg-cyan-400/10 blur-3xl"
        />
        <div className="relative flex flex-col gap-5 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <p className="mb-2 flex items-center gap-2 text-[10px] font-extrabold uppercase tracking-[0.2em] text-cyan-200">
              <span className="flex h-7 w-7 items-center justify-center rounded-lg border border-cyan-200/20 bg-cyan-300/10 text-base">
                ⚖
              </span>
              Comparador de servicios
            </p>
            <h1 className="text-2xl font-black tracking-tight text-white sm:text-3xl">
              Elige el servicio ideal
            </h1>
            <p className="mt-2 max-w-xl text-sm leading-relaxed text-slate-300">
              Revisa precios, condiciones y características en una sola vista para decidir con más
              claridad.
            </p>
          </div>
          <div className="flex shrink-0 flex-wrap gap-2">
            <span className="rounded-xl border border-white/10 bg-slate-950/45 px-3 py-2 text-xs font-semibold text-slate-200">
              {selected.length} servicios
            </span>
            <span className="rounded-xl border border-cyan-200/20 bg-cyan-300/10 px-3 py-2 text-xs font-bold text-cyan-100">
              {selectedCategory}
            </span>
          </div>
        </div>
      </section>

      <div className="overflow-hidden rounded-[28px] border border-slate-600/80 bg-gradient-to-br from-slate-900/95 via-slate-950/90 to-indigo-950/45 p-3 shadow-[0_24px_70px_rgba(2,6,23,0.55)] sm:p-5">
        <div className="mb-4 flex flex-col gap-3 rounded-xl border border-slate-700/70 bg-slate-950/45 p-3 sm:flex-row sm:items-center sm:justify-between">
          <div>
            <h2 className="text-sm font-extrabold text-white">Comparación detallada</h2>
            <p className="mt-1 text-xs text-slate-400">
              {selected.length} opciones · Categoría: {selectedCategory}
            </p>
          </div>
          <p className="flex flex-wrap items-center gap-2 text-[11px] leading-relaxed text-slate-300">
            <span className="inline-flex shrink-0 items-center gap-1 rounded-full border border-cyan-200/50 bg-cyan-300 px-2.5 py-1 font-extrabold text-slate-950">
              ✦ Mejor
            </span>
            Los destacados indican el mejor valor comparable.
            {!hasComparablePrices && (
              <span className="text-amber-200">
                No se destacan precios porque usan monedas o períodos distintos.
              </span>
            )}
          </p>
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] border-separate border-spacing-1.5">
            <thead>
              <tr>
                <th className="w-44 rounded-2xl border border-white/10 bg-slate-800/90 p-4 text-left text-[10px] font-extrabold uppercase tracking-[0.18em] text-slate-300">
                  Datos del servicio
                </th>

                {selected.map((service) => (
                  <th
                    key={service.id}
                    className="rounded-2xl border border-white/10 bg-gradient-to-br from-slate-800/95 via-slate-900 to-slate-950 p-4 text-center shadow-lg shadow-black/10"
                  >
                    <div className="flex flex-col items-center gap-2.5">
                      {service.image ? (
                        <div className="rounded-2xl border border-white/10 bg-slate-950/70 p-1.5 shadow-inner">
                          <img
                            src={service.image}
                            alt={service.name}
                            className="h-16 w-20 rounded-xl object-cover"
                          />
                        </div>
                      ) : (
                        <div className="flex h-16 w-20 items-center justify-center rounded-2xl border border-slate-700 bg-slate-700/80 text-[10px] text-slate-300">
                          Sin imagen
                        </div>
                      )}

                      <div>
                        <div className="text-[10px] font-semibold uppercase tracking-[0.18em] text-cyan-300">
                          {service.provider}
                        </div>
                        <div className="mt-1 max-w-[180px] text-xs font-semibold leading-tight text-slate-100">
                          {service.name}
                        </div>
                        <div className="mt-1.5 text-sm font-bold text-emerald-300">
                          {formatServicePrice(service.monthlyPrice, service.currency)}
                          <span className="ml-1 text-[10px] font-semibold text-slate-400">
                            {getServiceBillingPeriodLabel(service.billingPeriod)}
                          </span>
                        </div>
                      </div>

                      <button
                        type="button"
                        onClick={() =>
                          navigate({
                            id: "service-detail",
                            serviceId: service.id,
                          })
                        }
                        className="inline-flex min-h-8 w-fit min-w-0 self-center items-center justify-center whitespace-nowrap rounded-lg border border-slate-600 bg-slate-700/80 px-2.5 py-1 text-[11px] font-medium text-slate-200 transition-colors hover:border-cyan-400 hover:text-cyan-200"
                      >
                        Ver detalle
                      </button>
                      <button
                        type="button"
                        disabled={serviceCart.has(service.id)}
                        onClick={() => onAddToCart(service)}
                        style={{
                          background: serviceCart.has(service.id)
                            ? "linear-gradient(135deg, #064e3b 0%, #065f46 100%)"
                            : "linear-gradient(135deg, #ff6b59 0%, #f43f5e 100%)",
                          color: serviceCart.has(service.id) ? "#a7f3d0" : "#fff",
                        }}
                        className="inline-flex min-h-8 w-fit min-w-0 self-center items-center justify-center whitespace-nowrap rounded-lg px-2.5 py-1.5 text-[11px] font-semibold shadow-md shadow-rose-500/15 transition hover:brightness-105 disabled:cursor-default disabled:shadow-none"
                      >
                        {serviceCart.has(service.id) ? "✓ En la cesta" : "Agregar al carrito"}
                      </button>
                    </div>
                  </th>
                ))}
              </tr>
            </thead>

            <tbody>
              <tr>
                <th
                  colSpan={selected.length + 1}
                  className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[10px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                >
                  Resumen y condiciones
                </th>
              </tr>
              {compareRows.map((row) => (
                <tr key={row.key}>
                  <th
                    scope="row"
                    className="rounded-xl border border-slate-700 bg-slate-800/70 p-3 text-left text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-200"
                  >
                    {row.label}
                  </th>

                  {selected.map((service, index) => {
                    const value = row.getValue(service);
                    const isBest =
                      (row.key === "monthlyPrice" && bestPriceIndices.has(index)) ||
                      (row.key === "rating" && bestRatingIndices.has(index));

                    return (
                      <td
                        key={service.id}
                        className={`rounded-xl border p-3 text-center ${
                          isBest
                            ? "border-cyan-200 bg-gradient-to-br from-cyan-300/25 via-sky-500/15 to-violet-500/20 text-white shadow-[0_0_24px_rgba(34,211,238,0.24),inset_0_0_0_1px_rgba(165,243,252,0.2)] ring-1 ring-cyan-200/30"
                            : defaultCellClass
                        }`}
                      >
                        <span className="text-xs font-semibold">{value || "No informado"}</span>
                        {isBest && (
                          <span className="mt-1 block text-[10px] font-extrabold text-cyan-100">
                            {row.key === "monthlyPrice" ? "✦ Mejor precio" : "✦ Mejor valoración"}
                          </span>
                        )}
                      </td>
                    );
                  })}
                </tr>
              ))}

              {specGroups.map((group) => (
                <Fragment key={group.title}>
                  <tr>
                    <th
                      colSpan={selected.length + 1}
                      className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[10px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                    >
                      {group.title}
                    </th>
                  </tr>
                  {group.names.map((key) => {
                    const bestIndices = getSpecBestIndices(selected, key);
                    return (
                      <tr key={key}>
                        <th
                          scope="row"
                          className="rounded-xl border border-slate-700 bg-slate-800/70 p-3 text-left text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-200"
                        >
                          {key}
                        </th>
                        {selected.map((service, index) => (
                          <td
                            key={service.id}
                            className={`rounded-xl border p-2.5 text-center ${
                              bestIndices.has(index) ? bestCellClass : defaultCellClass
                            }`}
                          >
                            <span className="text-xs font-semibold">
                              {service.specs[key] || <span className="text-slate-500">—</span>}
                            </span>
                            {bestIndices.has(index) && (
                              <span className="mt-1 block text-[9px] font-extrabold text-cyan-100">
                                ✦ Mejor valor
                              </span>
                            )}
                          </td>
                        ))}
                      </tr>
                    );
                  })}
                </Fragment>
              ))}

              <tr>
                <th
                  colSpan={selected.length + 1}
                  className="rounded-xl border border-violet-400/30 bg-gradient-to-r from-violet-500/20 via-slate-900 to-cyan-500/15 px-4 py-3 text-left text-[10px] font-extrabold uppercase tracking-[0.18em] text-cyan-200"
                >
                  Beneficios incluidos
                </th>
              </tr>
              <tr>
                <th
                  scope="row"
                  className="rounded-xl border border-slate-700 bg-slate-800/70 p-3 text-left align-top text-[10px] font-semibold uppercase tracking-[0.12em] text-slate-200"
                >
                  Beneficios
                </th>
                {selected.map((service) => (
                  <td
                    key={service.id}
                    className="rounded-xl border border-white/[0.07] bg-slate-950/35 p-3"
                  >
                    {service.benefits.length > 0 ? (
                      <ul className="space-y-1">
                        {service.benefits.map((benefit) => (
                          <li
                            key={benefit}
                            className="flex items-center gap-1.5 text-[11px] text-slate-200"
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
                        ))}
                      </ul>
                    ) : (
                      <span className="text-xs text-slate-500">—</span>
                    )}
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
