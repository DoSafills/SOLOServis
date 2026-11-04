import { useState } from "react";
import type { FormEvent } from "react";
import type { Page } from "../../types";
import type { AuthUser, SearchHistoryEntry } from "../../services/api/personalization";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  navigate: (page: Page) => void;
  user: AuthUser | null;
  favoritesCount: number;
  history: SearchHistoryEntry[];
  onLogin: (email: string, password: string) => Promise<void>;
  onRegister: (name: string, email: string, password: string) => Promise<void>;
  onLogout: () => Promise<void>;
}

type Tab = "profile" | "favorites" | "history";

export default function UserPage({
  navigate,
  user,
  favoritesCount,
  history,
  onLogin,
  onRegister,
  onLogout,
}: Props) {
  const [activeTab, setActiveTab] = useState<Tab>("profile");
  const [mode, setMode] = useState<"login" | "register">("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);
    setSubmitting(true);
    try {
      if (mode === "register") await onRegister(name.trim(), email.trim(), password);
      else await onLogin(email.trim(), password);
      setPassword("");
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "No se pudo iniciar la sesión.");
    } finally {
      setSubmitting(false);
    }
  };

  const tabs: { id: Tab; label: string }[] = [
    { id: "profile", label: "Perfil" },
    { id: "favorites", label: `Favoritos${favoritesCount ? ` (${favoritesCount})` : ""}` },
    { id: "history", label: "Historial" },
  ];

  return (
    <div className="mx-auto max-w-7xl px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Mi cuenta" },
        ]}
      />

      {!user ? (
        <section className="mx-auto mt-8 max-w-lg rounded-3xl border border-violet-200/15 bg-slate-950/75 p-6 shadow-xl shadow-violet-950/20 sm:p-8">
          <div className="mb-6">
            <p className="text-xs font-bold uppercase tracking-[0.2em] text-cyan-300">
              SoloService
            </p>
            <h1 className="mt-2 text-2xl font-black text-white">
              {mode === "login" ? "Inicia sesión" : "Crea tu cuenta"}
            </h1>
            <p className="mt-2 text-sm leading-6 text-slate-300">
              Puedes explorar productos, servicios y tiendas sin una cuenta. Inicia sesión para
              guardar favoritos y consultar tu historial.
            </p>
          </div>

          <div className="mb-5 grid grid-cols-2 rounded-xl border border-white/10 bg-slate-900/70 p-1">
            <button
              type="button"
              onClick={() => {
                setMode("login");
                setFormError(null);
              }}
              className={`rounded-lg px-3 py-2 text-sm font-bold transition ${
                mode === "login" ? "bg-violet-500/25 text-white" : "text-slate-400"
              }`}
            >
              Iniciar sesión
            </button>
            <button
              type="button"
              onClick={() => {
                setMode("register");
                setFormError(null);
              }}
              className={`rounded-lg px-3 py-2 text-sm font-bold transition ${
                mode === "register" ? "bg-cyan-500/20 text-white" : "text-slate-400"
              }`}
            >
              Crear cuenta
            </button>
          </div>

          <form onSubmit={submit} className="space-y-4">
            {mode === "register" && (
              <label className="block text-sm font-semibold text-slate-200">
                Nombre
                <input
                  required
                  autoComplete="name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  minLength={2}
                  maxLength={150}
                  className="mt-1.5 w-full rounded-xl border border-white/10 bg-slate-900 px-4 py-3 text-white outline-none transition placeholder:text-slate-500 focus:border-cyan-300/60"
                  placeholder="Tu nombre"
                />
              </label>
            )}
            <label className="block text-sm font-semibold text-slate-200">
              Correo electrónico
              <input
                required
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                className="mt-1.5 w-full rounded-xl border border-white/10 bg-slate-900 px-4 py-3 text-white outline-none transition placeholder:text-slate-500 focus:border-cyan-300/60"
                placeholder="tu@correo.com"
              />
            </label>
            <label className="block text-sm font-semibold text-slate-200">
              Contraseña
              <input
                required
                type="password"
                autoComplete={mode === "register" ? "new-password" : "current-password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                minLength={mode === "register" ? 10 : undefined}
                className="mt-1.5 w-full rounded-xl border border-white/10 bg-slate-900 px-4 py-3 text-white outline-none transition placeholder:text-slate-500 focus:border-cyan-300/60"
                placeholder={mode === "register" ? "Al menos 10 caracteres" : "Tu contraseña"}
              />
            </label>
            {formError && (
              <p
                role="alert"
                className="rounded-xl border border-rose-300/25 bg-rose-950/40 p-3 text-sm text-rose-200"
              >
                {formError}
              </p>
            )}
            <button
              type="submit"
              disabled={submitting}
              className="w-full rounded-xl bg-gradient-to-r from-violet-600 to-cyan-500 px-4 py-3 text-sm font-extrabold text-white shadow-lg shadow-violet-950/30 transition hover:brightness-110 disabled:cursor-wait disabled:opacity-60"
            >
              {submitting
                ? "Un momento..."
                : mode === "login"
                  ? "Entrar a mi cuenta"
                  : "Crear cuenta"}
            </button>
          </form>
        </section>
      ) : (
        <div className="mt-6 flex flex-col gap-6 lg:flex-row">
          <aside className="shrink-0 lg:w-60">
            <div className="mb-4 flex flex-col items-center gap-3 rounded-2xl border border-violet-200/15 bg-slate-950/75 p-5">
              <div className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-cyan-300/40 bg-gradient-to-br from-violet-500/30 to-cyan-400/20 text-xl font-black text-white">
                {user.name.trim().slice(0, 2).toUpperCase()}
              </div>
              <div className="text-center">
                <div className="text-sm font-bold text-white">{user.name}</div>
                <div className="mt-1 text-xs text-slate-400">{user.email}</div>
              </div>
            </div>
            <nav className="space-y-1 rounded-2xl border border-violet-200/15 bg-slate-950/75 p-2">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  aria-current={activeTab === tab.id ? "page" : undefined}
                  className={`w-full rounded-xl px-3 py-2.5 text-left text-sm font-semibold transition ${
                    activeTab === tab.id
                      ? "bg-violet-500/20 text-cyan-100"
                      : "text-slate-300 hover:bg-white/5 hover:text-white"
                  }`}
                >
                  {tab.label}
                </button>
              ))}
              <button
                onClick={() => void onLogout()}
                className="w-full rounded-xl px-3 py-2.5 text-left text-sm font-semibold text-rose-300 transition hover:bg-rose-400/10"
              >
                Cerrar sesión
              </button>
            </nav>
          </aside>

          <section className="min-h-64 flex-1 rounded-2xl border border-violet-200/15 bg-slate-950/75 p-6">
            {activeTab === "profile" && (
              <>
                <h1 className="text-xl font-black text-white">Mi perfil</h1>
                <p className="mt-2 text-sm text-slate-300">
                  Esta es la cuenta activa para tus favoritos y tu historial.
                </p>
                <dl className="mt-6 grid gap-4 sm:grid-cols-2">
                  <div className="rounded-xl border border-white/10 bg-slate-900/70 p-4">
                    <dt className="text-xs font-bold uppercase tracking-wide text-slate-400">
                      Nombre
                    </dt>
                    <dd className="mt-1 text-sm font-semibold text-white">{user.name}</dd>
                  </div>
                  <div className="rounded-xl border border-white/10 bg-slate-900/70 p-4">
                    <dt className="text-xs font-bold uppercase tracking-wide text-slate-400">
                      Correo electrónico
                    </dt>
                    <dd className="mt-1 break-all text-sm font-semibold text-white">
                      {user.email}
                    </dd>
                  </div>
                </dl>
              </>
            )}
            {activeTab === "favorites" && (
              <>
                <h2 className="text-xl font-black text-white">Mis favoritos</h2>
                <p className="mt-2 text-sm text-slate-300">
                  {favoritesCount === 0
                    ? "Todavía no has guardado productos o servicios."
                    : `Tienes ${favoritesCount} elementos guardados.`}
                </p>
                <button
                  onClick={() => navigate({ id: "favorites" })}
                  className="mt-5 rounded-xl border border-cyan-200/20 bg-cyan-400/10 px-4 py-2.5 text-sm font-bold text-cyan-100 transition hover:bg-cyan-400/20"
                >
                  Ver todos mis favoritos
                </button>
              </>
            )}
            {activeTab === "history" && (
              <>
                <h2 className="text-xl font-black text-white">Historial de búsquedas</h2>
                {history.length === 0 ? (
                  <p className="mt-3 text-sm text-slate-300">
                    Tus búsquedas aparecerán aquí cuando explores productos o servicios con tu
                    sesión iniciada.
                  </p>
                ) : (
                  <ul className="mt-4 space-y-2">
                    {history.map((entry) => (
                      <li
                        key={entry.id}
                        className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-white/10 bg-slate-900/70 px-4 py-3"
                      >
                        <div>
                          <div className="text-sm font-semibold text-white">{entry.query}</div>
                          <div className="mt-1 text-xs text-slate-400">
                            {entry.type === "service" ? "Servicios" : "Productos"} ·{" "}
                            {new Date(entry.createdAt).toLocaleString()}
                          </div>
                        </div>
                        <button
                          onClick={() =>
                            navigate(
                              entry.type === "service"
                                ? { id: "search-services", query: entry.query }
                                : { id: "search-products", query: entry.query },
                            )
                          }
                          className="text-xs font-bold text-cyan-200 hover:text-white"
                        >
                          Buscar de nuevo
                        </button>
                      </li>
                    ))}
                  </ul>
                )}
              </>
            )}
          </section>
        </div>
      )}
    </div>
  );
}
