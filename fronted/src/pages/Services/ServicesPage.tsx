import { useEffect, useRef, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import ServiceCard from "../../components/services/ServiceCard";
import { Breadcrumb, EmptyState, Pagination } from "../../components/common/ui";
import { getServiceMacroCategory } from "../../Services/utils/serviceCategories";

interface Props {
  query: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  serviceCart: Set<string>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string, category: string) => void;
  onAddServiceToCart: (service: Service) => void;
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

const serviceCategoryColorStyles = [
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
] as const;

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
  serviceCart,
  onToggleFavorite,
  onToggleCompare,
  onAddServiceToCart,
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

      <nav aria-label="Categorías principales de servicios" className="mb-7 space-y-3">
        <button
          type="button"
          aria-pressed={activeMacro === "all" && activeCategory === "all"}
          onClick={() => selectParentCategory("all")}
          className="mb-1 inline-flex items-center gap-2 rounded-lg border border-white/25 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:border-white hover:bg-white/10"
        >
          Todos los servicios
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
        {serviceMacroCategories.map((macroCategory) => {
          const isExpanded = expandedMacroCategories.has(macroCategory.id);
          const categories = Object.values(
            services.reduce<Record<string, { name: string; count: number; image: string }>>(
              (grouped, service) => {
                if (getServiceMacroCategory(service) !== macroCategory.id) return grouped;
                const name = service.category.trim();
                if (!name) return grouped;
                const category = grouped[name] ?? { name, count: 0, image: "" };
                category.count += 1;
                if (!category.image && service.image) category.image = service.image;
                grouped[name] = category;
                return grouped;
              },
              {},
            ),
          ).sort((first, second) => first.name.localeCompare(second.name, "es"));
          const serviceCount = categories.reduce((count, category) => count + category.count, 0);
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
                  onClick={() =>
                    navigate({ id: "service-macrocategory", macroCategory: macroCategory.id })
                  }
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
                <div id={panelId} className="border-t border-white/10 p-3 sm:p-5">
                  {categories.length === 0 ? (
                    <p className="px-2 py-3 text-sm text-slate-300">
                      Todavía no hay servicios disponibles en esta categoría.
                    </p>
                  ) : (
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
                      {categories.map((category, index) => {
                        const colors =
                          serviceCategoryColorStyles[index % serviceCategoryColorStyles.length];
                        const isSelected =
                          activeMacro === macroCategory.id && activeCategory === category.name;

                        return (
                          <button
                            key={category.name}
                            type="button"
                            aria-pressed={isSelected}
                            onClick={() => {
                              navigate({
                                id: "service-category",
                                macroCategory: macroCategory.id,
                                category: category.name,
                              });
                            }}
                            className={`group relative flex min-h-32 min-w-0 items-center gap-3 overflow-hidden rounded-2xl border bg-gradient-to-br ${colors.card} p-3.5 text-left shadow-lg shadow-slate-950/25 transition duration-300 hover:-translate-y-1 hover:shadow-xl hover:shadow-violet-950/35 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-cyan-300 sm:gap-4 sm:p-4 ${
                              isSelected ? "ring-1 ring-cyan-300/60" : ""
                            }`}
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
                                  {macroCategory.icon}
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
                                <svg
                                  width="14"
                                  height="14"
                                  viewBox="0 0 24 24"
                                  fill="none"
                                  aria-hidden="true"
                                  className="transition-transform group-hover:translate-x-1"
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
                            </span>
                          </button>
                        );
                      })}
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
                          isInCart={serviceCart.has(service.id)}
                          onToggleFavorite={onToggleFavorite}
                          onToggleCompare={onToggleCompare}
                          onAddToCart={onAddServiceToCart}
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
