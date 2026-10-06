import { useEffect, useMemo, useState } from "react";
import type { Page, Service } from "../../types";
import { getServices } from "../../services/api/api";
import { getServiceMacroCategory } from "../../Services/utils/serviceCategories";
import { Breadcrumb, EmptyState } from "../../components/common/ui";

interface Props {
  macroCategory: string;
  navigate: (page: Page) => void;
}

const macroCategoryTitles: Record<string, string> = {
  internet: "Internet",
  insurance: "Seguros",
  technical: "Técnicos",
  education: "Educación",
};

const categoryCardStyles = [
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

export default function ServiceMacroCategoryPage({ macroCategory, navigate }: Props) {
  const [services, setServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const title = macroCategoryTitles[macroCategory] ?? macroCategory;

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

  const categories = useMemo(() => {
    const grouped = new Map<string, { count: number; image: string }>();
    services.forEach((service) => {
      if (getServiceMacroCategory(service) !== macroCategory) return;
      const name = service.category.trim();
      if (!name) return;
      const category = grouped.get(name) ?? { count: 0, image: "" };
      category.count += 1;
      if (!category.image && service.image) category.image = service.image;
      grouped.set(name, category);
    });
    return [...grouped.entries()]
      .map(([name, details]) => ({ name, ...details }))
      .sort((first, second) => first.name.localeCompare(second.name, "es"));
  }, [macroCategory, services]);

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
          { label: title },
        ]}
      />

      <button
        type="button"
        onClick={() => navigate({ id: "search-services", query: "" })}
        className="mb-5 inline-flex items-center gap-2 rounded-lg border border-white/25 px-4 py-2.5 text-sm font-semibold text-white transition-colors hover:border-white hover:bg-white/10"
      >
        ← Todas las categorías de servicios
      </button>

      <section className="mb-7 rounded-2xl border border-indigo-200/20 bg-gradient-to-br from-slate-900 via-indigo-950 to-slate-950 p-5 shadow-lg sm:p-7">
        <p className="text-xs font-bold uppercase tracking-[0.15em] text-cyan-200">Servicios</p>
        <h1 className="mt-1 text-2xl font-extrabold text-white sm:text-3xl">{title}</h1>
        <p className="mt-2 text-sm text-slate-300">
          Elige una microcategoría para abrir sus servicios y filtros específicos.
        </p>
      </section>

      {categories.length === 0 ? (
        <EmptyState
          title="No hay servicios en esta categoría"
          description="Vuelve a Servicios y selecciona otra macro categoría."
          action={{
            label: "Ver servicios",
            onClick: () => navigate({ id: "search-services", query: "" }),
          }}
        />
      ) : (
        <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 xl:grid-cols-3">
          {categories.map((category, index) => {
            const colors = categoryCardStyles[index % categoryCardStyles.length];

            return (
              <button
                key={category.name}
                type="button"
                onClick={() =>
                  navigate({
                    id: "service-category",
                    macroCategory,
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
                      {macroCategory === "internet"
                        ? "🌐"
                        : macroCategory === "insurance"
                          ? "🛡️"
                          : macroCategory === "technical"
                            ? "🧰"
                            : "📚"}
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
    </main>
  );
}
