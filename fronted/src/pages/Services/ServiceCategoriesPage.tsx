import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
}

const categoryColors = [
  {
    card: "from-blue-950 via-slate-900 to-cyan-950 border-cyan-300/25 hover:border-cyan-200/70",
    accent: "from-cyan-300 to-blue-400",
    image: "from-cyan-400/30 to-blue-500/25 border-cyan-100/25",
    glow: "bg-cyan-400/15 group-hover:bg-cyan-300/25",
    count: "border-cyan-200/20 bg-cyan-300/10 text-cyan-100",
    action: "border-cyan-100/20 bg-cyan-300/10 text-cyan-100 group-hover:bg-cyan-300/20",
  },
  {
    card: "from-fuchsia-950 via-slate-900 to-purple-950 border-fuchsia-300/25 hover:border-fuchsia-200/70",
    accent: "from-fuchsia-300 to-violet-400",
    image: "from-fuchsia-400/30 to-purple-500/25 border-fuchsia-100/25",
    glow: "bg-fuchsia-400/15 group-hover:bg-fuchsia-300/25",
    count: "border-fuchsia-200/20 bg-fuchsia-300/10 text-fuchsia-100",
    action:
      "border-fuchsia-100/20 bg-fuchsia-300/10 text-fuchsia-100 group-hover:bg-fuchsia-300/20",
  },
  {
    card: "from-emerald-950 via-slate-900 to-teal-950 border-emerald-300/25 hover:border-emerald-200/70",
    accent: "from-emerald-300 to-teal-400",
    image: "from-emerald-400/30 to-teal-500/25 border-emerald-100/25",
    glow: "bg-emerald-400/15 group-hover:bg-emerald-300/25",
    count: "border-emerald-200/20 bg-emerald-300/10 text-emerald-100",
    action:
      "border-emerald-100/20 bg-emerald-300/10 text-emerald-100 group-hover:bg-emerald-300/20",
  },
  {
    card: "from-amber-950 via-slate-900 to-rose-950 border-amber-300/25 hover:border-amber-200/70",
    accent: "from-amber-300 to-rose-400",
    image: "from-amber-400/30 to-rose-500/25 border-amber-100/25",
    glow: "bg-amber-400/15 group-hover:bg-amber-300/25",
    count: "border-amber-200/20 bg-amber-300/10 text-amber-100",
    action: "border-amber-100/20 bg-amber-300/10 text-amber-100 group-hover:bg-amber-300/20",
  },
];

const sectionStyles = [
  {
    card: "border-cyan-200/20 bg-gradient-to-r from-cyan-950/55 via-slate-950/70 to-violet-950/35",
    accent: "text-cyan-200",
  },
  {
    card: "border-fuchsia-200/20 bg-gradient-to-r from-fuchsia-950/45 via-slate-950/70 to-indigo-950/35",
    accent: "text-fuchsia-200",
  },
  {
    card: "border-emerald-200/20 bg-gradient-to-r from-emerald-950/45 via-slate-950/70 to-teal-950/35",
    accent: "text-emerald-200",
  },
];

const sectionIcon = (name: string) => {
  const normalized = name.toLocaleLowerCase("es");
  if (/internet|telefon/.test(normalized)) return "🌐";
  if (/seguridad/.test(normalized)) return "🛡️";
  if (/educaci|curso/.test(normalized)) return "📚";
  return "✨";
};

