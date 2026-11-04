import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
}

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
        const firstSection = result
          .map((service) => service.subcategory.trim() || "Servicios")
          .sort((first, second) => first.localeCompare(second, "es"))[0];
        setExpandedSections(new Set(firstSection ? [firstSection] : []));
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
      .map(([name, items]) => {
        const byCategory = items.reduce((groups, service) => {
          const category = service.category.trim() || name;
          const categoryItems = groups.get(category) ?? [];
          categoryItems.push(service);
          groups.set(category, categoryItems);
          return groups;
        }, new Map<string, Service[]>());
        return {
          id: name,
          name,
          icon: sectionIcon(name),
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
                className={`category-outline relative isolate overflow-hidden rounded-xl bg-slate-700/70 p-px ${
                  isExpanded ? "category-outline-active" : ""
                }`}
              >
                <div className="relative z-10 overflow-hidden rounded-[11px] bg-slate-900/45">
                  <div className="flex items-center">
                    <button
                      type="button"
                      aria-expanded={isExpanded}
                      aria-controls={panelId}
                      aria-label={`${isExpanded ? "Ocultar" : "Ver"} subcategorías de ${section.name}`}
                      onClick={() => toggleSection(section.id)}
                      className="flex min-w-0 flex-1 items-center gap-4 p-4 text-left sm:p-5"
                    >
                      <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl bg-slate-800/80 text-2xl">
                        {section.icon}
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block text-lg font-extrabold text-slate-100 sm:text-xl">
                          {section.name}
                        </span>
                        <span className="mt-1 block text-xs text-slate-300 sm:text-sm">
                          Explora las categorías de {section.name.toLocaleLowerCase("es")}
                        </span>
                      </span>
                      <span className="hidden shrink-0 text-xs text-slate-400 sm:block">
                        {section.categories.length}{" "}
                        {section.categories.length === 1 ? "categoría" : "categorías"} ·{" "}
                        {serviceCount} {serviceCount === 1 ? "servicio" : "servicios"}
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
                    <button
                      type="button"
                      onClick={() =>
                        navigate({
                          id: "search-services",
                          query: "",
                          group: section.name,
                        })
                      }
                      className="mr-3 inline-flex shrink-0 items-center gap-1 rounded-lg border border-cyan-300/25 bg-cyan-300/[0.06] px-3 py-2 text-xs font-semibold text-cyan-100 transition hover:border-cyan-300/60 hover:bg-cyan-300/10 sm:mr-4 sm:px-3.5"
                      aria-label={`Ver todo ${section.name}`}
                    >
                      Todo {section.name}
                      <span aria-hidden="true">→</span>
                    </button>
                  </div>

                  {isExpanded && (
                    <div id={panelId} className="border-t border-slate-800 p-3 sm:p-4">
                      <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                        {section.categories.map((category) => {
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
                              className="group relative flex min-h-28 min-w-0 items-center gap-3 overflow-hidden rounded-xl border border-slate-700/70 bg-gradient-to-br from-slate-900/90 to-slate-950/70 p-3.5 text-left transition-colors duration-200 hover:border-cyan-300/70 focus-visible:border-cyan-300 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300 sm:gap-4 sm:p-4"
                            >
                              <span className="relative flex h-16 w-16 shrink-0 items-center justify-center overflow-hidden rounded-xl border border-white/[0.06] bg-slate-950/70 p-2 shadow-inner sm:h-[4.5rem] sm:w-[4.5rem]">
                                {category.image ? (
                                  <img
                                    src={category.image}
                                    alt=""
                                    loading="lazy"
                                    className="h-full w-full rounded-xl object-cover"
                                  />
                                ) : (
                                  <span className="text-3xl" aria-hidden="true">
                                    {section.icon}
                                  </span>
                                )}
                              </span>
                              <span className="relative flex min-w-0 flex-1 flex-col justify-center">
                                <span className="line-clamp-2 text-sm font-semibold leading-snug text-slate-100">
                                  {category.name}
                                </span>
                                <span className="mt-2 inline-flex w-fit items-center gap-1.5 rounded-full border border-slate-700/80 bg-slate-950/50 px-2 py-1 text-[10px] font-medium text-slate-400">
                                  <span className="h-1 w-1 rounded-full bg-cyan-300/80" />
                                  {category.count} {category.count === 1 ? "servicio" : "servicios"}
                                </span>
                              </span>
                              <span className="flex h-8 w-8 shrink-0 items-center justify-center rounded-full border border-slate-700/80 bg-slate-950/50 text-slate-500">
                                <svg
                                  width="16"
                                  height="16"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  aria-hidden="true"
                                >
                                  <path
                                    d="M5 12h14m-6-6 6 6-6 6"
                                    stroke="currentColor"
                                    strokeWidth="2"
                                    strokeLinecap="round"
                                    strokeLinejoin="round"
                                  />
                                </svg>
                              </span>
                            </button>
                          );
                        })}
                      </div>
                    </div>
                  )}
                </div>
              </section>
            );
          })}
        </div>
      )}
    </div>
  );
}
