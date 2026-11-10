import { useState } from "react";
import type { Page } from "../../types";

interface Props {
  navigate: (page: Page) => void;
  currentPage: Page;
  favCount: number;
  cartCount: number;
}

export default function Header({ navigate, currentPage, favCount, cartCount }: Props) {
  const [query, setQuery] = useState("");
  const [mobileOpen, setMobileOpen] = useState(false);

  const handleSearch = (e: React.FormEvent) => {
    e.preventDefault();
    if (!query.trim()) return;
    navigate({ id: "search-products", query: query.trim() });
    setQuery("");
    setMobileOpen(false);
  };

  const navLinks = [
    {
      label: "Productos",
      active: currentPage.id === "product-categories" || currentPage.id === "search-products",
      action: () => navigate({ id: "product-categories" }),
    },
    {
      label: "Servicios",
      active:
        currentPage.id === "service-categories" ||
        currentPage.id === "search-services" ||
        currentPage.id === "service-detail",
      action: () => navigate({ id: "service-categories" }),
    },
    {
      label: "Tiendas",
      active: currentPage.id === "stores" || currentPage.id === "store-detail",
      action: () => navigate({ id: "stores" }),
    },
  ];

  return (
    <header
      style={{
        background:
          "linear-gradient(110deg, rgba(7,12,27,0.98) 0%, rgba(20,18,48,0.97) 52%, rgba(8,22,43,0.98) 100%)",
        backdropFilter: "blur(18px)",
        borderBottom: "1px solid rgba(129,140,248,0.22)",
        boxShadow: "0 12px 36px rgba(2,6,23,0.24)",
      }}
      className="sticky top-0 z-50"
    >
      <div className="mx-auto max-w-7xl px-4 py-3">
        <div className="flex items-center gap-3 lg:gap-5">
          {/* Logo */}
          <button
            onClick={() => navigate({ id: "home" })}
            aria-label="SoloService, inicio"
            className="group flex shrink-0 items-center gap-2.5"
          >
            <div className="relative flex h-10 w-10 items-center justify-center overflow-hidden rounded-xl bg-gradient-to-br from-fuchsia-500 via-violet-500 to-cyan-400 shadow-lg shadow-violet-950/50 transition duration-300 group-hover:rotate-[-5deg] group-hover:scale-105">
              <svg width="21" height="21" viewBox="0 0 24 24" fill="none">
                <path
                  d="M4 5h16M4 12h10M4 19h13"
                  stroke="white"
                  strokeWidth="2.2"
                  strokeLinecap="round"
                />
                <circle cx="19" cy="19" r="3" fill="#A5F3FC" stroke="#312E81" strokeWidth="1.5" />
              </svg>
            </div>
            <span className="block text-left text-[15px] font-black leading-none tracking-tight text-white sm:text-lg">
              Solo
              <span className="bg-gradient-to-r from-fuchsia-300 via-violet-300 to-cyan-200 bg-clip-text text-transparent">
                Service
              </span>
              <span className="mt-1 hidden text-[9px] font-semibold uppercase tracking-[0.2em] text-slate-400 sm:block">
                Compara y elige mejor
              </span>
            </span>
          </button>

          {/* Search bar (desktop) */}
          <form
            onSubmit={handleSearch}
            className="hidden min-w-0 max-w-xl flex-1 items-center gap-2 md:flex"
          >
            <div className="group/search relative flex-1">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Buscar producto o servicio..."
                aria-label="Buscar producto o servicio"
                className="w-full rounded-2xl border border-indigo-200/15 bg-slate-950/65 py-2.5 pl-4 pr-12 text-sm text-white shadow-inner shadow-black/15 outline-none transition placeholder:text-slate-400 focus:border-cyan-300/60 focus:bg-slate-950/90 focus:ring-4 focus:ring-cyan-400/10"
              />
              <button
                type="submit"
                aria-label="Buscar"
                className="absolute right-1.5 top-1/2 flex h-8 w-9 -translate-y-1/2 items-center justify-center rounded-xl bg-gradient-to-br from-violet-500 to-cyan-500 text-white shadow-md shadow-violet-950/40 transition hover:brightness-110"
              >
                <svg
                  width="16"
                  height="16"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="currentColor"
                  strokeWidth="2.5"
                >
                  <circle cx="11" cy="11" r="8" />
                  <path d="m21 21-4.35-4.35" />
                </svg>
              </button>
            </div>
          </form>

          {/* Nav links (desktop) */}
          <nav
            aria-label="Navegación principal"
            className="hidden items-center gap-1 rounded-2xl border border-white/[0.07] bg-white/[0.035] p-1 lg:flex"
          >
            {navLinks.map((l) => (
              <button
                key={l.label}
                onClick={l.action}
                aria-current={l.active ? "page" : undefined}
                className={`rounded-xl px-3 py-2 text-sm font-bold transition-all duration-200 ${
                  l.active
                    ? "bg-gradient-to-r from-violet-500/25 to-cyan-400/15 text-white shadow-inner shadow-white/5"
                    : "text-slate-300 hover:bg-white/[0.07] hover:text-cyan-100"
                }`}
              >
                {l.label}
              </button>
            ))}
          </nav>

          <div className="hidden flex-1 md:block lg:hidden" />

          {/* Right icons */}
          <div className="ml-auto flex items-center gap-1.5 md:ml-0 sm:gap-2">
            {/* Cart */}
            <button
              onClick={() => navigate({ id: "cart" })}
              aria-label={`Carrito, ${cartCount} artículos`}
              title="Carrito"
              className={`relative rounded-xl border p-2.5 transition-all duration-200 ${
                currentPage.id === "cart"
                  ? "border-cyan-300/35 bg-cyan-400/15 text-cyan-100"
                  : "border-transparent text-slate-300 hover:border-cyan-200/20 hover:bg-cyan-400/10 hover:text-cyan-100"
              }`}
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <circle cx="9" cy="20" r="1" />
                <circle cx="18" cy="20" r="1" />
                <path d="M2 3h2l2.7 12.4a2 2 0 0 0 2 1.6h8.9a2 2 0 0 0 2-1.6L21 8H5" />
              </svg>
              {cartCount > 0 && (
                <span className="absolute -right-1 -top-1 flex h-[18px] min-w-[18px] items-center justify-center rounded-full border-2 border-[#10152d] bg-gradient-to-r from-fuchsia-500 to-cyan-400 px-1 text-[10px] font-black text-white">
                  {cartCount}
                </span>
              )}
            </button>

            {/* Favorites */}
            <button
              onClick={() => navigate({ id: "favorites" })}
              aria-label={`Favoritos, ${favCount} productos`}
              title="Favoritos"
              className={`relative rounded-xl border p-2.5 transition-all duration-200 ${
                currentPage.id === "favorites"
                  ? "border-fuchsia-300/35 bg-fuchsia-400/15 text-fuchsia-100"
                  : "border-transparent text-slate-300 hover:border-fuchsia-200/20 hover:bg-fuchsia-400/10 hover:text-fuchsia-100"
              }`}
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
              </svg>
              {favCount > 0 && (
                <span className="absolute -right-1 -top-1 flex h-[18px] min-w-[18px] items-center justify-center rounded-full border-2 border-[#10152d] bg-gradient-to-r from-fuchsia-500 to-violet-500 px-1 text-[10px] font-black text-white">
                  {favCount}
                </span>
              )}
            </button>

            {/* User */}
            <button
              onClick={() => navigate({ id: "user" })}
              aria-label="Mi cuenta"
              title="Mi cuenta"
              className={`rounded-xl border p-2.5 transition-all duration-200 ${
                currentPage.id === "user"
                  ? "border-violet-300/35 bg-violet-400/15 text-violet-100"
                  : "border-transparent text-slate-300 hover:border-violet-200/20 hover:bg-violet-400/10 hover:text-violet-100"
              }`}
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                <path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" />
                <circle cx="12" cy="7" r="4" />
              </svg>
            </button>

            {/* Mobile menu toggle */}
            <button
              onClick={() => setMobileOpen(!mobileOpen)}
              aria-label={mobileOpen ? "Cerrar menú" : "Abrir menú"}
              aria-expanded={mobileOpen}
              className="rounded-xl border border-indigo-200/10 p-2.5 text-slate-200 transition-all duration-200 hover:border-violet-200/30 hover:bg-violet-400/10 hover:text-white lg:hidden"
            >
              <svg
                width="18"
                height="18"
                viewBox="0 0 24 24"
                fill="none"
                stroke="currentColor"
                strokeWidth="2"
              >
                {mobileOpen ? (
                  <>
                    <path d="M18 6 6 18" />
                    <path d="m6 6 12 12" />
                  </>
                ) : (
                  <>
                    <path d="M3 12h18" />
                    <path d="M3 6h18" />
                    <path d="M3 18h18" />
                  </>
                )}
              </svg>
            </button>
          </div>
        </div>

        {/* Mobile menu */}
        {mobileOpen && (
          <div className="mt-3 space-y-3 rounded-2xl border border-indigo-200/15 bg-slate-950/80 p-3 shadow-xl shadow-black/30 lg:hidden">
            <form onSubmit={handleSearch} className="flex gap-2">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Buscar producto o servicio..."
                aria-label="Buscar producto o servicio"
                className="min-w-0 flex-1 rounded-xl border border-indigo-200/15 bg-slate-900 px-4 py-2.5 text-sm text-white outline-none placeholder:text-slate-400 focus:border-cyan-300/60 focus:ring-4 focus:ring-cyan-400/10"
              />
              <button
                type="submit"
                className="rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-4 py-2.5 text-sm font-bold text-white shadow-md shadow-violet-950/40 transition hover:brightness-110"
              >
                Buscar
              </button>
            </form>
            {navLinks.map((l) => (
              <button
                key={l.label}
                onClick={() => {
                  l.action();
                  setMobileOpen(false);
                }}
                aria-current={l.active ? "page" : undefined}
                className={`w-full rounded-xl border px-3 py-2.5 text-left text-sm font-bold transition-all ${
                  l.active
                    ? "border-violet-300/20 bg-gradient-to-r from-violet-500/20 to-cyan-400/10 text-white"
                    : "border-transparent text-slate-300 hover:bg-white/[0.06] hover:text-white"
                }`}
              >
                {l.label}
              </button>
            ))}
          </div>
        )}
      </div>
    </header>
  );
}
