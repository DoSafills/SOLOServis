import { useEffect, useRef, useState } from "react";
import type { ChangeEvent, FormEvent } from "react";
import type { Page } from "../../types";
import {
  ApiError,
  changeAccountPassword,
  formatSearchHistoryQuery,
  updateAccountName,
  uploadAccountAvatar,
  type AuthUser,
  type SearchHistoryEntry,
} from "../../services/api/personalization";
import { Breadcrumb, Skeleton } from "../../components/common/ui";
import FavoritesPage from "./FavoritesPage";

interface Props {
  navigate: (page: Page) => void;
  section?: Tab;
  user: AuthUser | null;
  favoritesCount: number;
  favorites: Set<string>;
  authenticated: boolean;
  history: SearchHistoryEntry[];
  accountDataLoading: boolean;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onLogin: (email: string, password: string) => Promise<void>;
  onRegister: (name: string, email: string, password: string) => Promise<void>;
  onLogout: () => Promise<void>;
}

type Tab = "profile" | "favorites" | "history";

export default function UserPage({
  navigate,
  section,
  user,
  favoritesCount,
  favorites,
  authenticated,
  history,
  accountDataLoading,
  onToggleFavorite,
  onLogin,
  onRegister,
  onLogout,
}: Props) {
  const activeTab = section ?? "profile";
  const [mode, setMode] = useState<"login" | "register">("login");
  const [name, setName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [formError, setFormError] = useState<string | null>(null);
  const [submitting, setSubmitting] = useState(false);
  const [logoutError, setLogoutError] = useState<string | null>(null);
  const [loggingOut, setLoggingOut] = useState(false);
  const [profile, setProfile] = useState<AuthUser | null>(user);
  const [profileName, setProfileName] = useState(user?.name ?? "");
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileMessage, setProfileMessage] = useState<string | null>(null);
  const [savingName, setSavingName] = useState(false);
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);
  const avatarPreviewRef = useRef<string | null>(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [changingPassword, setChangingPassword] = useState(false);

  useEffect(() => {
    setProfile(user);
    setProfileName(user?.name ?? "");
  }, [user]);

  useEffect(
    () => () => {
      if (avatarPreviewRef.current) URL.revokeObjectURL(avatarPreviewRef.current);
    },
    [],
  );

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

  const showProfileError = (error: unknown, fallback: string) => {
    setProfileMessage(null);
    setProfileError(error instanceof Error ? error.message : fallback);
  };

  const saveName = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    const trimmed = profileName.trim();
    if (!trimmed || trimmed.length > 150) {
      setProfileMessage(null);
      setProfileError("El nombre debe tener entre 1 y 150 caracteres.");
      return;
    }
    setSavingName(true);
    setProfileError(null);
    setProfileMessage(null);
    try {
      const updated = await updateAccountName(trimmed);
      setProfile((current) => ({ ...(current ?? updated), ...updated }));
      setProfileName(updated.name);
      setProfileMessage("Nombre actualizado.");
    } catch (error) {
      showProfileError(error, "No se pudo guardar el nombre.");
    } finally {
      setSavingName(false);
    }
  };

  const selectAvatar = (event: ChangeEvent<HTMLInputElement>) => {
    setProfileError(null);
    setProfileMessage(null);
    const file = event.target.files?.[0] ?? null;
    if (!file) return;
    if (!["image/jpeg", "image/png", "image/webp"].includes(file.type)) {
      event.target.value = "";
      setProfileError("La foto debe ser JPG, PNG o WebP.");
      return;
    }
    if (file.size > 2 * 1024 * 1024) {
      event.target.value = "";
      setProfileError("La foto no puede superar 2 MB.");
      return;
    }
    if (avatarPreviewRef.current) URL.revokeObjectURL(avatarPreviewRef.current);
    const preview = URL.createObjectURL(file);
    avatarPreviewRef.current = preview;
    setAvatarPreview(preview);
    setAvatarFile(file);
  };

  const saveAvatar = async () => {
    if (!avatarFile) return;
    setUploadingAvatar(true);
    setProfileError(null);
    setProfileMessage(null);
    try {
      const avatarUrl = await uploadAccountAvatar(avatarFile);
      if (avatarPreviewRef.current) URL.revokeObjectURL(avatarPreviewRef.current);
      avatarPreviewRef.current = null;
      setAvatarPreview(`${avatarUrl}?v=${Date.now()}`);
      setAvatarFile(null);
      setProfile((current) => (current ? { ...current, avatarUrl } : current));
      setProfileMessage("Foto actualizada.");
    } catch (error) {
      showProfileError(error, "No se pudo subir la foto.");
    } finally {
      setUploadingAvatar(false);
    }
  };

  const savePassword = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setProfileError(null);
    setProfileMessage(null);
    if (!currentPassword) {
      setProfileError("Ingresa tu contraseña actual.");
      return;
    }
    if (newPassword.length < 10) {
      setProfileError("La contraseña nueva debe tener al menos 10 caracteres.");
      return;
    }
    if (newPassword !== confirmPassword) {
      setProfileError("La confirmación no coincide con la contraseña nueva.");
      return;
    }
    setChangingPassword(true);
    try {
      await changeAccountPassword(currentPassword, newPassword);
      setCurrentPassword("");
      setNewPassword("");
      setConfirmPassword("");
      setProfileMessage("Contraseña actualizada.");
    } catch (error) {
      if (error instanceof ApiError && error.status === 401) {
        setProfileError("La contraseña actual no es correcta.");
      } else {
        showProfileError(error, "No se pudo cambiar la contraseña.");
      }
    } finally {
      setChangingPassword(false);
    }
  };

  const displayName = profile?.name ?? user?.name ?? "";
  const displayEmail = profile?.email ?? user?.email ?? "";
  const avatarSrc = avatarPreview ?? profile?.avatarUrl ?? null;

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
              {avatarSrc ? (
                <img
                  src={avatarSrc}
                  alt="Foto de perfil"
                  className="h-16 w-16 rounded-full border-2 border-prime/50 object-cover"
                />
              ) : (
                <div className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-prime/50 bg-prime-muted text-xl font-black text-prime">
                  {displayName.trim().slice(0, 2).toUpperCase()}
                </div>
              )}
              <div className="text-center">
                <div className="text-sm font-bold text-text">{displayName}</div>
                <div className="mt-1 text-xs text-muted">{displayEmail}</div>
              </div>
            </div>
            <nav className="space-y-1 rounded-2xl border border-bd bg-surface p-2">
              {tabs.map((tab) => (
                <button
                  key={tab.id}
                  onClick={() => navigate({ id: "user", section: tab.id })}
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

          {activeTab === "favorites" ? (
            <FavoritesPage
              navigate={navigate}
              favorites={favorites}
              authenticated={authenticated}
              accountDataLoading={accountDataLoading}
              onToggleFavorite={onToggleFavorite}
              embedded
            />
          ) : (
            <section className="min-h-64 flex-1 rounded-2xl border border-bd bg-surface p-6">
              {activeTab === "profile" && (
                <>
                  <h1 className="text-xl font-black text-text">Mi perfil</h1>
                  <p className="mt-2 text-sm text-muted">
                    Esta es la cuenta activa para tus favoritos y tu historial.
                  </p>
                  <div className="mt-6 space-y-4">
                    <section className="rounded-xl border border-bd bg-surface-2 p-4">
                      <h2 className="text-sm font-bold text-text">Foto de perfil</h2>
                      <div className="mt-3 flex flex-wrap items-center gap-4">
                        {avatarSrc ? (
                          <img
                            src={avatarSrc}
                            alt="Foto de perfil"
                            className="h-16 w-16 rounded-full border-2 border-prime/50 object-cover"
                          />
                        ) : (
                          <div className="flex h-16 w-16 items-center justify-center rounded-full border-2 border-prime/50 bg-prime-muted text-xl font-black text-prime">
                            {displayName.trim().slice(0, 2).toUpperCase()}
                          </div>
                        )}
                        <div className="min-w-48 flex-1">
                          <input
                            type="file"
                            accept="image/jpeg,image/png,image/webp"
                            onChange={selectAvatar}
                            className="block w-full text-sm text-muted file:mr-3 file:rounded-lg file:border-0 file:bg-prime-muted file:px-3 file:py-2 file:text-sm file:font-bold file:text-prime"
                          />
                          <p className="mt-1 text-xs text-muted">JPG, PNG o WebP de hasta 2 MB.</p>
                          <button
                            type="button"
                            onClick={() => void saveAvatar()}
                            disabled={!avatarFile || uploadingAvatar}
                            className="mt-2 rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] transition hover:bg-prime-dark disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {uploadingAvatar ? "Subiendo..." : "Subir foto"}
                          </button>
                        </div>
                      </div>
                    </section>

                    <form
                      onSubmit={saveName}
                      noValidate
                      className="rounded-xl border border-bd bg-surface-2 p-4"
                    >
                      <label className="block text-sm font-semibold text-text">
                        Nombre
                        <input
                          required
                          value={profileName}
                          onChange={(event) => setProfileName(event.target.value)}
                          maxLength={150}
                          className="mt-1.5 w-full rounded-xl border border-bd bg-surface px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                          placeholder="Tu nombre"
                        />
                      </label>
                      <button
                        type="submit"
                        disabled={savingName}
                        className="mt-3 rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] transition hover:bg-prime-dark disabled:cursor-wait disabled:opacity-60"
                      >
                        {savingName ? "Guardando..." : "Guardar nombre"}
                      </button>
                    </form>

                    <div className="rounded-xl border border-bd bg-surface-2 p-4">
                      <div className="text-xs font-bold uppercase tracking-wide text-muted">
                        Correo electrónico
                      </div>
                      <div className="mt-1 break-all text-sm font-semibold text-text">
                        {displayEmail}
                      </div>
                    </div>

                    <form
                      onSubmit={savePassword}
                      noValidate
                      className="rounded-xl border border-bd bg-surface-2 p-4"
                    >
                      <h2 className="text-sm font-bold text-text">Cambiar contraseña</h2>
                      <label className="mt-3 block text-sm font-semibold text-text">
                        Contraseña actual
                        <input
                          required
                          type="password"
                          autoComplete="current-password"
                          value={currentPassword}
                          onChange={(event) => setCurrentPassword(event.target.value)}
                          maxLength={1024}
                          className="mt-1.5 w-full rounded-xl border border-bd bg-surface px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                          placeholder="Tu contraseña actual"
                        />
                      </label>
                      <label className="mt-3 block text-sm font-semibold text-text">
                        Contraseña nueva
                        <input
                          required
                          type="password"
                          autoComplete="new-password"
                          value={newPassword}
                          onChange={(event) => setNewPassword(event.target.value)}
                          minLength={10}
                          maxLength={1024}
                          className="mt-1.5 w-full rounded-xl border border-bd bg-surface px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                          placeholder="Al menos 10 caracteres"
                        />
                      </label>
                      <label className="mt-3 block text-sm font-semibold text-text">
                        Confirmar contraseña nueva
                        <input
                          required
                          type="password"
                          autoComplete="new-password"
                          value={confirmPassword}
                          onChange={(event) => setConfirmPassword(event.target.value)}
                          minLength={10}
                          maxLength={1024}
                          className="mt-1.5 w-full rounded-xl border border-bd bg-surface px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                          placeholder="Repite la contraseña nueva"
                        />
                      </label>
                      <button
                        type="submit"
                        disabled={changingPassword}
                        className="mt-3 rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] transition hover:bg-prime-dark disabled:cursor-wait disabled:opacity-60"
                      >
                        {changingPassword ? "Cambiando..." : "Cambiar contraseña"}
                      </button>
                    </form>

                    {profileError && (
                      <p
                        role="alert"
                        className="rounded-xl border border-rose-300/25 bg-rose-950/40 p-3 text-sm text-rose-200"
                      >
                        {profileError}
                      </p>
                    )}
                    {profileMessage && (
                      <p
                        role="status"
                        className="rounded-xl border border-prime/30 bg-prime-muted p-3 text-sm text-prime"
                      >
                        {profileMessage}
                      </p>
                    )}
                  </div>
                </>
              )}
              {activeTab === "history" && (
                <>
                  <h2 className="text-xl font-black text-text">Historial de búsquedas</h2>
                  {accountDataLoading ? (
                    <div role="status" aria-label="Cargando historial" className="mt-4 space-y-2">
                      {[0, 1, 2].map((item) => (
                        <Skeleton key={item} className="h-16 w-full rounded-xl" />
                      ))}
                    </div>
                  ) : history.length === 0 ? (
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
                            <div className="text-sm font-semibold text-text">
                              {formatSearchHistoryQuery(entry.query)}
                            </div>
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
          )}
        </div>
      )}
    </div>
  );
}
