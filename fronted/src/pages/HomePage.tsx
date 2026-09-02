import { useEffect, useState } from "react";
import type { Page } from "../types";
import type { Product, Service } from "../types";
import { getProducts, getServices } from "../Services/api/frontend-src/api";
import ProductCard from "../components/ProductCard";
import ServiceCard from "../components/ServiceCard";

const productCategories = [
  { name: "Tecnología", icon: "⚡", color: "#E8001B" },
  { name: "Computación", icon: "💻", color: "#818CF8" },
  { name: "Celulares", icon: "📱", color: "#F472B6" },
  { name: "Electrodomésticos", icon: "🏠", color: "#FB923C" },
  { name: "Gaming", icon: "🎮", color: "#A78BFA" },
  { name: "Hogar", icon: "🛋️", color: "#34D399" },
];

const serviceCategories = [
  { name: "Internet", icon: "🌐", color: "#E8001B" },
  { name: "Telefonía", icon: "📡", color: "#818CF8" },
  { name: "Streaming", icon: "▶️", color: "#F472B6" },
  { name: "Seguros", icon: "🛡️", color: "#FBBF24" },
  { name: "Técnicos", icon: "🔧", color: "#60A5FA" },
  { name: "Educación", icon: "📚", color: "#34D399" },
];

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

