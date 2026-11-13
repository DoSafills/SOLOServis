import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
}

const categoryColors = {
  card: "from-[#22282C] via-[#191D20] to-[#191D20] border-[#343B40] hover:border-[#D1A85A]/70",
  accent: "from-[#D1A85A] to-[#B98C40]",
  image: "from-[#343B40] to-[#22282C] border-[#343B40]",
  glow: "bg-[#D1A85A]/10 group-hover:bg-[#D1A85A]/20",
  count: "border-[#343B40] bg-[#22282C] text-[#A3AAAD]",
  action:
    "border-[#343B40] bg-[#22282C] text-[#D1A85A] group-hover:border-[#D1A85A]/50 group-hover:bg-[#D1A85A]/10",
};

const sectionStyle = {
  card: "border-[#343B40] bg-gradient-to-r from-[#22282C] via-[#191D20] to-[#191D20]",
  accent: "text-[#D1A85A]",
};

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
          style: sectionStyle,
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

      <section className="mt-6 mb-7 flex flex-col gap-4 border-b border-bd pb-7 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <p className="mb-2 text-xs font-semibold uppercase tracking-[0.16em] text-prime">
            Catálogo
          </p>
          <h1 className="text-2xl font-bold text-text sm:text-3xl">
            Explora servicios por categoría
          </h1>
          <p className="mt-2 text-sm text-muted">
            Encuentra y compara ofertas de servicios por tipo.
          </p>
        </div>
        <button
          onClick={showAllServices}
          className="inline-flex items-center justify-center gap-2 self-start rounded-lg border border-prime/40 bg-prime-muted px-4 py-2.5 text-sm font-semibold text-prime transition-colors hover:border-prime hover:bg-prime/20 sm:self-auto"
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
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-bd bg-surface-2 text-2xl shadow-inner">
                    {section.icon}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span
                      className={`block text-lg font-extrabold sm:text-xl ${section.style.accent}`}
                    >
                      {section.name}
                    </span>
                    <span className="mt-1 block text-xs text-muted sm:text-sm">
                      Explora las categorías de {section.name.toLocaleLowerCase("es")}
                    </span>
                  </span>
                  <span className="hidden shrink-0 rounded-full border border-bd bg-surface-2 px-3 py-1 text-xs font-bold text-muted sm:block">
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
                    className={`shrink-0 text-muted transition-transform ${isExpanded ? "rotate-180" : ""}`}
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
                  <div id={panelId} className="border-t border-bd p-3 sm:p-5">
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      {section.categories.map((category) => {
                        const colors = categoryColors;
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
                            className={`group relative flex min-h-32 min-w-0 items-center gap-3 overflow-hidden rounded-2xl border bg-gradient-to-br ${colors.card} p-3.5 text-left shadow-lg shadow-black/25 transition duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-black/30 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime sm:gap-4 sm:p-4`}
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
                              <span className="line-clamp-2 text-base font-extrabold leading-snug text-text sm:text-lg">
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
