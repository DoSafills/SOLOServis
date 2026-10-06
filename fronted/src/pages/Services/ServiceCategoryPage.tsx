import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import ServiceCard from "../../components/services/ServiceCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/common/ui";
import { getServiceMacroCategory } from "../../Services/utils/serviceCategories";

interface Props {
  macroCategory: string;
  category: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  serviceCart: Set<string>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string, category: string) => void;
  onAddServiceToCart: (service: Service) => void;
}

type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";

const normalize = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase()
    .trim();

const formatMacroCategory = (id: string) => {
  switch (id) {
    case "internet":
      return "Internet";
    case "insurance":
      return "Seguros";
    case "technical":
      return "Técnicos";
    case "education":
      return "Educación";
    default:
      return id;
  }
};

export default function ServiceCategoryPage({
  macroCategory,
  category,
  navigate,
  favorites,
  compareList,
  serviceCart,
  onToggleFavorite,
  onToggleCompare,
  onAddServiceToCart,
}: Props) {
  const [services, setServices] = useState<Service[]>([]);
  const [selectedSpecs, setSelectedSpecs] = useState<Record<string, Set<string>>>({});
  const [minimumPrice, setMinimumPrice] = useState("");
  const [maximumPrice, setMaximumPrice] = useState("");
  const [sort, setSort] = useState<SortOption>("relevance");
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const perPage = 6;

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

  const categoryServices = useMemo(
    () =>
      services.filter(
        (service) =>
          service.category.trim() === category &&
          getServiceMacroCategory(service) === macroCategory,
      ),
    [category, macroCategory, services],
  );
  const selectedServices = categoryServices;

  const specifications = useMemo(
    () =>
      [...new Set(selectedServices.flatMap((service) => Object.keys(service.specs)))].sort(
        (first, second) => first.localeCompare(second, "es"),
      ),
    [selectedServices],
  );

  const specificationOptions = useMemo(
    () =>
      Object.fromEntries(
        specifications.map((name) => [
          name,
          [
            ...new Set(
              selectedServices
                .map((service) => service.specs[name]?.trim())
                .filter((value): value is string => Boolean(value)),
            ),
          ].sort((first, second) => first.localeCompare(second, "es", { numeric: true })),
        ]),
      ),
    [selectedServices, specifications],
  );

  const filteredServices = selectedServices.filter((service) => {
    const price = Number.isFinite(service.monthlyPrice) ? service.monthlyPrice : null;
    const minimum = minimumPrice ? Number(minimumPrice) : null;
    const maximum = maximumPrice ? Number(maximumPrice) : null;
    if (minimum !== null && Number.isFinite(minimum) && (price === null || price < minimum)) {
      return false;
    }
    if (maximum !== null && Number.isFinite(maximum) && (price === null || price > maximum)) {
      return false;
    }

    return Object.entries(selectedSpecs).every(([name, values]) => {
      if (values.size === 0) return true;
      return values.has(normalize(service.specs[name] ?? ""));
    });
  });

  const sortedServices = [...filteredServices].sort((first, second) => {
    if (sort === "price-asc") return first.monthlyPrice - second.monthlyPrice;
    if (sort === "price-desc") return second.monthlyPrice - first.monthlyPrice;
    if (sort === "rating") return second.rating - first.rating;
    return 0;
  });
  const paginatedServices = sortedServices.slice((page - 1) * perPage, page * perPage);
  const activeFilterCount =
    Number(Boolean(minimumPrice || maximumPrice)) +
    Object.values(selectedSpecs).reduce((count, values) => count + values.size, 0);

  const toggleSpecification = (name: string, value: string) => {
    const normalizedValue = normalize(value);
    setSelectedSpecs((previous) => {
      const next = { ...previous };
      const values = new Set(next[name] ?? []);
      if (values.has(normalizedValue)) values.delete(normalizedValue);
      else values.add(normalizedValue);
      if (values.size === 0) delete next[name];
      else next[name] = values;
      return next;
    });
    setPage(1);
  };

  const clearFilters = () => {
    setSelectedSpecs({});
    setMinimumPrice("");
    setMaximumPrice("");
    setPage(1);
  };

  const renderFilters = () => (
    <div className="space-y-3">
      <h2 className="text-sm font-bold text-slate-900">Filtros de {category}</h2>
      <fieldset>
        <legend className="mb-2 text-xs font-semibold text-slate-700">Precio</legend>
        <div className="flex gap-2">
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Mínimo
            <input
              type="number"
              min="0"
              value={minimumPrice}
              onChange={(event) => {
                setMinimumPrice(event.target.value);
                setPage(1);
              }}
              placeholder="$0"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
            />
          </label>
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Máximo
            <input
              type="number"
              min="0"
              value={maximumPrice}
              onChange={(event) => {
                setMaximumPrice(event.target.value);
                setPage(1);
              }}
              placeholder="Sin límite"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
            />
          </label>
        </div>
      </fieldset>

      {specifications.map((name) => (
        <fieldset key={name} className="border-t border-slate-200 pt-3">
          <legend className="text-xs font-semibold text-slate-700">{name}</legend>
          <div className="mt-2 max-h-40 space-y-2 overflow-y-auto">
            {specificationOptions[name].map((value) => (
              <label key={value} className="flex cursor-pointer items-start gap-2 text-xs text-slate-700">
                <input
                  type="checkbox"
                  checked={selectedSpecs[name]?.has(normalize(value)) ?? false}
                  onChange={() => toggleSpecification(name, value)}
                  className="mt-0.5 accent-prime"
                />
                <span>{value}</span>
              </label>
            ))}
          </div>
        </fieldset>
      ))}

      {activeFilterCount > 0 && (
        <button
          type="button"
          onClick={clearFilters}
          className="pt-2 text-xs font-semibold text-violet-700 hover:text-violet-900"
        >
          Limpiar filtros ({activeFilterCount})
        </button>
      )}
    </div>
  );

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="text-muted">Cargando servicios...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-20 text-center">
        <p className="text-warn mb-4">{error}</p>
        <button
          type="button"
          onClick={() => navigate({ id: "search-services", query: "" })}
          className="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-5 py-2 text-sm font-semibold text-white"
        >
          Volver a Servicios
        </button>
      </div>
    );
  }

  return (
    <main className="mx-auto max-w-7xl px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Servicios", onClick: () => navigate({ id: "search-services", query: "" }) },
          { label: formatMacroCategory(macroCategory) },
          { label: category },
        ]}
      />

      <button
        type="button"
        onClick={() => navigate({ id: "service-macrocategory", macroCategory })}
        className="mb-5 inline-flex items-center gap-2 rounded-lg border border-white/25 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:border-white hover:bg-white/10"
      >
        ← Todas las categorías de servicios
      </button>

      <section className="mb-7 rounded-2xl border border-indigo-200/20 bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-950 p-5 shadow-lg sm:p-7">
        <p className="text-xs font-bold uppercase tracking-[0.15em] text-cyan-200">
          {formatMacroCategory(macroCategory)}
        </p>
        <h1 className="mt-1 text-2xl font-extrabold text-white sm:text-3xl">{category}</h1>
        <p className="mt-2 text-sm text-slate-300">
          {filteredServices.length} servicios disponibles
        </p>
      </section>

      {categoryServices.length === 0 ? (
        <EmptyState
          title="No encontramos servicios en esta categoría"
          description="Vuelve a Servicios y selecciona otra categoría."
          action={{
            label: "Ver servicios",
            onClick: () => navigate({ id: "search-services", query: "" }),
          }}
        />
      ) : (
        <>
          <div className="mb-5 flex flex-wrap items-center justify-between gap-3">
            <p className="text-sm text-slate-300">
              {filteredServices.length} servicios encontrados
            </p>
            <label className="flex items-center gap-2 text-sm text-slate-200">
              Ordenar
              <select
                value={sort}
                onChange={(event) => {
                  setSort(event.target.value as SortOption);
                  setPage(1);
                }}
                className="rounded-xl border border-indigo-200/20 bg-slate-900 px-3 py-2 text-sm text-slate-200 focus:border-cyan-300 focus:outline-none"
              >
                <option value="relevance">Relevancia</option>
                <option value="price-asc">Precio: menor a mayor</option>
                <option value="price-desc">Precio: mayor a menor</option>
                <option value="rating">Mejor valoración</option>
              </select>
            </label>
          </div>

          <details className="mb-5 rounded-2xl border border-slate-200 bg-slate-50 p-4 lg:hidden">
            <summary className="cursor-pointer text-sm font-semibold text-slate-900">
              Filtros específicos{activeFilterCount ? ` (${activeFilterCount})` : ""}
            </summary>
            <div className="pt-4">{renderFilters()}</div>
          </details>

          <div className="flex items-start gap-6">
            <aside className="sticky top-24 hidden w-64 shrink-0 self-start rounded-2xl border border-slate-200 bg-slate-50 p-5 shadow-sm lg:block">
              {renderFilters()}
            </aside>
            {paginatedServices.length === 0 ? (
              <div className="flex-1">
                <EmptyState
                  title="No hay servicios que coincidan con los filtros"
                  description="Prueba quitando algunos filtros o selecciona otra microcategoría."
                  action={{ label: "Limpiar filtros", onClick: clearFilters }}
                />
              </div>
            ) : (
              <div className="min-w-0 flex-1">
                <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
                  {paginatedServices.map((service) => (
                    <ServiceCard
                      key={service.id}
                      service={service}
                      navigate={navigate}
                      isFavorite={favorites.has(service.id)}
                      isComparing={compareList.has(service.id)}
                      isInCart={serviceCart.has(service.id)}
                      onToggleFavorite={onToggleFavorite}
                      onToggleCompare={onToggleCompare}
                      onAddToCart={onAddServiceToCart}
                    />
                  ))}
                </div>
                <Pagination
                  page={page}
                  total={filteredServices.length}
                  perPage={perPage}
                  onChange={setPage}
                />
              </div>
            )}
          </div>
        </>
      )}
    </main>
  );
}
