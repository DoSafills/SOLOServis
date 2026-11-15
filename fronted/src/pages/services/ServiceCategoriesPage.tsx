import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import { Breadcrumb } from "../../components/common/ui";
import CategoryIcon from "../../components/common/CategoryIcon";
import { getServiceCategoryIcon } from "../../constants/productCatalog";

interface Props {
  navigate: (page: Page) => void;
}

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
        const firstSection = [
          ...new Set(result.map((service) => service.subcategory.trim() || "Servicios")),
        ].sort((first, second) => first.localeCompare(second, "es"))[0];
        setExpandedSections(firstSection ? new Set([firstSection]) : new Set());
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
          icon: getServiceCategoryIcon(name),
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
            const panelId = `service-category-section-${section.id.replace(/[^a-zA-Z0-9_-]/g, "-")}`;

            return (
              <section
                key={section.id}
                className={`category-outline relative isolate overflow-hidden rounded-xl bg-surface-3 p-px ${
                  isExpanded ? "category-outline-active" : ""
                }`}
              >
                <div className="relative z-10 overflow-hidden rounded-[11px] bg-surface">
                  <div>
                    <button
                      type="button"
                      onClick={() => toggleSection(section.id)}
                      aria-expanded={isExpanded}
                      aria-controls={panelId}
                      aria-label={`${isExpanded ? "Ocultar" : "Ver"} subcategorías de ${section.name}`}
                      className="flex min-w-0 w-full items-center gap-3 px-4 pt-4 pb-2 text-left sm:gap-4 sm:px-5 sm:pt-5"
                    >
                      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl bg-surface-2 text-prime">
                        <CategoryIcon name={section.icon} />
                      </span>
                      <span className="min-w-0 flex-1">
                        <span className="block text-base font-semibold text-text sm:text-lg">
                          {section.name}
                        </span>
                        <span className="mt-1 block text-xs text-muted sm:text-sm">
                          Explora las categorías de {section.name.toLocaleLowerCase("es")}
                        </span>
                      </span>
                      <span className="hidden shrink-0 text-xs text-muted sm:block">
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
                    <div className="pb-4 pl-[4.25rem] pr-4 sm:pb-5 sm:pl-[4.5rem] sm:pr-5">
                      <button
                        type="button"
                        onClick={() =>
                          navigate({
                            id: "search-services",
                            query: "",
                            group: section.name,
                          })
                        }
                        className="inline-flex max-w-full items-center gap-1 rounded-lg border border-prime/40 bg-prime-muted px-3 py-2 text-xs font-semibold text-prime transition hover:border-prime hover:bg-prime/20 sm:px-3.5"
                        aria-label={`Ver todos los servicios de ${section.name}`}
                      >
                        <span className="truncate">Todo {section.name}</span>
                        <span aria-hidden="true">→</span>
                      </button>
                    </div>
                  </div>

                  {isExpanded && (
                    <div id={panelId} className="border-t border-bd p-3 sm:p-4">
                      <div className="grid grid-cols-1 gap-2 sm:grid-cols-2 xl:grid-cols-3">
                        {section.categories.map((category) => {
                          const openCategory = () =>
                            navigate({
                              id: "search-services",
                              query: "",
                              category: category.name,
                            });

                          return (
                            <article
                              key={category.name}
                              role="button"
                              tabIndex={0}
                              aria-label={`Ver categoría ${category.name}`}
                              onClick={openCategory}
                              onKeyDown={(event) => {
                                if (event.key === "Enter" || event.key === " ") {
                                  event.preventDefault();
                                  openCategory();
                                }
                              }}
                              className="group relative flex min-h-40 min-w-0 cursor-pointer overflow-hidden rounded-[1.5rem] border border-bd bg-surface text-left shadow-lg shadow-black/20 transition-all duration-300 hover:-translate-y-1 hover:border-prime/50 hover:shadow-2xl hover:shadow-black/30 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime"
                            >
                              <div className="absolute inset-y-0 left-0 w-[58%] bg-surface-2">
                                {category.image ? (
                                  <img
                                    src={category.image}
                                    alt=""
                                    loading="lazy"
                                    className="h-full w-full object-cover object-center transition-transform duration-500 group-hover:scale-[1.03]"
                                  />
                                ) : (
                                  <span className="flex h-full w-full items-center justify-center text-prime">
                                    <CategoryIcon
                                      name={getServiceCategoryIcon(category.name)}
                                      className="h-8 w-8"
                                    />
                                  </span>
                                )}
                                <span
                                  aria-hidden="true"
                                  className="pointer-events-none absolute inset-0 bg-gradient-to-r from-transparent via-transparent to-slate-950/25"
                                />
                              </div>
                              <div
                                aria-hidden="true"
                                className="absolute inset-y-0 right-0 w-[66%]"
                                style={{
                                  background:
                                    "linear-gradient(135deg, #343B40 0%, #22282C 42%, #191D20 100%)",
                                  clipPath: "polygon(18% 0, 100% 0, 100% 100%, 0 100%)",
                                }}
                              />
                              <div className="relative z-10 ml-[38%] flex w-[62%] min-w-0 flex-col items-start justify-center gap-2 p-3 pl-8 sm:gap-2.5 sm:p-4 sm:pl-10">
                                <h3 className="line-clamp-2 text-sm font-bold leading-snug text-text transition-colors group-hover:text-prime sm:text-base">
                                  {category.name}
                                </h3>
                                <span className="inline-flex items-center gap-1.5 rounded-full border border-bd bg-surface-2 px-2 py-1 text-[10px] font-medium text-muted">
                                  <span
                                    aria-hidden="true"
                                    className="h-1 w-1 rounded-full bg-prime"
                                  />
                                  {category.count} {category.count === 1 ? "servicio" : "servicios"}
                                </span>
                                <span className="mt-0.5 inline-flex h-9 w-9 items-center justify-center self-end rounded-full bg-prime text-[#181714] shadow-lg shadow-black/20 transition-all group-hover:-translate-y-0.5 group-hover:bg-prime-dark group-hover:shadow-xl">
                                  <svg
                                    aria-hidden="true"
                                    viewBox="0 0 20 20"
                                    fill="none"
                                    className="h-5 w-5"
                                  >
                                    <path
                                      d="M4.5 10h11m-4.5-4.5L15.5 10 11 14.5"
                                      stroke="currentColor"
                                      strokeWidth="1.7"
                                      strokeLinecap="round"
                                      strokeLinejoin="round"
                                    />
                                  </svg>
                                </span>
                              </div>
                            </article>
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