export default function HomePage({
  navigate,
  favorites,
  compareList,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [query, setQuery] = useState("");
  const [featuredProducts, setFeaturedProducts] = useState<Product[]>([]);
  const [featuredServices, setFeaturedServices] = useState<Service[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadData() {
      setLoading(true);
      setError(null);
      try {
        const [productsRes, servicesRes] = await Promise.all([getProducts(), getServices()]);
        if (cancelled) return;
        setFeaturedProducts(productsRes.slice(0, 4));
        setFeaturedServices(servicesRes.slice(0, 4));
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : "Error al cargar los datos");
        }
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadData();
    return () => {
      cancelled = true;
    };
  }, []);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    navigate({ id: "search-products", query: query.trim() });
  };

  return (
    <div>
      {/* Hero */}
      <section
        style={{
          background:
            "radial-gradient(ellipse 80% 60% at 50% -10%, rgba(232,0,27,0.15) 0%, transparent 70%), #0A0A0A",
          borderBottom: "1px solid #2A2A2A",
        }}
        className="py-20 px-4"
      >
        <div className="max-w-3xl mx-auto text-center">
          <div
            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full mb-6"
            style={{ background: "rgba(232,0,27,0.12)", border: "1px solid rgba(232,0,27,0.3)" }}
          >
            <span className="w-1.5 h-1.5 rounded-full bg-prime animate-pulse" />
            <span className="text-xs font-semibold text-prime">
              +2.400 tiendas comparadas en tiempo real
            </span>
          </div>

          <h1 className="text-4xl md:text-6xl font-bold text-text mb-4 leading-tight tracking-tight">
            Compara precios,
            <br />
            <span className="text-prime">elige mejor.</span>
          </h1>
          <p className="text-base text-muted max-w-lg mx-auto mb-10">
            Busca productos y servicios entre cientos de tiendas y proveedores. Siempre el precio
            más bajo.
          </p>

          {/* Search */}
          <form onSubmit={handleSearch} className="flex gap-2 max-w-xl mx-auto">
            <div className="relative flex-1">
              <svg
                className="absolute left-4 top-1/2 -translate-y-1/2 text-muted"
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <circle cx="11" cy="11" r="8" />
                <path d="m21 21-4.35-4.35" />
              </svg>
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="¿Qué producto o servicio estás buscando?"
                style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                className="w-full pl-11 pr-4 py-3.5 rounded-2xl text-sm text-text placeholder-muted focus:outline-none focus:border-prime transition-colors duration-200"
              />
            </div>
            <button
              type="submit"
              style={{ background: "#E8001B", color: "#0A0A0A" }}
              className="px-6 py-3.5 rounded-2xl font-semibold text-sm hover:opacity-90 transition-opacity prime-glow whitespace-nowrap"
            >
              Buscar
            </button>
          </form>

          {/* Quick links */}
          <div className="flex flex-wrap justify-center gap-2 mt-4">
            {["RTX 4060", "iPhone 15", "Notebook gaming", "Internet hogar", "Netflix"].map((q) => (
              <button
                key={q}
                onClick={() => navigate({ id: "search-products", query: q })}
                style={{ background: "#111111", border: "1px solid #2A2A2A", color: "#64748B" }}
                className="px-3 py-1 rounded-full text-xs hover:border-prime hover:text-prime transition-all"
              >
                {q}
              </button>
            ))}
          </div>
        </div>
      </section>

      {/* Stats */}
      <section
        style={{ background: "#0A0A0A", borderBottom: "1px solid #2A2A2A" }}
        className="py-8 px-4"
      >
        <div className="max-w-7xl mx-auto grid grid-cols-2 md:grid-cols-4 gap-6 text-center">
          {[
            { value: "2.400+", label: "Tiendas y proveedores" },
            { value: "850K+", label: "Productos indexados" },
            { value: "$12.500", label: "Ahorro promedio/compra" },
            { value: "24/7", label: "Actualización de precios" },
          ].map((stat) => (
            <div key={stat.label}>
              <div className="price text-2xl font-bold text-prime">{stat.value}</div>
              <div className="text-xs text-muted mt-1">{stat.label}</div>
            </div>
          ))}
        </div>
      </section>

      <div className="max-w-7xl mx-auto px-4">
        {/* Product categories */}
        <section className="py-12">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Categorías de productos</h2>
              <p className="text-xs text-muted mt-1">Encuentra lo que buscas por categoría</p>
            </div>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-6 gap-3">
            {productCategories.map((cat) => (
              <button
                key={cat.name}
                onClick={() => navigate({ id: "search-products", query: cat.name })}
                style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                className="flex flex-col items-center gap-2 p-4 rounded-2xl hover:border-prime hover:bg-prime-muted transition-all duration-200 group"
              >
                <span className="text-2xl">{cat.icon}</span>
                <span className="text-xs font-medium text-muted-2 group-hover:text-prime transition-colors">
                  {cat.name}
                </span>
              </button>
            ))}
          </div>
        </section>

        {/* Featured products */}
        <section className="pb-12">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Productos destacados</h2>
              <p className="text-xs text-muted mt-1">Los más buscados esta semana</p>
            </div>
            <button
              onClick={() => navigate({ id: "search-products", query: "" })}
              className="text-sm text-prime hover:text-prime-dark font-medium transition-colors"
            >
              Ver todos →
            </button>
          </div>

          {loading && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {[...Array(4)].map((_, i) => (
                <div
                  key={i}
                  style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                  className="rounded-2xl h-64 animate-pulse"
                />
              ))}
            </div>
          )}

          {!loading && error && (
            <p className="text-sm text-red-400">
              No pudimos cargar los productos destacados: {error}
            </p>
          )}

          {!loading && !error && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {featuredProducts.map((p) => (
                <ProductCard
                  key={p.id}
                  product={p}
                  navigate={navigate}
                  isFavorite={favorites.has(p.id)}
                  isComparing={compareList.has(p.id)}
                  onToggleFavorite={onToggleFavorite}
                  onToggleCompare={onToggleCompare}
                />
              ))}
            </div>
          )}
        </section>

        {/* Service categories */}
        <section className="pb-12">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Categorías de servicios</h2>
              <p className="text-xs text-muted mt-1">Compara planes de internet, telefonía y más</p>
            </div>
          </div>
          <div className="grid grid-cols-3 sm:grid-cols-6 gap-3">
            {serviceCategories.map((cat) => (
              <button
                key={cat.name}
                onClick={() => navigate({ id: "search-services", query: cat.name })}
                style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                className="flex flex-col items-center gap-2 p-4 rounded-2xl hover:border-prime hover:bg-prime-muted transition-all duration-200 group"
              >
                <span className="text-2xl">{cat.icon}</span>
                <span className="text-xs font-medium text-muted-2 group-hover:text-prime transition-colors">
                  {cat.name}
                </span>
              </button>
            ))}
          </div>
        </section>

        {/* Featured services */}
        <section className="pb-16">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Servicios populares</h2>
              <p className="text-xs text-muted mt-1">Encuentra el mejor plan para ti</p>
            </div>
            <button
              onClick={() => navigate({ id: "search-services", query: "" })}
              className="text-sm text-prime hover:text-prime-dark font-medium transition-colors"
            >
              Ver todos →
            </button>
          </div>

          {loading && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {[...Array(4)].map((_, i) => (
                <div
                  key={i}
                  style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                  className="rounded-2xl h-64 animate-pulse"
                />
              ))}
            </div>
          )}

          {!loading && error && (
            <p className="text-sm text-red-400">
              No pudimos cargar los servicios destacados: {error}
            </p>
          )}

          {!loading && !error && (
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
              {featuredServices.map((s) => (
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
        </section>
      </div>
    </div>
  );
}
