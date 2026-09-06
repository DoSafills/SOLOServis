import { useEffect, useState } from "react";
import type { Page, Service } from "../types";
import { getServices } from "../Services/api/frontend-src/api";
import ServiceCard from "../components/ServiceCard";
import { Breadcrumb, EmptyState } from "../components/ui";

interface Props {
  query: string;
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
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
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [sort, setSort] = useState("relevance");
  const [selectedCats, setSelectedCats] = useState<Set<string>>(new Set());

  // Trae el catálogo completo: las categorías del sidebar y el filtrado por
  // texto/categoría se calculan en el cliente, igual que con los datos mock.
  useEffect(() => {
    let cancelled = false;

    async function loadServices() {
      setLoading(true);
      setError(null);
      try {
        const result = await getServices();
        if (cancelled) return;
        setServices(result);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Error al cargar los servicios");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadServices();

    return () => {
      cancelled = true;
    };
  }, []);

  const categories = [...new Set(services.map((s) => s.category))];

  let filtered = services.filter((s) => {
    if (
      query &&
      !s.name.toLowerCase().includes(query.toLowerCase()) &&
      !s.provider.toLowerCase().includes(query.toLowerCase()) &&
      !s.category.toLowerCase().includes(query.toLowerCase())
    )
      return false;
    if (selectedCats.size > 0 && !selectedCats.has(s.category)) return false;
    return true;
  });

  filtered = [...filtered].sort((a, b) => {
    if (sort === "price-asc") return a.monthlyPrice - b.monthlyPrice;
    if (sort === "price-desc") return b.monthlyPrice - a.monthlyPrice;
    if (sort === "rating") return b.rating - a.rating;
    return 0;
  });

  const toggleCat = (cat: string) => {
    setSelectedCats((prev) => {
      const next = new Set(prev);
      if (next.has(cat)) {
        next.delete(cat);
      } else {
        next.add(cat);
      }
      return next;
    });
  };

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Servicios" },
          ...(query ? [{ label: `"${query}"` }] : []),
        ]}
      />

      <div className="flex items-start justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-text">
            {query ? `Resultados para "${query}"` : "Todos los servicios"}
          </h1>
          <p className="text-sm text-muted mt-1">
            {loading ? "Cargando…" : `${filtered.length} servicios encontrados`}
          </p>
        </div>
        <div className="flex items-center gap-2 flex-wrap">
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value)}
            style={{ background: "#111111", border: "1px solid #2A2A2A", color: "#94A3B8" }}
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
        {/* Sidebar */}
        <aside
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="hidden lg:block w-52 shrink-0 rounded-2xl p-5 self-start sticky top-24"
        >
          <h3 className="text-sm font-semibold text-text mb-4">Categorías</h3>
          <div className="space-y-2">
            {categories.map((cat) => (
              <label key={cat} className="flex items-center gap-2 cursor-pointer">
                <input
                  type="checkbox"
                  checked={selectedCats.has(cat)}
                  onChange={() => toggleCat(cat)}
                  className="accent-prime"
                />
                <span className="text-sm text-muted-2">{cat}</span>
              </label>
            ))}
          </div>
        </aside>

        {/* Results */}
        <div className="flex-1 min-w-0">
          {error ? (
            <EmptyState
              title="No pudimos cargar los servicios"
              description={error}
              action={{ label: "Volver al inicio", onClick: () => navigate({ id: "home" }) }}
            />
          ) : loading ? (
            <div className="text-center py-20">
              <p className="text-muted">Cargando servicios…</p>
            </div>
          ) : filtered.length === 0 ? (
            <EmptyState
              title="No encontramos servicios"
              description="No hay servicios que coincidan con tu búsqueda."
              action={{
                label: "Ver todos los servicios",
                onClick: () => navigate({ id: "search-services", query: "" }),
              }}
            />
          ) : (
            <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
              {filtered.map((s) => (
                <ServiceCard
                  key={s.id}
                  service={s}
                  navigate={navigate}
                  isFavorite={favorites.has(s.id)}
                  isComparing={compareList.has(s.id)}
                  onToggleFavorite={onToggleFavorite}
                  onToggleCompare={onToggleCompare}
                />
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
