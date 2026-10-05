import { useEffect, useRef, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import ServiceCard from "../../components/services/ServiceCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/common/ui";

interface Props {
  query: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string) => void;
}

type SortOption = "relevance" | "price-asc" | "price-desc" | "rating";

interface ServiceFilters {
  minimumPrice: string;
  maximumPrice: string;
  minimumDownloadSpeed: string;
  maximumDownloadSpeed: string;
  minimumUploadSpeed: string;
  maximumUploadSpeed: string;
  providers: Set<string>;
  minimumRating: string;
  installation: string;
  contract: string;
  coverage: string;
  specifications: Record<string, Set<string>>;
}

const initialFilters: ServiceFilters = {
  minimumPrice: "",
  maximumPrice: "",
  minimumDownloadSpeed: "",
  maximumDownloadSpeed: "",
  minimumUploadSpeed: "",
  maximumUploadSpeed: "",
  providers: new Set(),
  minimumRating: "",
  installation: "",
  contract: "",
  coverage: "",
  specifications: {},
};

const normalize = (value: string) =>
  value
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .toLocaleLowerCase()
    .trim();

const serviceMacroCategories = [
  {
    id: "internet",
    title: "Internet",
    description: "Internet, fibra óptica y telefonía",
    icon: "🌐",
    style: "border-cyan-200/20 bg-gradient-to-r from-cyan-950/55 via-slate-950/70 to-blue-950/35",
    accent: "text-cyan-200",
  },
  {
    id: "insurance",
    title: "Seguros",
    description: "Seguridad digital, VPN y antivirus",
    icon: "🛡️",
    style:
      "border-emerald-200/20 bg-gradient-to-r from-emerald-950/55 via-slate-950/70 to-teal-950/35",
    accent: "text-emerald-200",
  },
  {
    id: "technical",
    title: "Técnicos",
    description: "Soporte, instalación y servicios técnicos",
    icon: "🧰",
    style:
      "border-amber-200/20 bg-gradient-to-r from-amber-950/55 via-slate-950/70 to-orange-950/35",
    accent: "text-amber-200",
  },
  {
    id: "education",
    title: "Educación",
    description: "Plataformas y suscripciones educativas",
    icon: "📚",
    style:
      "border-fuchsia-200/20 bg-gradient-to-r from-fuchsia-950/50 via-slate-950/70 to-violet-950/35",
    accent: "text-fuchsia-200",
  },
] as const;

type ServiceMacroCategory = (typeof serviceMacroCategories)[number]["id"];

const getServiceMacroCategory = (service: Service): ServiceMacroCategory | "other" => {
  const category = normalize(`${service.category} ${service.subcategory}`);
  if (/internet|fibra|telefonia/.test(category)) return "internet";
  if (/segur|seguro|vpn|antivirus/.test(category)) return "insurance";
  if (/educacion|educativo|academia|curso/.test(category)) return "education";
  if (/tecnico|soporte|reparacion|instalacion/.test(category)) return "technical";
  return "other";
};

