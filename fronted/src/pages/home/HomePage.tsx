import { useEffect, useState } from "react";
import { UI_TEXT } from "../../constants/uiText";
import type { Page, Product, Service, StoreOffer } from "../../types";
import { PRODUCT_TYPES } from "../../constants/productCatalog";

import { getProducts, getServices } from "../../services/api/api";
import ProductCard from "../../components/products/ProductCard";
import ServiceCard from "../../components/services/ServiceCard";

const productCategories = PRODUCT_TYPES;

const serviceCategories = [
  { id: "internet", name: UI_TEXT.categories.internet, icon: "🌐", color: "#F472B6" },
  { id: "insurance", name: UI_TEXT.categories.insurance, icon: "🛡️", color: "#FBBF24" },
  { id: "technical", name: UI_TEXT.categories.technicians, icon: "🔧", color: "#60A5FA" },
  { id: "education", name: UI_TEXT.categories.education, icon: "📚", color: "#34D399" },
];

const featuredCategories = [
  ...productCategories.map((category) => ({ ...category, kind: "product" as const })),
  ...serviceCategories.map((category) => ({ ...category, kind: "service" as const })),
];

interface Props {
  navigate: (page: Page) => void;
  favorites: Set<string>;
  compareList: Set<string>;
  productCompareList: Set<string>;
  cartProductIds: Set<string>;
  serviceCart: Set<string>;
  onAddToCart: (product: Product, offer: StoreOffer) => void;
  onAddServiceToCart: (service: Service) => void;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string, category: string) => void;
  onToggleProductCompare: (id: string, category: string) => void;
}

