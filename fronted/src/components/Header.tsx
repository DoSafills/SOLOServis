import { useState } from "react";
import type { Page } from "../types";

interface Props {
  navigate: (page: Page) => void;
  currentPage: Page;
  favCount: number;
}

export default function Header({ navigate, favCount }: Props) {
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
    { label: "Productos", action: () => navigate({ id: "search-products", query: "" }) },
    { label: "Servicios", action: () => navigate({ id: "search-services", query: "" }) },
    { label: "Tiendas", action: () => navigate({ id: "stores" }) },
  ];

  return (
    <header
      style={{
        background: "rgba(8,14,28,0.95)",
        backdropFilter: "blur(12px)",
        borderBottom: "1px solid #2A2A2A",
      }}
      className="sticky top-0 z-50"
    >
      <div className="max-w-7xl mx-auto px-4 py-3">
        <div className="flex items-center gap-4">
          {/* Logo */}
          <button
            onClick={() => navigate({ id: "home" })}
            className="flex items-center gap-2 shrink-0"
          >
            <div className="w-8 h-8 rounded-lg bg-prime flex items-center justify-center">
              <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                <path
                  d="M2 3h12M2 8h8M2 13h10"
                  stroke="#0A0A0A"
                  strokeWidth="2.5"
                  strokeLinecap="round"
                />
                <circle cx="13" cy="13" r="2" fill="#0A0A0A" />
              </svg>
            </div>
            <span className="font-bold text-lg tracking-tight text-text hidden sm:block">
              compare<span className="text-prime">ya</span>
            </span>
          </button>

          {/* Search bar (desktop) */}
          <form
            onSubmit={handleSearch}
            className="hidden md:flex flex-1 max-w-xl items-center gap-2"
          >
            <div className="relative flex-1">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Buscar producto o servicio..."
                style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                className="w-full pl-4 pr-10 py-2 rounded-xl text-sm text-text placeholder-muted focus:outline-none focus:border-prime transition-colors duration-200"
              />
              <button
                type="submit"
                className="absolute right-2 top-1/2 -translate-y-1/2 text-muted hover:text-prime transition-colors"
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
          <nav className="hidden lg:flex items-center gap-1">
            {navLinks.map((l) => (
              <button
                key={l.label}
                onClick={l.action}
                className="px-3 py-1.5 rounded-lg text-sm font-medium text-muted-2 hover:text-prime hover:bg-prime-muted transition-all duration-200"
              >
                {l.label}
              </button>
            ))}
          </nav>

          <div className="flex-1 hidden md:block lg:hidden" />

          {/* Right icons */}
          <div className="flex items-center gap-2 ml-auto md:ml-0">
            {/* Favorites */}
            <button
              onClick={() => navigate({ id: "favorites" })}
              className="relative p-2 rounded-lg text-muted-2 hover:text-prime hover:bg-prime-muted transition-all duration-200"
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
                <span className="absolute -top-0.5 -right-0.5 w-4 h-4 bg-prime text-bg text-[10px] font-bold rounded-full flex items-center justify-center">
                  {favCount}
                </span>
              )}
            </button>

            {/* User */}
            <button
              onClick={() => navigate({ id: "user" })}
              className="p-2 rounded-lg text-muted-2 hover:text-prime hover:bg-prime-muted transition-all duration-200"
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
              className="lg:hidden p-2 rounded-lg text-muted-2 hover:text-prime hover:bg-prime-muted transition-all duration-200"
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
          <div className="lg:hidden mt-3 pb-2 space-y-2">
            <form onSubmit={handleSearch} className="flex gap-2">
              <input
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder="Buscar producto o servicio..."
                style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                className="flex-1 px-4 py-2 rounded-xl text-sm text-text placeholder-muted focus:outline-none focus:border-prime transition-colors"
              />
              <button
                type="submit"
                style={{ background: "#E8001B" }}
                className="px-4 py-2 rounded-xl text-bg font-medium text-sm"
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
                className="w-full text-left px-3 py-2 rounded-lg text-sm font-medium text-muted-2 hover:text-prime hover:bg-prime-muted transition-all"
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
