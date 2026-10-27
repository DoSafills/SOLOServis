import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import ServiceCard from "../../components/services/ServiceCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/common/ui";

interface Props {
  query: string;
  category?: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  cartServiceIds: ReadonlySet<string>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string) => void;
  onAddServiceToCart: (service: Service) => Promise<boolean>;
}

type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";

interface ServiceFilterState {
  priceMin: string;
  priceMax: string;
  selectedProviders: Set<string>;
  selectedSpecs: Record<string, Set<string>>;
  availableOnly: boolean;
}

const emptyFilters: ServiceFilterState = {
  priceMin: "",
  priceMax: "",
  selectedProviders: new Set(),
  selectedSpecs: {},
  availableOnly: false,
};

const normalizeFilterValue = (value: string) => value.trim().toLocaleLowerCase("es");

const getServiceSpecFilters = (services: Service[]) =>
  [...new Set(services.flatMap((service) => Object.keys(service.specs)))]
    .sort((a, b) => a.localeCompare(b, "es"))
    .map((name) => ({
      name,
      values: [
        ...new Set(
          services
            .map((service) => service.specs[name]?.trim())
            .filter((value): value is string => Boolean(value)),
        ),
      ].sort((a, b) => a.localeCompare(b, "es", { numeric: true })),
    }))
    .filter((spec) => spec.values.length > 0);