const serviceSpecFilterGroups = [
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

const getServicePrice = (service: Service) =>
  Number.isFinite(service.monthlyPrice) ? service.monthlyPrice : null;

const getPriceBasis = (service: Service) =>
  `${service.currency || "CLP"}:${service.billingPeriod || "monthly"}`;

const getSpeed = (service: Service, direction: "bajada" | "subida") => {
  const specification = Object.entries(service.specs).find(([name]) =>
    normalize(name).includes(`velocidad de ${direction}`),
  );
  if (!specification) return null;

  const speed = Number.parseFloat(specification[1].replace(",", "."));
  return Number.isFinite(speed) ? speed : null;
};

const getContract = (service: Service) =>
  service.contractPeriod?.trim() ||
  (service.contractMonths ? `${service.contractMonths} meses` : "No informado");

const getInstallation = (service: Service) =>
  service.installationCost === null
    ? "No informado"
    : service.installationCost === 0
      ? "Gratis"
      : "Con costo";

function FilterGroup({
  title,
  children,
  open = false,
}: {
  title: string;
  children: React.ReactNode;
  open?: boolean;
}) {
  return (
    <details className="border-b border-slate-200 pb-2" open={open}>
      <summary className="cursor-pointer list-none py-2 text-sm font-semibold text-slate-900">
        {title} <span className="float-right text-slate-500">⌄</span>
      </summary>
      <div className="pb-2 pt-1">{children}</div>
    </details>
  );
}

export default function ServicesPage({
  query,
  navigate,
  favorites,
  compareList,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [services, setServices] = useState<Service[]>([]);
  const [sort, setSort] = useState<SortOption>("relevance");
  const [activeMacro, setActiveMacro] = useState("all");
  const [activeCategory, setActiveCategory] = useState("all");
  const [expandedMacroCategories, setExpandedMacroCategories] = useState<Set<string>>(
    () => new Set(["internet"]),
  );
  const [filters, setFilters] = useState<ServiceFilters>(initialFilters);
  const [filtersOpen, setFiltersOpen] = useState(false);
  const [page, setPage] = useState(1);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const resultsRef = useRef<HTMLElement>(null);
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

  useEffect(() => {
    if (activeCategory !== "all") {
      resultsRef.current?.scrollIntoView({ behavior: "smooth", block: "start" });
    }
  }, [activeCategory]);

  const servicesInCategory = services.filter((service) => {
    if (activeMacro === "all") return false;
    if (activeMacro !== "all" && getServiceMacroCategory(service) !== activeMacro) return false;
    if (activeCategory !== "all" && service.category.trim() !== activeCategory) return false;
    return true;
  });

  const queryMatches = servicesInCategory.filter((service) => {
    const searchable = normalize(
      [
        service.name,
        service.provider,
        service.category,
        service.subcategory,
        service.description,
        service.coverage,
        ...Object.entries(service.specs).flat(),
      ].join(" "),
    );
    return !query || searchable.includes(normalize(query));
  });
  const isInternetCategory =
    activeMacro === "internet" ||
    normalize(activeCategory).includes("internet") ||
    normalize(activeCategory).includes("fibra");

  const providers = [
    ...new Set(queryMatches.map((service) => service.provider.trim()).filter(Boolean)),
  ].sort((first, second) => first.localeCompare(second, "es"));

  const contracts = [...new Set(queryMatches.map(getContract))].sort((first, second) =>
    first.localeCompare(second, "es", { numeric: true }),
  );
  const specificationNames = [
    ...new Set(queryMatches.flatMap((service) => Object.keys(service.specs))),
  ].sort((first, second) => first.localeCompare(second, "es"));
  const assignedSpecifications = new Set<string>();
  const specificationGroups = serviceSpecFilterGroups
    .map(({ title, pattern }) => {
      const specifications = specificationNames
        .filter((name) => {
          if (assignedSpecifications.has(name) || !pattern.test(name)) return false;
          assignedSpecifications.add(name);
          return true;
        })
        .map((name) => ({
          name,
          values: [
            ...new Set(
              queryMatches
                .map((service) => service.specs[name]?.trim())
                .filter((value): value is string => Boolean(value)),
            ),
          ].sort((first, second) => first.localeCompare(second, "es", { numeric: true })),
        }))
        .filter((specification) => specification.values.length > 0);
      return { title, specifications };
    })
    .filter((group) => group.specifications.length > 0);
  const visibleSpecificationGroups = specificationGroups
    .map((group) => ({
      ...group,
      specifications: group.specifications.filter(
        ({ name }) =>
          !isInternetCategory ||
          (!normalize(name).includes("velocidad de bajada") &&
            !normalize(name).includes("velocidad de subida")),
      ),
    }))
    .filter((group) => group.specifications.length > 0);

  const filtered = queryMatches.filter((service) => {
    const price = getServicePrice(service);
    const minimum = filters.minimumPrice ? Number(filters.minimumPrice) : null;
    const maximum = filters.maximumPrice ? Number(filters.maximumPrice) : null;
    if (minimum !== null && Number.isFinite(minimum) && (price === null || price < minimum)) {
      return false;
    }
    if (maximum !== null && Number.isFinite(maximum) && (price === null || price > maximum)) {
      return false;
    }
    const minimumDownload = filters.minimumDownloadSpeed
      ? Number(filters.minimumDownloadSpeed)
      : null;
    const maximumDownload = filters.maximumDownloadSpeed
      ? Number(filters.maximumDownloadSpeed)
      : null;
    const minimumUpload = filters.minimumUploadSpeed ? Number(filters.minimumUploadSpeed) : null;
    const maximumUpload = filters.maximumUploadSpeed ? Number(filters.maximumUploadSpeed) : null;
    const downloadSpeed = getSpeed(service, "bajada");
    const uploadSpeed = getSpeed(service, "subida");
    if (
      (minimumDownload !== null &&
        Number.isFinite(minimumDownload) &&
        (downloadSpeed === null || downloadSpeed < minimumDownload)) ||
      (maximumDownload !== null &&
        Number.isFinite(maximumDownload) &&
        (downloadSpeed === null || downloadSpeed > maximumDownload)) ||
      (minimumUpload !== null &&
        Number.isFinite(minimumUpload) &&
        (uploadSpeed === null || uploadSpeed < minimumUpload)) ||
      (maximumUpload !== null &&
        Number.isFinite(maximumUpload) &&
        (uploadSpeed === null || uploadSpeed > maximumUpload))
    ) {
      return false;
    }
    if (filters.providers.size > 0 && !filters.providers.has(service.provider)) return false;
    if (filters.minimumRating && service.rating < Number(filters.minimumRating)) {
      return false;
    }
    if (filters.installation && getInstallation(service) !== filters.installation) return false;
    if (filters.contract && getContract(service) !== filters.contract) return false;
    if (filters.coverage && !normalize(service.coverage).includes(normalize(filters.coverage))) {
      return false;
    }
    return Object.entries(filters.specifications).every(([name, values]) => {
      if (values.size === 0) return true;
      return values.has(normalize(service.specs[name] ?? ""));
    });
  });

  const sorted = [...filtered].sort((first, second) => {
    if (sort === "price-asc") {
      const basisOrder = getPriceBasis(first).localeCompare(getPriceBasis(second));
      if (basisOrder !== 0) return basisOrder;
      return (getServicePrice(first) ?? Infinity) - (getServicePrice(second) ?? Infinity);
    }
    if (sort === "price-desc") {
      const basisOrder = getPriceBasis(first).localeCompare(getPriceBasis(second));
      if (basisOrder !== 0) return basisOrder;
      return (getServicePrice(second) ?? -Infinity) - (getServicePrice(first) ?? -Infinity);
    }
    if (sort === "rating") return second.rating - first.rating;
    return 0;
  });

  const paginated = sorted.slice((page - 1) * perPage, page * perPage);
  const activeFilterCount =
    Number(Boolean(filters.minimumPrice || filters.maximumPrice)) +
    Number(Boolean(filters.minimumDownloadSpeed || filters.maximumDownloadSpeed)) +
    Number(Boolean(filters.minimumUploadSpeed || filters.maximumUploadSpeed)) +
    filters.providers.size +
    Number(Boolean(filters.minimumRating)) +
    Number(Boolean(filters.installation)) +
    Number(Boolean(filters.contract)) +
    Number(Boolean(filters.coverage)) +
    Object.values(filters.specifications).reduce((count, values) => count + values.size, 0);

  const updateFilters = (change: Partial<ServiceFilters>) => {
    setFilters((previous) => ({ ...previous, ...change }));
    setPage(1);
  };

  const clearFilters = () => {
    setFilters(initialFilters);
    setPage(1);
  };

  const toggleProvider = (provider: string) => {
    setFilters((previous) => {
      const providers = new Set(previous.providers);
      if (providers.has(provider)) providers.delete(provider);
      else providers.add(provider);
      return { ...previous, providers };
    });
    setPage(1);
  };

  const toggleSpecificationValue = (name: string, value: string) => {
    const normalizedValue = normalize(value);
    setFilters((previous) => {
      const specifications = { ...previous.specifications };
      const values = new Set(specifications[name] ?? []);
      if (values.has(normalizedValue)) values.delete(normalizedValue);
      else values.add(normalizedValue);

      if (values.size === 0) delete specifications[name];
      else specifications[name] = values;
      return { ...previous, specifications };
    });
    setPage(1);
  };

  const selectParentCategory = (category: string) => {
    setActiveMacro(category);
    setActiveCategory("all");
    if (category !== "all") {
      setExpandedMacroCategories((previous) => new Set(previous).add(category));
    }
    setFilters(initialFilters);
    setPage(1);
  };

  const renderFilters = () => (
    <div className="space-y-2">
      <p className="pb-1 text-[10px] font-bold uppercase tracking-[0.15em] text-violet-700">
        Filtros generales
      </p>
      <FilterGroup title="Precio mínimo y máximo" open>
        <div className="flex gap-2">
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Mínimo
            <input
              type="number"
              min="0"
              value={filters.minimumPrice}
              onChange={(event) => updateFilters({ minimumPrice: event.target.value })}
              placeholder="$0"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
            />
          </label>
          <label className="min-w-0 flex-1 text-xs text-slate-600">
            Máximo
            <input
              type="number"
              min="0"
              value={filters.maximumPrice}
              onChange={(event) => updateFilters({ maximumPrice: event.target.value })}
              placeholder="Sin límite"
              className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
            />
          </label>
        </div>
      </FilterGroup>

      <FilterGroup title="Proveedor">
        <div className="max-h-48 space-y-2 overflow-y-auto pb-2 pt-1">
          {providers.map((provider) => (
            <label
              key={provider}
              className="flex cursor-pointer items-center gap-2 text-sm text-slate-700"
            >
              <input
                type="checkbox"
                checked={filters.providers.has(provider)}
                onChange={() => toggleProvider(provider)}
                className="accent-prime"
              />
              {provider}
            </label>
          ))}
        </div>
      </FilterGroup>

      <FilterGroup title="Valoración mínima">
        <select
          value={filters.minimumRating}
          onChange={(event) => updateFilters({ minimumRating: event.target.value })}
          className="w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
        >
          <option value="">Cualquier valoración</option>
          <option value="4">4 estrellas o más</option>
          <option value="3">3 estrellas o más</option>
          <option value="2">2 estrellas o más</option>
        </select>
      </FilterGroup>

      <FilterGroup title="Filtros avanzados">
        <p className="pt-2 text-[10px] font-bold uppercase tracking-[0.15em] text-cyan-700">
          Filtros generales avanzados
        </p>
        <FilterGroup title="Costo de instalación">
          <select
            value={filters.installation}
            onChange={(event) => updateFilters({ installation: event.target.value })}
            className="w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
          >
            <option value="">Cualquier costo</option>
            <option value="Gratis">Instalación gratis</option>
            <option value="Con costo">Con costo</option>
            <option value="No informado">No informado</option>
          </select>
        </FilterGroup>

        <FilterGroup title="Permanencia">
          <select
            value={filters.contract}
            onChange={(event) => updateFilters({ contract: event.target.value })}
            className="w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
          >
            <option value="">Cualquier permanencia</option>
            {contracts.map((contract) => (
              <option key={contract} value={contract}>
                {contract}
              </option>
            ))}
          </select>
        </FilterGroup>

        <FilterGroup title="Cobertura">
          <input
            type="search"
            value={filters.coverage}
            onChange={(event) => updateFilters({ coverage: event.target.value })}
            placeholder="Ej.: Santiago, RM"
            className="w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 placeholder:text-slate-500 focus:border-violet-500 focus:outline-none"
          />
        </FilterGroup>

        {isInternetCategory && (
          <>
            <p className="pt-2 text-[10px] font-bold uppercase tracking-[0.15em] text-cyan-700">
              Velocidad de Internet (Mbps)
            </p>
            {(
              [
                {
                  title: "Velocidad de bajada",
                  minimum: filters.minimumDownloadSpeed,
                  maximum: filters.maximumDownloadSpeed,
                  minimumKey: "minimumDownloadSpeed",
                  maximumKey: "maximumDownloadSpeed",
                },
                {
                  title: "Velocidad de subida",
                  minimum: filters.minimumUploadSpeed,
                  maximum: filters.maximumUploadSpeed,
                  minimumKey: "minimumUploadSpeed",
                  maximumKey: "maximumUploadSpeed",
                },
              ] as const
            ).map((speedFilter) => (
              <FilterGroup key={speedFilter.title} title={speedFilter.title}>
                <div className="flex gap-2">
                  <label className="min-w-0 flex-1 text-xs text-slate-600">
                    Mínimo
                    <input
                      type="number"
                      min="0"
                      value={speedFilter.minimum}
                      onChange={(event) =>
                        updateFilters({ [speedFilter.minimumKey]: event.target.value })
                      }
                      placeholder="Mbps"
                      className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
                    />
                  </label>
                  <label className="min-w-0 flex-1 text-xs text-slate-600">
                    Máximo
                    <input
                      type="number"
                      min="0"
                      value={speedFilter.maximum}
                      onChange={(event) =>
                        updateFilters({ [speedFilter.maximumKey]: event.target.value })
                      }
                      placeholder="Mbps"
                      className="mt-1 w-full rounded-lg border border-slate-300 bg-white px-2.5 py-2 text-sm text-slate-900 focus:border-violet-500 focus:outline-none"
                    />
                  </label>
                </div>
              </FilterGroup>
            ))}
            <p className="text-[11px] leading-relaxed text-amber-700">
              Algunas velocidades de subida son estimadas y están marcadas como no verificadas.
            </p>
          </>
        )}

        {visibleSpecificationGroups.length > 0 && (
          <p className="pt-2 text-[10px] font-bold uppercase tracking-[0.15em] text-cyan-700">
            Filtros específicos de categoría
          </p>
        )}
        {visibleSpecificationGroups.map((group) => (
          <FilterGroup key={group.title} title={group.title}>
            <div className="max-h-64 space-y-3 overflow-y-auto pb-2 pt-1">
              {group.specifications.map((specification) => (
                <fieldset key={specification.name}>
                  <legend className="mb-1.5 text-xs font-semibold text-slate-600">
                    {specification.name}
                  </legend>
                  <div className="space-y-1.5">
                    {specification.values.map((value) => (
                      <label
                        key={value}
                        className="flex cursor-pointer items-start gap-2 text-xs text-slate-700"
                      >
                        <input
                          type="checkbox"
                          checked={
                            filters.specifications[specification.name]?.has(normalize(value)) ??
                            false
                          }
                          onChange={() => toggleSpecificationValue(specification.name, value)}
                          className="mt-0.5 accent-prime"
                        />
                        <span>{value}</span>
                      </label>
                    ))}
                  </div>
                </fieldset>
              ))}
            </div>
          </FilterGroup>
        ))}
      </FilterGroup>

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
          onClick={() => navigate({ id: "search-services", query: "" })}
          className="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-5 py-2 text-sm font-semibold text-white"
        >
          Ver todos los servicios
        </button>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Servicios" },
          ...(activeMacro !== "all"
            ? [
                {
                  label:
                    serviceMacroCategories.find((category) => category.id === activeMacro)?.title ??
                    activeMacro,
                },
              ]
            : []),
          ...(activeCategory !== "all" ? [{ label: activeCategory }] : []),
          ...(query ? [{ label: `"${query}"` }] : []),
        ]}
      />

      <nav aria-label="Categorías principales de servicios" className="mb-6 space-y-3">
        <button
          type="button"
          aria-pressed={activeMacro === "all" && activeCategory === "all"}
          onClick={() => selectParentCategory("all")}
          className={`rounded-full border px-4 py-2 text-sm font-semibold transition ${
            activeMacro === "all" && activeCategory === "all"
              ? "border-cyan-300 bg-cyan-300/10 text-cyan-100"
              : "border-white/15 bg-slate-950/50 text-slate-200 hover:border-white/35"
          }`}
        >
          Todos los servicios
        </button>
        {serviceMacroCategories.map((macroCategory) => {
          const isExpanded = expandedMacroCategories.has(macroCategory.id);
          const categoryCounts = services.reduce<Record<string, number>>((counts, service) => {
            if (getServiceMacroCategory(service) === macroCategory.id) {
              const category = service.category.trim();
              if (category) counts[category] = (counts[category] ?? 0) + 1;
            }
            return counts;
          }, {});
          const categories = Object.entries(categoryCounts).sort(([first], [second]) =>
            first.localeCompare(second, "es"),
          );
          const serviceCount = categories.reduce((count, [, total]) => count + total, 0);
          const panelId = `service-macro-${macroCategory.id}`;

          return (
            <section
              key={macroCategory.id}
              className={`overflow-hidden rounded-2xl border transition ${
                activeMacro === macroCategory.id ? "border-cyan-300/70 ring-1 ring-cyan-300/25" : ""
              } ${macroCategory.style}`}
            >
              <div className="flex items-center gap-2 p-3 sm:p-4">
                <button
                  type="button"
                  aria-pressed={activeMacro === macroCategory.id}
                  onClick={() => selectParentCategory(macroCategory.id)}
                  className="flex min-w-0 flex-1 items-center gap-4 rounded-xl p-1 text-left transition hover:bg-white/[0.035] focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300"
                >
                  <span className="flex h-12 w-12 shrink-0 items-center justify-center rounded-2xl border border-white/10 bg-slate-950/45 text-2xl shadow-inner">
                    {macroCategory.icon}
                  </span>
                  <span className="min-w-0 flex-1">
                    <span
                      className={`block text-lg font-extrabold sm:text-xl ${macroCategory.accent}`}
                    >
                      {macroCategory.title}
                    </span>
                    <span className="mt-1 block text-xs text-slate-300 sm:text-sm">
                      {macroCategory.description}
                    </span>
                  </span>
                  <span className="hidden shrink-0 rounded-full border border-white/10 bg-slate-950/45 px-3 py-1 text-xs font-bold text-slate-200 sm:block">
                    {categories.length} {categories.length === 1 ? "categoría" : "categorías"} ·{" "}
                    {serviceCount} {serviceCount === 1 ? "servicio" : "servicios"}
                  </span>
                </button>
                <button
                  type="button"
                  aria-expanded={isExpanded}
                  aria-controls={panelId}
                  aria-label={`${isExpanded ? "Ocultar" : "Mostrar"} subcategorías de ${macroCategory.title}`}
                  onClick={() => {
                    setExpandedMacroCategories((previous) => {
                      const next = new Set(previous);
                      if (next.has(macroCategory.id)) next.delete(macroCategory.id);
                      else next.add(macroCategory.id);
                      return next;
                    });
                  }}
                  className="flex h-10 w-10 shrink-0 items-center justify-center rounded-xl text-lg text-slate-300 transition hover:bg-white/10 hover:text-white focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300"
                >
                  <span aria-hidden="true">{isExpanded ? "−" : "+"}</span>
                </button>
              </div>

              {isExpanded && (
                <div id={panelId} className="border-t border-white/10 p-3 sm:p-4">
                  {categories.length === 0 ? (
                    <p className="px-2 py-3 text-sm text-slate-300">
                      Todavía no hay servicios disponibles en esta categoría.
                    </p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      <button
                        type="button"
                        aria-pressed={activeMacro === macroCategory.id && activeCategory === "all"}
                        onClick={() => selectParentCategory(macroCategory.id)}
                        className={`rounded-full border px-3 py-2 text-xs font-semibold transition ${
                          activeMacro === macroCategory.id && activeCategory === "all"
                            ? "border-cyan-300 bg-cyan-300/10 text-cyan-100"
                            : "border-white/15 bg-slate-950/40 text-slate-200 hover:border-white/35"
                        }`}
                      >
                        Todas ({serviceCount})
                      </button>
                      {categories.map(([category, count]) => (
                        <button
                          key={category}
                          type="button"
                          aria-pressed={
                            activeMacro === macroCategory.id && activeCategory === category
                          }
                          onClick={() => {
                            selectParentCategory(macroCategory.id);
                            setActiveCategory(category);
                          }}
                          className={`rounded-full border px-3 py-2 text-xs font-semibold transition ${
                            activeMacro === macroCategory.id && activeCategory === category
                              ? "border-cyan-300 bg-cyan-300/10 text-cyan-100"
                              : "border-white/15 bg-slate-950/40 text-slate-200 hover:border-white/35"
                          }`}
                        >
                          {category} ({count})
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}
            </section>
          );
        })}
      </nav>

      {activeMacro === "all" ? (
        <section className="rounded-2xl border border-white/10 bg-slate-950/50 px-5 py-8 text-center">
          <h1 className="text-xl font-bold text-white">Explora los servicios por categoría</h1>
          <p className="mx-auto mt-2 max-w-xl text-sm text-slate-300">
            Selecciona Internet, Seguros, Técnicos o Educación para ver los servicios y sus filtros.
          </p>
        </section>
      ) : (
        <>
          <section ref={resultsRef} className="scroll-mt-24">
            <div className="mb-6 flex flex-wrap items-start justify-between gap-3">
              <div>
                <h1 className="text-2xl font-bold text-white">
                  {activeCategory !== "all"
                    ? activeCategory
                    : serviceMacroCategories.find((category) => category.id === activeMacro)?.title}
                </h1>
                <p className="mt-1 text-sm text-slate-300">{sorted.length} servicios encontrados</p>
              </div>

              <div className="flex flex-wrap items-center gap-2">
                <button
                  type="button"
                  onClick={() => setFiltersOpen(true)}
                  className="inline-flex items-center gap-1.5 rounded-xl border border-indigo-200/20 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition-all hover:border-cyan-300 hover:text-cyan-100 lg:hidden"
                >
                  Filtros{activeFilterCount > 0 ? ` (${activeFilterCount})` : ""}
                </button>
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
              </div>
            </div>

            <div className="flex items-start gap-6">
              <aside className="sticky top-24 hidden w-56 shrink-0 self-start rounded-2xl border border-slate-200 bg-slate-50 p-5 shadow-sm lg:block">
                <h2 className="mb-4 text-sm font-semibold text-slate-900">Filtros</h2>
                {renderFilters()}
              </aside>

              {filtersOpen && (
                <div className="fixed inset-0 z-50 lg:hidden">
                  <button
                    type="button"
                    aria-label="Cerrar filtros"
                    className="absolute inset-0 bg-black/60 backdrop-blur-sm"
                    onClick={() => setFiltersOpen(false)}
                  />
                  <div className="absolute bottom-0 right-0 top-0 w-72 max-w-[90vw] overflow-y-auto border-l border-slate-200 bg-slate-50 p-6 shadow-2xl">
                    <div className="mb-5 flex items-center justify-between">
                      <h2 className="text-sm font-semibold text-slate-900">Filtros de servicios</h2>
                      <button
                        type="button"
                        aria-label="Cerrar filtros"
                        onClick={() => setFiltersOpen(false)}
                        className="text-xl leading-none text-slate-500 hover:text-slate-900"
                      >
                        ×
                      </button>
                    </div>
                    {renderFilters()}
                    <button
                      type="button"
                      onClick={() => setFiltersOpen(false)}
                      className="mt-6 w-full rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-4 py-2.5 text-sm font-bold text-white"
                    >
                      Ver {sorted.length} resultados
                    </button>
                  </div>
                </div>
              )}

              <div className="min-w-0 flex-1">
                {paginated.length === 0 ? (
                  <EmptyState
                    title="No encontramos servicios"
                    description="No hay servicios que coincidan con la búsqueda o los filtros seleccionados."
                    action={{
                      label: "Limpiar filtros",
                      onClick: clearFilters,
                    }}
                  />
                ) : (
                  <>
                    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2 xl:grid-cols-3">
                      {paginated.map((service) => (
                        <ServiceCard
                          key={service.id}
                          service={service}
                          navigate={navigate}
                          isFavorite={favorites.has(service.id)}
                          isComparing={compareList.has(service.id)}
                          onToggleFavorite={onToggleFavorite}
                          onToggleCompare={onToggleCompare}
                        />
                      ))}
                    </div>
                    <Pagination
                      page={page}
                      total={sorted.length}
                      perPage={perPage}
                      onChange={setPage}
                    />
                  </>
                )}
              </div>
            </div>
          </section>
        </>
      )}
    </div>
  );
}
