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
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const [loggingOut, setLoggingOut] = useState(false);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError(null);
    const trimmedName = name.trim();
    const trimmedEmail = email.trim();
    if (mode === "register" && (trimmedName.length < 2 || trimmedName.length > 150)) {
      setFormError("El nombre debe tener entre 2 y 150 caracteres.");
      return;
    }
    if (trimmedEmail.length > 254 || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(trimmedEmail)) {
      setFormError("Ingresa un correo electrónico válido (máximo 254 caracteres).");
      return;
    }
    if (!password || (mode === "register" && password.length < 10)) {
      setFormError(
        mode === "register"
          ? "La contraseña debe tener al menos 10 caracteres."
          : "Ingresa tu contraseña.",
      );
      return;
    }
    setSubmitting(true);
    try {
      if (mode === "register") await onRegister(trimmedName, trimmedEmail, password);
      else await onLogin(trimmedEmail, password);
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
        <section className="mx-auto mt-8 max-w-lg rounded-3xl border border-bd bg-surface p-6 shadow-xl shadow-black/25 sm:p-8">
          <div className="mb-6">
            <p className="text-xs font-bold uppercase tracking-[0.2em] text-prime">SoloService</p>
            <h1 className="mt-2 text-2xl font-black text-text">
              {mode === "login" ? "Inicia sesión" : "Crea tu cuenta"}
            </h1>
            <p className="mt-2 text-sm leading-6 text-muted">
              Puedes explorar productos, servicios y tiendas sin una cuenta. Inicia sesión para
              guardar favoritos y consultar tu historial.
            </p>
          </div>

          <div className="mb-5 grid grid-cols-2 rounded-xl border border-bd bg-surface-2 p-1">
            <button
              type="button"
              onClick={() => {
                setMode("login");
                setFormError(null);
              }}
              className={`rounded-lg px-3 py-2 text-sm font-bold transition ${
                mode === "login" ? "bg-prime-muted text-prime" : "text-muted"
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
                mode === "register" ? "bg-prime-muted text-prime" : "text-muted"
              }`}
            >
              Crear cuenta
            </button>
          </div>

          <form onSubmit={submit} noValidate className="space-y-4">
            {mode === "register" && (
              <label className="block text-sm font-semibold text-text">
                Nombre
                <input
                  required
                  autoComplete="name"
                  value={name}
                  onChange={(event) => setName(event.target.value)}
                  minLength={2}
                  maxLength={150}
                  className="mt-1.5 w-full rounded-xl border border-bd bg-surface-2 px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                  placeholder="Tu nombre"
                />
              </label>
            )}
            <label className="block text-sm font-semibold text-text">
              Correo electrónico
              <input
                required
                type="email"
                autoComplete="email"
                value={email}
                onChange={(event) => setEmail(event.target.value)}
                maxLength={254}
                className="mt-1.5 w-full rounded-xl border border-bd bg-surface-2 px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                placeholder="tu@correo.com"
              />
            </label>
            <label className="block text-sm font-semibold text-text">
              Contraseña
              <input
                required
                type="password"
                autoComplete={mode === "register" ? "new-password" : "current-password"}
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                minLength={mode === "register" ? 10 : undefined}
                maxLength={1024}
                className="mt-1.5 w-full rounded-xl border border-bd bg-surface-2 px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
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
              className="w-full rounded-xl bg-prime px-4 py-3 text-sm font-extrabold text-[#181714] shadow-lg shadow-black/20 transition hover:bg-prime-dark disabled:cursor-wait disabled:opacity-60"
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
            <div className="mb-4 flex flex-col items-center gap-3 rounded-2xl border border-bd bg-surface p-5">
              <div className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-prime/50 bg-prime-muted text-xl font-black text-prime">
                {user.name.trim().slice(0, 2).toUpperCase()}
              </div>
              <div className="text-center">
                <div className="text-sm font-bold text-text">{user.name}</div>
                <div className="mt-1 text-xs text-muted">{user.email}</div>
              </div>
            </div>
            <nav className="space-y-1 rounded-2xl border border-bd bg-surface p-2">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => setActiveTab(tab.id)}
                  aria-current={activeTab === tab.id ? "page" : undefined}
                  className={`w-full rounded-xl px-3 py-2.5 text-left text-sm font-semibold transition ${
                    activeTab === tab.id
                      ? "bg-prime-muted text-prime"
                      : "text-muted hover:bg-surface-2 hover:text-text"
                  }`}
                >
                  {tab.label}
                </button>
              ))}
              <button
                disabled={loggingOut}
                onClick={() => {
                  setLogoutError(null);
                  setLoggingOut(true);
                  void onLogout()
                    .catch((error: unknown) =>
                      setLogoutError(
                        error instanceof Error ? error.message : "No se pudo cerrar la sesión.",
                      ),
                    )
                    .finally(() => setLoggingOut(false));
                }}
                className="w-full rounded-xl px-3 py-2.5 text-left text-sm font-semibold text-rose-300 transition hover:bg-rose-400/10"
              >
                {loggingOut ? "Cerrando sesión..." : "Cerrar sesión"}
              </button>
            </nav>
            {logoutError && (
              <p role="alert" className="mt-3 text-sm text-rose-300">
                {logoutError}
              </p>
            )}
          </aside>

          <section className="min-h-64 flex-1 rounded-2xl border border-bd bg-surface p-6">
            {activeTab === "profile" && (
              <>
                <h1 className="text-xl font-black text-text">Mi perfil</h1>
                <p className="mt-2 text-sm text-muted">
                  Esta es la cuenta activa para tus favoritos y tu historial.
                </p>
                <dl className="mt-6 grid gap-4 sm:grid-cols-2">
                  <div className="rounded-xl border border-bd bg-surface-2 p-4">
                    <dt className="text-xs font-bold uppercase tracking-wide text-muted">Nombre</dt>
                    <dd className="mt-1 text-sm font-semibold text-text">{user.name}</dd>
                  </div>
                  <div className="rounded-xl border border-bd bg-surface-2 p-4">
                    <dt className="text-xs font-bold uppercase tracking-wide text-muted">
                      Correo electrónico
                    </dt>
                    <dd className="mt-1 break-all text-sm font-semibold text-text">{user.email}</dd>
                  </div>
                </dl>
              </>
            )}
            {activeTab === "favorites" && (
              <>
                <h2 className="text-xl font-black text-text">Mis favoritos</h2>
                <p className="mt-2 text-sm text-muted">
                  {favoritesCount === 0
                    ? "Todavía no has guardado productos o servicios."
                    : `Tienes ${favoritesCount} elementos guardados.`}
                </p>
                <button
                  onClick={() => navigate({ id: "favorites" })}
                  className="mt-5 rounded-xl border border-prime/40 bg-prime-muted px-4 py-2.5 text-sm font-bold text-prime transition hover:bg-prime/20"
                >
                  Ver todos mis favoritos
                </button>
              </>
            )}
            {activeTab === "history" && (
              <>
                <h2 className="text-xl font-black text-text">Historial de búsquedas</h2>
                {history.length === 0 ? (
                  <p className="mt-3 text-sm text-muted">
                    Tus búsquedas aparecerán aquí cuando explores productos o servicios con tu
                    sesión iniciada.
                  </p>
                ) : (
                  <ul className="mt-4 space-y-2">
                    {history.map((entry) => (
                      <li
                        key={entry.id}
                        className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-bd bg-surface-2 px-4 py-3"
                      >
                        <div>
                          <div className="text-sm font-semibold text-text">{entry.query}</div>
                          <div className="mt-1 text-xs text-muted">
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
                          className="text-xs font-bold text-prime hover:text-text"
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
