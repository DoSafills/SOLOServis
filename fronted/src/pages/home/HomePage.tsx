import { useEffect, useState } from "react";
import { UI_TEXT } from "../../constants/uiText";
import type { Page, Product, Service } from "../../types";

import { getProducts, getServices } from "../../services/api/api";
import ProductCard from "../../components/products/ProductCard";
import ServiceCard from "../../components/services/ServiceCard";


const productCategories = [
  { name: UI_TEXT.categories.technology, icon: "⚡", color: "#E8001B" },
  { name: UI_TEXT.categories.computing, icon: "💻", color: "#818CF8" },
  { name: UI_TEXT.categories.phones, icon: "📱", color: "#F472B6" },
  { name: UI_TEXT.categories.appliances, icon: "🏠", color: "#FB923C" },
  { name: UI_TEXT.categories.gaming, icon: "🎮", color: "#A78BFA" },
  { name: UI_TEXT.categories.home, icon: "🛋️", color: "#34D399" },
];

const serviceCategories = [
  { name: UI_TEXT.categories.internet, icon: "🌐", color: "#F472B6" },
  { name: UI_TEXT.categories.insurance, icon: "🛡️", color: "#FBBF24" },
  { name: UI_TEXT.categories.technicians, icon: "🔧", color: "#60A5FA" },
  { name: UI_TEXT.categories.education, icon: "📚", color: "#34D399" },
];
interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  productCompareList: Set<string>;
  serviceCompareList: Set<string>;
  onToggleFavorite: (id: string) => void;
  onToggleProductCompare: (id: string) => void;
  onToggleServiceCompare: (id: string) => void;
}

export default function HomePage({
  navigate,
  favorites,
  productCompareList,
  serviceCompareList,
  onToggleFavorite,
  onToggleProductCompare,
  onToggleServiceCompare,
}: Props) {
  const [query, setQuery] = useState("");

  const [products, setProducts] = useState<Product[]>([]);
  const [services, setServices] = useState<Service[]>([]);

  useEffect(() => {
    Promise.all([getProducts(), getServices()])
      .then(([products, services]) => {
        setProducts(products);
        setServices(services);
      })
      .catch(console.error);
  }, []);

  const featuredProducts = products.slice(0, 4);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    navigate({ id: "search-products", query: query.trim() });
  };

  const featuredServices = services.slice(0, 4);

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
            style={{
              background: "rgba(232,0,27,0.12)",
              border: "1px solid rgba(232,0,27,0.3)",
            }}
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
                style={{
                  background: "#111111",
                  border: "1px solid #2A2A2A",
                  color: "#64748B",
                }}
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
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {featuredProducts.map((p) => (
              <ProductCard
                key={p.id}
                product={p}
                navigate={navigate}
                isFavorite={favorites.has(p.id)}
                isComparing={productCompareList.has(p.id)}
                onToggleFavorite={onToggleFavorite}
                onToggleCompare={onToggleProductCompare}
              />
            ))}
          </div>
        </section>

        {/* Service categories */}
        <section className="pb-12">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">{UI_TEXT.services.categories}</h2>
              <p className="text-xs text-muted mt-1">{UI_TEXT.services.categoryDescription}</p>
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
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
            {featuredServices.map((s) => (
              <ServiceCard
                key={s.id}
                service={s}
                navigate={navigate}
                isFavorite={favorites.has(s.id)}
                isComparing={serviceCompareList.has(s.id)}
                onToggleFavorite={onToggleFavorite}
                onToggleCompare={onToggleServiceCompare}
              />
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}