export default function ServicesPage({
  query,
  category,
  navigate,
  favorites,
  compareList,
  cartServiceIds,
  onToggleFavorite,
  onToggleCompare,
  onAddServiceToCart,
}: Props) {
  const [services, setServices] = useState<Service[]>([]);
  const [sort, setSort] = useState<SortOption>("relevance");
  const [activeGroup, setActiveGroup] = useState("all");
  const [activeMicrocategory, setActiveMicrocategory] = useState("all");
  const [microcategoriesByGroup, setMicrocategoriesByGroup] = useState<Record<string, string>>({
    all: "all",
  });
  const [filtersByMicrocategory, setFiltersByMicrocategory] = useState<
    Record<string, ServiceFilterState>
  >({});
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const perPage = 6;
  const filterKey =
    activeMicrocategory === "all" ? activeGroup : `${activeGroup}:${activeMicrocategory}`;
  const activeFilters = filtersByMicrocategory[filterKey] ?? emptyFilters;
  const { priceMin, priceMax, selectedProviders, selectedSpecs, availableOnly } = activeFilters;

  const updateActiveFilters = (update: Partial<ServiceFilterState>) => {
    setFiltersByMicrocategory((previous) => ({
      ...previous,
      [filterKey]: { ...(previous[filterKey] ?? emptyFilters), ...update },
    }));
    setPage(1);
  };

  const updateActiveFiltersWith = (update: (current: ServiceFilterState) => ServiceFilterState) => {
    setFiltersByMicrocategory((previous) => ({
      ...previous,
      [filterKey]: update(previous[filterKey] ?? emptyFilters),
    }));
    setPage(1);
  };

  useEffect(() => {
    let cancelled = false;
    async function loadServices() {
      setLoading(true);
      setError(null);
      try {
        const result = await getServices();
        if (!cancelled) setServices(result);
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Error al cargar los servicios");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }
    void loadServices();
    return () => {
      cancelled = true;
    };
  }, []);

  const departmentServices = services.filter((service) => {
    if (category && normalizeFilterValue(service.category) !== normalizeFilterValue(category)) {
      return false;
    }
    if (
      query &&
      !service.name.toLocaleLowerCase("es").includes(query.toLocaleLowerCase("es")) &&
      !service.provider.toLocaleLowerCase("es").includes(query.toLocaleLowerCase("es")) &&
      !service.category.toLocaleLowerCase("es").includes(query.toLocaleLowerCase("es")) &&
      !service.subcategory.toLocaleLowerCase("es").includes(query.toLocaleLowerCase("es"))
    ) {
      return false;
    }
    return true;
  });

  const groups = useMemo(
    () =>
      [...new Set(departmentServices.map((service) => service.subcategory || "Servicios"))].sort(
        (a, b) => a.localeCompare(b, "es"),
      ),
    [departmentServices],
  );
  const selectedGroup = activeGroup === "all" ? groups : [activeGroup];
  const matchingGroupServices = departmentServices.filter((service) =>
    selectedGroup.includes(service.subcategory || "Servicios"),
  );
  const microcategories = [
    ...new Set(matchingGroupServices.map((service) => service.category)),
  ].sort((a, b) => a.localeCompare(b, "es"));
  const matchingServices = matchingGroupServices.filter(
    (service) => activeMicrocategory === "all" || service.category === activeMicrocategory,
  );
  const providers = [...new Set(matchingServices.map((service) => service.provider))].sort((a, b) =>
    a.localeCompare(b, "es"),
  );
  const specFilters = getServiceSpecFilters(matchingServices);
  const minimumPrice = priceMin ? Number(priceMin) : null;
  const maximumPrice = priceMax ? Number(priceMax) : null;

  let filtered = matchingServices.filter((service) => {
    if (selectedProviders.size > 0 && !selectedProviders.has(service.provider)) return false;
    if (availableOnly && (!service.offerId || service.offerId <= 0)) return false;
    if (
      minimumPrice !== null &&
      Number.isFinite(minimumPrice) &&
      service.monthlyPrice < minimumPrice
    ) {
      return false;
    }
    if (
      maximumPrice !== null &&
      Number.isFinite(maximumPrice) &&
      service.monthlyPrice > maximumPrice
    ) {
      return false;
    }
    return Object.entries(selectedSpecs).every(([name, values]) => {
      if (values.size === 0) return true;
      const value = service.specs[name];
      return value ? values.has(normalizeFilterValue(value)) : false;
    });
  });

  filtered = [...filtered].sort((first, second) => {
    if (sort === "price-asc") return first.monthlyPrice - second.monthlyPrice;
    if (sort === "price-desc") return second.monthlyPrice - first.monthlyPrice;
    if (sort === "rating") return second.rating - first.rating;
    return 0;
  });
  const paginated = filtered.slice((page - 1) * perPage, page * perPage);
  const getFilterCount = (filters: ServiceFilterState) =>
    filters.selectedProviders.size +
    Object.values(filters.selectedSpecs).reduce((count, values) => count + values.size, 0) +
    Number(Boolean(filters.priceMin || filters.priceMax)) +
    Number(filters.availableOnly);

  const toggleProvider = (provider: string) => {
    updateActiveFiltersWith((current) => {
      const next = new Set(current.selectedProviders);
      if (next.has(provider)) next.delete(provider);
      else next.add(provider);
      return { ...current, selectedProviders: next };
    });
  };

  const toggleSpecValue = (name: string, value: string) => {
    const normalizedValue = normalizeFilterValue(value);
    updateActiveFiltersWith((current) => {
      const next = { ...current.selectedSpecs };
      const values = new Set(next[name] ?? []);
      if (values.has(normalizedValue)) values.delete(normalizedValue);
      else values.add(normalizedValue);
      if (values.size === 0) delete next[name];
      else next[name] = values;
      return { ...current, selectedSpecs: next };
    });
  };

  const clearFilters = () => updateActiveFilters({ ...emptyFilters });
  const goToAllServices = () => navigate({ id: "search-services", query: "" });
  const getGroupIcon = (name: string) => {
    const normalized = name.toLocaleLowerCase("es");
    if (/internet|telefon/.test(normalized)) return "🌐";
    if (/seguridad/.test(normalized)) return "🛡️";
    if (/educaci|curso/.test(normalized)) return "📚";
    return "✨";
  };

  const renderFilters = () => (
    <div className="space-y-2">
      <details className="border-b border-slate-200 pb-2" open>
        <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
          Precio del servicio <span className="float-right text-slate-500">⌄</span>
        </summary>
        <div className="flex gap-2 pb-2 pt-1">
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Mínimo
            <input
              type="number"
              min="0"
              value={priceMin}
              onChange={(event) => updateActiveFilters({ priceMin: event.target.value })}
              placeholder="$0"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 placeholder:text-slate-500 focus:border-violet-500 focus:outline-none"
            />
          </label>
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Máximo
            <input
              type="number"
              min="0"
              value={priceMax}
              onChange={(event) => updateActiveFilters({ priceMax: event.target.value })}
              placeholder="Sin límite"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 placeholder:text-slate-500 focus:border-violet-500 focus:outline-none"
            />
          </label>
        </div>
      </details>

      <details className="border-b border-slate-200 pb-2">
        <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
          Proveedor <span className="float-right text-slate-500">⌄</span>
        </summary>
        <div className="max-h-48 space-y-2 overflow-y-auto pb-2 pt-1">
          {providers.map((provider) => (
            <label
              key={provider}
              className="flex cursor-pointer items-center gap-2 text-sm text-slate-700"
            >
              <input
                type="checkbox"
                checked={selectedProviders.has(provider)}
                onChange={() => toggleProvider(provider)}
                className="accent-prime"
              />
              <span>{provider}</span>
            </label>
          ))}
        </div>
      </details>

      <details className="border-b border-slate-200 pb-2">
        <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
          Disponibilidad <span className="float-right text-slate-500">⌄</span>
        </summary>
        <label className="flex cursor-pointer items-center gap-2 pb-2 pt-1 text-sm text-slate-700">
          <input
            type="checkbox"
            checked={availableOnly}
            onChange={(event) => updateActiveFilters({ availableOnly: event.target.checked })}
            className="accent-prime"
          />
          Solo servicios con oferta
        </label>
      </details>

      {specFilters.length > 0 && (
        <details className="border-b border-slate-200 pb-2" open>
          <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
            Características de{" "}
            {activeMicrocategory === "all" ? "la categoría" : activeMicrocategory}
            <span className="float-right text-slate-500">⌄</span>
          </summary>
          <div className="max-h-72 space-y-3 overflow-y-auto pb-2 pt-1">
            {specFilters.map((spec) => (
              <fieldset key={spec.name}>
                <legend className="mb-1.5 text-xs font-semibold text-slate-600">{spec.name}</legend>
                <div className="space-y-1.5">
                  {spec.values.map((value) => (
                    <label
                      key={value}
                      className="flex cursor-pointer items-start gap-2 text-xs text-slate-700"
                    >
                      <input
                        type="checkbox"
                        checked={
                          selectedSpecs[spec.name]?.has(normalizeFilterValue(value)) ?? false
                        }
                        onChange={() => toggleSpecValue(spec.name, value)}
                        className="mt-0.5 accent-prime"
                      />
                      <span>{value}</span>
                    </label>
                  ))}
                </div>
              </fieldset>
            ))}
          </div>
        </details>
      )}

      {getFilterCount(activeFilters) > 0 && (
        <button
          type="button"
          onClick={clearFilters}
          className="pt-2 text-xs font-semibold text-violet-700 hover:text-violet-900"
        >
          Limpiar filtros
        </button>
      )}
    </div>
  );

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando servicios...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn mb-4">{error}</p>
        <button
          onClick={goToAllServices}
          style={{ background: "#E8001B", color: "#0A0A0A" }}
          className="px-5 py-2 rounded-xl text-sm font-semibold"
        >
          Ver todos los servicios
        </button>
      </div>
    );
  }

  const selectedGroupName = activeGroup === "all" ? "Todos los servicios" : activeGroup;
  const heading =
    category ??
    (activeMicrocategory !== "all"
      ? activeMicrocategory
      : activeGroup !== "all"
        ? `${activeGroup}${query ? `: ${query}` : ""}`
        : query
          ? `Resultados para "${query}"`
          : selectedGroupName);

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Servicios", onClick: () => navigate({ id: "service-categories" }) },
          ...(category ? [{ label: category }] : []),
          ...(query ? [{ label: `"${query}"` }] : []),
        ]}
      />

      {!category && groups.length > 0 && (
        <nav
          aria-label="Categorías principales de servicios"
          role="tablist"
          className="mb-5 flex gap-2 overflow-x-auto border-b border-white/10 pb-3"
        >
          {[{ name: "Todos", id: "all" }, ...groups.map((name) => ({ name, id: name }))].map(
            (group) => {
              const selectedMicrocategory = microcategoriesByGroup[group.id] ?? "all";
              const groupFilterKey =
                selectedMicrocategory === "all" ? group.id : `${group.id}:${selectedMicrocategory}`;
              const groupFilterCount = getFilterCount(
                filtersByMicrocategory[groupFilterKey] ?? emptyFilters,
              );
              return (
                <button
                  key={group.id}
                  type="button"
                  role="tab"
                  aria-selected={activeGroup === group.id}
                  onClick={() => {
                    setActiveGroup(group.id);
                    setActiveMicrocategory(microcategoriesByGroup[group.id] ?? "all");
                    setPage(1);
                  }}
                  className={`inline-flex shrink-0 items-center gap-2 rounded-t-lg border-b-2 px-3 py-2 text-sm font-semibold transition-colors ${
                    activeGroup === group.id
                      ? "border-prime text-prime"
                      : "border-transparent text-muted-2 hover:text-text"
                  }`}
                >
                  {group.id !== "all" && <span aria-hidden="true">{getGroupIcon(group.name)}</span>}
                  {group.name}
                  {groupFilterCount > 0 && (
                    <span className="rounded-full bg-prime/15 px-1.5 py-0.5 text-[10px] text-prime">
                      {groupFilterCount}
                    </span>
                  )}
                </button>
              );
            },
          )}
        </nav>
      )}

      {!category && microcategories.length > 0 && (
        <nav
          aria-label="Microcategorías de servicios"
          role="tablist"
          className="mb-6 flex gap-2 overflow-x-auto pb-1"
        >
          {["Todas las microcategorías", ...microcategories].map((name, index) => {
            const microcategory = index === 0 ? "all" : name;
            return (
              <button
                key={microcategory}
                type="button"
                role="tab"
                aria-selected={activeMicrocategory === microcategory}
                onClick={() => {
                  setActiveMicrocategory(microcategory);
                  setMicrocategoriesByGroup((previous) => ({
                    ...previous,
                    [activeGroup]: microcategory,
                  }));
                  setPage(1);
                }}
                className={`inline-flex shrink-0 items-center gap-2 rounded-full border px-3 py-1.5 text-xs font-medium transition-colors ${
                  activeMicrocategory === microcategory
                    ? "border-prime bg-prime/10 text-prime"
                    : "border-white/10 text-muted-2 hover:border-white/30 hover:text-text"
                }`}
              >
                {name}
                {getFilterCount(
                  filtersByMicrocategory[
                    microcategory === "all" ? activeGroup : `${activeGroup}:${microcategory}`
                  ] ?? emptyFilters,
                ) > 0 && (
                  <span className="text-[10px]">
                    {getFilterCount(
                      filtersByMicrocategory[
                        microcategory === "all" ? activeGroup : `${activeGroup}:${microcategory}`
                      ] ?? emptyFilters,
                    )}
                  </span>
                )}
              </button>
            );
          })}
        </nav>
      )}

      <div className="flex items-start justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-text">
            {query && !category ? `Resultados para "${query}"` : heading}
          </h1>
          <p className="text-sm text-muted mt-1">{filtered.length} servicios encontrados</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          <button
            type="button"
            onClick={() => setFiltersOpen(true)}
            style={{ background: "#111111", border: "1px solid #2A2A2A" }}
            className="lg:hidden flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm text-muted-2 hover:border-prime hover:text-prime transition-all"
          >
            <svg
              width="14"
              height="14"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
              aria-hidden="true"
            >
              <line x1="4" y1="6" x2="11" y2="6" />
              <line x1="8" y1="2" x2="8" y2="10" />
              <line x1="4" y1="18" x2="11" y2="18" />
              <line x1="8" y1="14" x2="8" y2="22" />
              <line x1="13" y1="12" x2="20" y2="12" />
              <line x1="16" y1="8" x2="16" y2="16" />
            </svg>
            Filtros
          </button>
          <select
            value={sort}
            onChange={(event) => {
              setSort(event.target.value as SortOption);
              setPage(1);
            }}
            style={{
              background: "#111111",
              border: "1px solid #2A2A2A",
              color: "#94A3B8",
            }}
            className="px-3 py-2 rounded-xl text-sm focus:outline-none focus:border-prime transition-colors"
          >
            <option value="relevance">Relevancia</option>
            <option value="price-asc">Precio: menor a mayor</option>
            <option value="price-desc">Precio: mayor a menor</option>
            <option value="rating">Mejor valoración</option>
          </select>
          {compareList.size >= 2 && (
            <button
              onClick={() => navigate({ id: "service-comparison", serviceIds: [...compareList] })}
              style={{ background: "#E8001B", color: "#0A0A0A" }}
              className="px-4 py-2 rounded-xl text-sm font-semibold hover:opacity-90 transition-opacity"
            >
              Comparar {compareList.size} servicios
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-6">
        <aside className="hidden lg:block w-56 shrink-0 rounded-2xl border border-slate-200 bg-slate-50 p-5 self-start sticky top-24 shadow-sm">
          <h3 className="text-sm font-semibold text-slate-900 mb-5">Filtros</h3>
          {renderFilters()}
        </aside>

        {filtersOpen && (
          <div className="fixed inset-0 z-50 lg:hidden">
            <div
              className="absolute inset-0 bg-black/60 backdrop-blur-sm"
              onClick={() => setFiltersOpen(false)}
            />
            <div className="absolute right-0 top-0 bottom-0 w-72 border-l border-slate-200 bg-slate-50 p-6 overflow-y-auto shadow-2xl">
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-sm font-semibold text-slate-900">Filtros</h3>
                <button
                  type="button"
                  onClick={() => setFiltersOpen(false)}
                  aria-label="Cerrar filtros"
                  className="text-slate-500 hover:text-slate-900 transition-colors"
                >
                  <svg
                    width="18"
                    height="18"
                    viewBox="0 0 24 24"
                    fill="none"
                    stroke="currentColor"
                    strokeWidth="2"
                    aria-hidden="true"
                  >
                    <path d="M18 6 6 18" />
                    <path d="m6 6 12 12" />
                  </svg>
                </button>
              </div>
              {renderFilters()}
            </div>
          </div>
        )}

        <div className="flex-1 min-w-0">
          {paginated.length === 0 ? (
            <EmptyState
              title="No encontramos servicios"
              description={`No hay servicios que coincidan con "${query}". Intenta con otros términos o elimina algunos filtros.`}
              action={{ label: "Modificar búsqueda", onClick: () => navigate({ id: "home" }) }}
            />
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
                {paginated.map((service) => (
                  <ServiceCard
                    key={service.id}
                    service={service}
                    navigate={navigate}
                    isFavorite={favorites.has(service.id)}
                    isComparing={compareList.has(service.id)}
                    isInCart={cartServiceIds.has(service.id)}
                    onToggleFavorite={onToggleFavorite}
                    onToggleCompare={onToggleCompare}
                    onAddToCart={onAddServiceToCart}
                  />
                ))}
              </div>
              <Pagination
                page={page}
                total={filtered.length}
                perPage={perPage}
                onChange={setPage}
              />
            </>
          )}
        </div>
      </div>
    </div>
  );
}