export default function HomePage({
  navigate,
  favorites,
  compareList,
  productCompareList,
  cartProductIds,
  serviceCart,
  onAddToCart,
  onAddServiceToCart,
  onToggleFavorite,
  onToggleCompare,
  onToggleProductCompare,
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
            "radial-gradient(circle at top, rgba(124, 58, 237, 0.28), transparent 32%), radial-gradient(circle at 20% 20%, rgba(45, 212, 191, 0.12), transparent 28%), linear-gradient(135deg, #0b1020 0%, #0f172a 42%, #111827 100%)",
          borderBottom: "1px solid rgba(148, 163, 184, 0.2)",
        }}
        className="relative overflow-hidden py-20 px-4"
      >
        <div className="absolute inset-0 opacity-30" aria-hidden="true">
          <div className="absolute -top-20 left-1/2 h-72 w-72 -translate-x-1/2 rounded-full bg-gradient-to-r from-[#7c3aed]/35 via-[#ff6b6b]/20 to-[#22d3ee]/20 blur-3xl" />
        </div>

        <div className="relative max-w-4xl mx-auto text-center">
          <div className="mb-6 flex flex-wrap justify-center gap-2">
            {featuredCategories.map((cat) => (
              <button
                key={`${cat.name}-${cat.icon}`}
                onClick={() =>
                  cat.kind === "service"
                    ? navigate({ id: "service-macrocategory", macroCategory: cat.id })
                    : navigate({ id: "search-products", query: "", productGroup: cat.id })
                }
                style={{
                  background: "rgba(15, 23, 42, 0.72)",
                  border: "1px solid rgba(148, 163, 184, 0.25)",
                  boxShadow: "0 10px 25px rgba(15, 23, 42, 0.2)",
                }}
                className="group inline-flex items-center gap-2 rounded-full px-3 py-2 transition-all duration-200 hover:-translate-y-0.5 hover:border-[#a78bfa]"
              >
                <span
                  className="flex h-7 w-7 items-center justify-center rounded-full text-base"
                  style={{
                    background: `${cat.color}20`,
                    boxShadow: `inset 0 0 0 1px ${cat.color}40`,
                  }}
                >
                  {cat.icon}
                </span>
                <span className="text-[11px] font-semibold text-slate-100 group-hover:text-white">
                  {cat.name}
                </span>
              </button>
            ))}
          </div>

          <div
            className="inline-flex items-center gap-2 px-3 py-1.5 rounded-full mb-6"
            style={{
              background: "rgba(124, 58, 237, 0.12)",
              border: "1px solid rgba(168, 85, 247, 0.35)",
              boxShadow: "0 0 0 1px rgba(124,58,237,0.1)",
            }}
          >
            <span className="w-1.5 h-1.5 rounded-full bg-[#7c3aed] animate-pulse" />
            <span className="text-xs font-semibold text-[#c4b5fd]">
              +2.400 tiendas comparadas en tiempo real
            </span>
          </div>

          <h1 className="text-3xl md:text-5xl font-black text-white mb-4 leading-[0.95] tracking-[-0.05em]">
            Compara precios,
            <br />
            <span className="bg-gradient-to-r from-[#ff8a65] via-[#ff6b6b] to-[#8b5cf6] bg-clip-text text-transparent">
              elige mejor.
            </span>
          </h1>
          <p className="text-base text-slate-300 max-w-xl mx-auto mb-10 leading-relaxed">
            Busca productos y servicios entre cientos de tiendas y proveedores. Compara, ahorra y
            elige con confianza.
          </p>

          {/* Search */}
          <form onSubmit={handleSearch} className="flex gap-2 max-w-2xl mx-auto">
            <div className="relative flex-1">
              <svg
                className="absolute left-4 top-1/2 -translate-y-1/2 text-slate-400"
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
                style={{
                  background: "rgba(15, 23, 42, 0.8)",
                  border: "1px solid rgba(148, 163, 184, 0.25)",
                }}
                className="w-full pl-11 pr-4 py-3.5 rounded-2xl text-sm text-white placeholder-slate-400 focus:outline-none focus:border-[#8b5cf6] transition-colors duration-200 shadow-[0_12px_30px_rgba(15,23,42,0.4)]"
              />
            </div>
            <button
              type="submit"
              style={{
                background: "linear-gradient(135deg, #ff7a59 0%, #ff5f7b 32%, #8b5cf6 100%)",
                color: "#fff",
                boxShadow: "0 12px 30px rgba(139, 92, 246, 0.35)",
              }}
              className="px-6 py-3.5 rounded-2xl font-semibold text-sm hover:scale-[1.01] transition-all prime-glow whitespace-nowrap"
            >
              Buscar
            </button>
          </form>

          {/* Quick links */}
          <div className="flex flex-wrap justify-center gap-2 mt-5">
            {["RTX 4060", "iPhone 15", "Notebook gaming", "Internet hogar", "Netflix"].map((q) => (
              <button
                key={q}
                onClick={() => navigate({ id: "search-products", query: q })}
                style={{
                  background: "rgba(15, 23, 42, 0.7)",
                  border: "1px solid rgba(148, 163, 184, 0.2)",
                  color: "#cbd5e1",
                }}
                className="px-3 py-1.5 rounded-full text-xs hover:border-[#8b5cf6] hover:text-white transition-all"
              >
                {q}
              </button>
            ))}
          </div>
        </div>
      </section>

      {/* Stats */}
      <section
        style={{
          background: "linear-gradient(180deg, #0b1020 0%, #101827 100%)",
          borderBottom: "1px solid rgba(148,163,184,0.2)",
        }}
        className="py-8 px-4"
      >
        <div className="max-w-7xl mx-auto grid grid-cols-2 md:grid-cols-4 gap-4 text-center">
          {[
            { value: "2.400+", label: "Tiendas y proveedores" },
            { value: "850K+", label: "Productos indexados" },
            { value: "$12.500", label: "Ahorro promedio/compra" },
            { value: "24/7", label: "Actualización de precios" },
          ].map((stat, index) => (
            <div
              key={stat.label}
              className="rounded-2xl border border-slate-700/60 bg-slate-900/60 p-4 shadow-[0_10px_25px_rgba(15,23,42,0.2)]"
              style={{
                background:
                  index % 2 === 0
                    ? "linear-gradient(135deg, rgba(99,102,241,0.12), rgba(15,23,42,0.8))"
                    : "linear-gradient(135deg, rgba(255,107,107,0.1), rgba(15,23,42,0.8))",
              }}
            >
              <div className="price text-2xl font-bold bg-gradient-to-r from-[#8b5cf6] via-[#ff7a59] to-[#22d3ee] bg-clip-text text-transparent">
                {stat.value}
              </div>
              <div className="text-xs text-slate-300 mt-1">{stat.label}</div>
            </div>
          ))}
        </div>
      </section>

      <div className="max-w-7xl mx-auto px-4">
        {/* Featured products */}
        <section className="pb-12">
          <div className="flex items-center justify-between mb-6">
            <div>
              <h2 className="text-xl font-bold text-text">Productos destacados</h2>
              <p className="text-xs text-muted mt-1">Los más buscados esta semana</p>
            </div>
            <button
              onClick={() => navigate({ id: "search-products", query: "" })}
              className="text-sm font-medium transition-colors bg-gradient-to-r from-[#ff7a59] to-[#8b5cf6] bg-clip-text text-transparent"
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
                isInCart={cartProductIds.has(p.id)}
                onAddToCart={onAddToCart}
                onToggleFavorite={onToggleFavorite}
                onToggleCompare={onToggleProductCompare}
              />
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
              className="text-sm font-medium transition-colors bg-gradient-to-r from-[#22d3ee] to-[#8b5cf6] bg-clip-text text-transparent"
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
                isComparing={compareList.has(s.id)}
                isInCart={serviceCart.has(s.id)}
                onToggleFavorite={onToggleFavorite}
                onToggleCompare={onToggleCompare}
                onAddToCart={onAddServiceToCart}
              />
            ))}
          </div>
        </section>
      </div>
    </div>
  );
}