export default function ServiceCategoriesPage({ navigate }: Props) {
  const [services, setServices] = useState<Service[]>([]);
  const [expandedSections, setExpandedSections] = useState<Set<string>>(() => new Set());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;
    getServices()
      .then((result) => {
        if (cancelled) return;
        setServices(result);
        setExpandedSections(new Set(result.map((service) => service.subcategory || "Servicios")));
      })
      .catch((err: unknown) => {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "No se pudieron cargar las categorías");
        }
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, []);

  const sections = useMemo(() => {
    const servicesByParent = services.reduce((groups, service) => {
      const parent = service.subcategory.trim() || "Servicios";
      const items = groups.get(parent) ?? [];
      items.push(service);
      groups.set(parent, items);
      return groups;
    }, new Map<string, Service[]>());

    return [...servicesByParent]
      .sort(([first], [second]) => first.localeCompare(second, "es"))
      .map(([name, items], sectionIndex) => {
        const byCategory = items.reduce((groups, service) => {
          const category = service.category.trim() || name;
          const categoryItems = groups.get(category) ?? [];
          categoryItems.push(service);
          groups.set(category, categoryItems);
          return groups;
        }, new Map<string, Service[]>());
        const style = sectionStyles[sectionIndex % sectionStyles.length];

        return {
          id: name,
          name,
          icon: sectionIcon(name),
          style,
          categories: [...byCategory]
            .sort(([first], [second]) => first.localeCompare(second, "es"))
            .map(([category, categoryServices]) => ({
              name: category,
              count: categoryServices.length,
              image: categoryServices.find((service) => service.image)?.image ?? "",
            })),
        };
      });
  }, [services]);

  const showAllServices = () => navigate({ id: "search-services", query: "" });
  const toggleSection = (sectionId: string) => {
    setExpandedSections((previous) => {
      const next = new Set(previous);
      if (next.has(sectionId)) next.delete(sectionId);
      else next.add(sectionId);
      return next;
    });
  };

  return (
    <div className="mx-auto max-w-7xl px-4 py-8 sm:py-12">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Servicios" },
        ]}
      />

      <section className="mt-6 mb-7 flex flex-col gap-4 border-b border-white/10 pb-7 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="mb-2 text-xs font-semibold uppercase tracking-[0.16em] text-cyan-300">
            Catálogo
          </p>
          <h1 className="text-2xl font-bold text-white sm:text-3xl">
            Explora servicios por categoría
          </h1>
          <p className="mt-2 text-sm text-slate-300">
            Encuentra y compara ofertas de servicios por tipo.
          </p>
        </div>
        <button
          onClick={showAllServices}
          className="inline-flex items-center justify-center gap-2 self-start rounded-lg border border-white/25 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:border-white hover:bg-white/10 sm:self-auto"
        >
          Ver todos los servicios
          <svg width="16" height="16" viewBox="0 0 24 24" fill="none" aria-hidden="true">
            <path
              d="M5 12h14m-6-6 6 6-6 6"
              stroke="currentColor"
              strokeWidth="2"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </button>
      </section>

      {loading ? (
        <p className="py-8 text-center text-sm text-muted">Cargando categorías...</p>
      ) : error ? (
        <div className="py-8 text-center">
          <p className="mb-4 text-sm text-warn">{error}</p>
          <button onClick={showAllServices} className="text-sm font-semibold text-prime">
            Ver todos los servicios
          </button>
        </div>
      ) : services.length === 0 ? (
        <div className="py-8 text-center">
          <p className="mb-4 text-sm text-muted">Todavía no hay categorías disponibles.</p>
          <button onClick={showAllServices} className="text-sm font-semibold text-prime">
            Ver todos los servicios
          </button>
        </div>
      ) : (
        <div className="space-y-4">
          {sections.map((section) => {
            const isExpanded = expandedSections.has(section.id);
            const serviceCount = section.categories.reduce(
              (sum, category) => sum + category.count,
              0,
            );
            const panelId = `service-category-section-${section.id}`;

            return (
              <section
                key={section.id}
                className={`overflow-hidden rounded-2xl border ${section.style.card}`}
              >
                <button
                  type="button"
                  aria-expanded={isExpanded}
                  aria-controls={panelId}
                  onClick={() => toggleSection(section.id)}
                  className="flex w-full items-center gap-4 p-4 text-left transition hover:bg-white/[0.035] sm:p-5"
                >
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-slate-950/45 text-2xl shadow-inner">
                    {section.icon}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span
                      className={`block text-lg font-extrabold sm:text-xl ${section.style.accent}`}
                    >
                      {section.name}
                    </span>
                    <span className="mt-1 block text-xs text-slate-300 sm:text-sm">
                      Explora las categorías de {section.name.toLocaleLowerCase("es")}
                    </span>
                  </span>
                  <span className="hidden shrink-0 rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs font-bold text-slate-200 sm:block">
                    {section.categories.length}{" "}
                    {section.categories.length === 1 ? "categoría" : "categorías"} · {serviceCount}{" "}
                    {serviceCount === 1 ? "servicio" : "servicios"}
                  </span>
                  <svg
                    width="20"
                    height="20"
                    viewBox="0 0 24 24"
                    fill="none"
                    aria-hidden="true"
                    className={`shrink-0 text-slate-300 transition-transform ${isExpanded ? "rotate-180" : ""}`}
                  >
                    <path
                      d="m6 9 6 6 6-6"
                      stroke="currentColor"
                      strokeWidth="2"
                      strokeLinecap="round"
                      strokeLinejoin="round"
                    />
                  </svg>
                </button>

                {isExpanded && (
                  <div id={panelId} className="border-t border-white/10 p-3 sm:p-5">
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      {section.categories.map((category, index) => {
                        const colors = categoryColors[index % categoryColors.length];
                        return (
                          <button
                            key={category.name}
                            type="button"
                            onClick={() =>
                              navigate({
                                id: "search-services",
                                query: "",
                                category: category.name,
                              })
                            }
                            className={`group relative flex min-h-32 min-w-0 items-center gap-3 overflow-hidden rounded-2xl border bg-gradient-to-br ${colors.card} p-3.5 text-left shadow-lg shadow-slate-950/25 transition duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-violet-950/35 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300 sm:gap-4 sm:p-4`}
                          >
                            <span
                              aria-hidden="true"
                              className={`pointer-events-none absolute -right-8 -top-10 h-36 w-36 rounded-full blur-2xl transition ${colors.glow}`}
                            />
                            <span
                              aria-hidden="true"
                              className={`absolute inset-x-0 top-0 h-1 bg-gradient-to-r ${colors.accent} opacity-80`}
                            />
                            <span
                              className={`relative flex h-[4.5rem] w-[4.5rem] shrink-0 items-center justify-center overflow-hidden rounded-2xl border bg-gradient-to-br ${colors.image} p-1.5 shadow-lg shadow-black/20 sm:h-20 sm:w-20`}
                            >
                              {category.image ? (
                                <img
                                  src={category.image}
                                  alt=""
                                  loading="lazy"
                                  className="h-full w-full rounded-xl object-cover drop-shadow-lg transition-transform duration-300 group-hover:scale-110"
                                />
                              ) : (
                                <span className="text-3xl" aria-hidden="true">
                                  {section.icon}
                                </span>
                              )}
                            </span>
                            <span className="relative flex min-w-0 flex-1 flex-col justify-center">
                              <span className="line-clamp-2 text-base font-extrabold leading-snug text-white sm:text-lg">
                                {category.name}
                              </span>
                              <span
                                className={`mt-2 inline-flex w-fit items-center rounded-full border px-2.5 py-1 text-xs font-bold ${colors.count}`}
                              >
                                {category.count} {category.count === 1 ? "servicio" : "servicios"}
                              </span>
                              <span
                                className={`mt-2.5 inline-flex w-fit items-center gap-1 rounded-lg border px-2.5 py-1 text-xs font-extrabold transition-colors ${colors.action}`}
                              >
                                Explorar
                                <span aria-hidden="true">→</span>
                              </span>
                            </span>
                          </button>
                        );
                      })}
                    </div>
                  </div>
                )}
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
