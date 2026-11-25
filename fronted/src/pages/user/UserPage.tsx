import { useEffect, useRef, useState } from "react";
import type { ChangeEvent, FormEvent } from "react";
import type { Page } from "../../types";
import {
  ApiError,
  changeAccountPassword,
  formatSearchHistoryQuery,
  logoutAllAccounts,
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
type PasswordVisibilityField = "current" | "new" | "confirm";

interface PasswordFieldProps {
  id: string;
  label: string;
  value: string;
  autoComplete: "current-password" | "new-password";
  placeholder: string;
  visible: boolean;
  minLength?: number;
  maxLength?: number;
  onChange: (value: string) => void;
  onToggleVisibility: () => void;
}

/**
 * Dibuja un campo de contraseña con su etiqueta y el control para mostrar u ocultar el valor.
 * Recibe el identificador, los textos, el estado y los callbacks del formulario; devuelve
 * el JSX accesible del campo para reutilizarlo sin duplicar la lógica de visibilidad.
 */
function PasswordField({
  id,
  label,
  value,
  autoComplete,
  placeholder,
  visible,
  minLength,
  maxLength,
  onChange,
  onToggleVisibility,
}: PasswordFieldProps) {
  return (
    <div>
      <label htmlFor={id} className="block text-sm font-semibold text-text">
        {label}
      </label>
      <div className="relative mt-1.5">
        <input
          id={id}
          required
          type={visible ? "text" : "password"}
          autoComplete={autoComplete}
          value={value}
          onChange={(event) => onChange(event.target.value)}
          minLength={minLength}
          maxLength={maxLength}
          className="w-full rounded-xl border border-bd bg-surface px-4 py-3 pr-20 text-text outline-none transition placeholder:text-muted focus:border-prime"
          placeholder={placeholder}
        />
        <button
          type="button"
          onClick={onToggleVisibility}
          aria-label={`${visible ? "Ocultar" : "Mostrar"} ${label.toLowerCase()}`}
          aria-pressed={visible}
          className="absolute right-2 top-1/2 -translate-y-1/2 rounded-lg px-2 py-1 text-xs font-bold text-muted transition hover:bg-surface-2 hover:text-text"
        >
          {visible ? "Ocultar" : "Mostrar"}
        </button>
      </div>
    </div>
  );
}

/**
 * Muestra la cuenta activa, sus pestañas y los formularios para iniciar sesión o editar
 * el perfil. Recibe los datos y callbacks de la cuenta desde App y devuelve la pantalla
 * completa de Mi cuenta, manteniendo la navegación y los estados de carga existentes.
 */
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
  const [loggingOutAll, setLoggingOutAll] = useState(false);
  const [profile, setProfile] = useState<AuthUser | null>(user);
  const [profileName, setProfileName] = useState(user?.name ?? "");
  const [syncedUser, setSyncedUser] = useState(user);
  if (user !== syncedUser) {
    setSyncedUser(user);
    setProfile(user);
    setProfileName(user?.name ?? "");
  }
  const [profileError, setProfileError] = useState<string | null>(null);
  const [profileMessage, setProfileMessage] = useState<string | null>(null);
  const [savingName, setSavingName] = useState(false);
  const [avatarFile, setAvatarFile] = useState<File | null>(null);
  const [avatarPreview, setAvatarPreview] = useState<string | null>(null);
  const avatarPreviewRef = useRef<string | null>(null);
  const avatarInputRef = useRef<HTMLInputElement>(null);
  const [uploadingAvatar, setUploadingAvatar] = useState(false);
  const [currentPassword, setCurrentPassword] = useState("");
  const [newPassword, setNewPassword] = useState("");
  const [confirmPassword, setConfirmPassword] = useState("");
  const [passwordVisibility, setPasswordVisibility] = useState<
    Record<PasswordVisibilityField, boolean>
  >({ current: false, new: false, confirm: false });
  const [changingPassword, setChangingPassword] = useState(false);

  useEffect(
    () => () => {
      if (avatarPreviewRef.current) URL.revokeObjectURL(avatarPreviewRef.current);
    },
    [],
  );

  /**
   * Valida y envía el formulario de acceso o registro. Recibe el evento del formulario,
   * ejecuta el callback correspondiente y no devuelve un valor; centraliza los errores
   * para mostrarlos sin cambiar el flujo de autenticación existente.
   */
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

  /**
   * Muestra un error de edición del perfil y limpia el mensaje de éxito anterior.
   * Recibe el error desconocido y un texto de respaldo; no devuelve un valor.
   */
  const showProfileError = (error: unknown, fallback: string) => {
    setProfileMessage(null);
    setProfileError(error instanceof Error ? error.message : fallback);
  };

  /**
   * Valida y guarda el nombre editado del perfil. Recibe el evento del formulario,
   * actualiza el estado local con la respuesta de la API y no devuelve un valor.
   */
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

  /**
   * Valida la imagen elegida y crea su vista previa local. Recibe el evento del input
   * de archivo, actualiza la selección sin subirla todavía y no devuelve un valor.
   */
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

  /**
   * Sube la foto seleccionada y reemplaza el preview por la URL devuelta por la API.
   * No recibe parámetros ni devuelve un valor; mantiene el archivo disponible si la
   * subida falla para que la persona pueda reintentarla.
   */
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
      if (avatarInputRef.current) avatarInputRef.current.value = "";
      setProfile((current) => (current ? { ...current, avatarUrl } : current));
      setProfileMessage("Foto actualizada.");
    } catch (error) {
      showProfileError(error, "No se pudo subir la foto.");
    } finally {
      setUploadingAvatar(false);
    }
  };

  /**
   * Cancela la foto pendiente y libera su vista previa. No recibe parámetros ni devuelve
   * un valor; deja el input limpio para permitir elegir el mismo archivo otra vez.
   */
  const cancelAvatar = () => {
    if (avatarPreviewRef.current) URL.revokeObjectURL(avatarPreviewRef.current);
    avatarPreviewRef.current = null;
    setAvatarPreview(null);
    setAvatarFile(null);
    if (avatarInputRef.current) avatarInputRef.current.value = "";
    setProfileError(null);
    setProfileMessage(null);
  };

  /**
   * Alterna la visibilidad de uno de los campos de contraseña. Recibe la clave del campo
   * y actualiza su estado sin modificar el valor escrito; no devuelve un valor.
   */
  const togglePasswordVisibility = (field: PasswordVisibilityField) => {
    setPasswordVisibility((current) => ({ ...current, [field]: !current[field] }));
  };

  /**
   * Valida y envía el cambio de contraseña de la sesión activa. Recibe el evento del
   * formulario, limpia los campos tras guardar y no devuelve un valor.
   */
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
  const hasNameChanges = profileName !== displayName;
  const hasPasswordChanges = Boolean(currentPassword || newPassword || confirmPassword);

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
              <button
                disabled={loggingOutAll}
                onClick={() => {
                  setLogoutError(null);
                  setLoggingOutAll(true);
                  void logoutAllAccounts()
                    .then(() => onLogout())
                    .catch((error: unknown) =>
                      setLogoutError(
                        error instanceof Error ? error.message : "No se pudo cerrar la sesión.",
                      ),
                    )
                    .finally(() => setLoggingOutAll(false));
                }}
                className="w-full rounded-xl px-3 py-2.5 text-left text-sm font-semibold text-rose-300 transition hover:bg-rose-400/10"
              >
                {loggingOutAll ? "Cerrando todas las sesiones..." : "Cerrar todas mis sesiones"}
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
                  <div className="border-b border-bd pb-6">
                    <h1 className="text-xl font-black text-text">Mi perfil</h1>
                    <p className="mt-2 text-sm text-muted">
                      Esta es la cuenta activa para tus favoritos y tu historial.
                    </p>
                  </div>

                  {(profileError || profileMessage) && (
                    <div className="mt-5 space-y-2" aria-live="polite">
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
                  )}

                  <div className="mt-6 space-y-5">
                    <section className="rounded-2xl border border-bd bg-surface-2 p-5 sm:p-6">
                      <h2 className="text-sm font-bold text-text">Foto de perfil</h2>
                      <div className="mt-5 flex flex-col items-center gap-5 rounded-2xl border border-bd bg-surface p-5 sm:flex-row sm:items-center">
                        {avatarSrc ? (
                          <img
                            src={avatarSrc}
                            alt="Vista previa de la foto de perfil"
                            className="h-28 w-28 shrink-0 rounded-full border-2 border-prime/50 object-cover sm:h-32 sm:w-32"
                          />
                        ) : (
                          <div className="flex h-28 w-28 shrink-0 items-center justify-center rounded-full border-2 border-prime/50 bg-prime-muted text-3xl font-black text-prime sm:h-32 sm:w-32">
                            {displayName.trim().slice(0, 2).toUpperCase()}
                          </div>
                        )}
                        <div className="w-full min-w-0 flex-1 text-center sm:text-left">
                          <input
                            ref={avatarInputRef}
                            id="profile-avatar"
                            type="file"
                            accept="image/jpeg,image/png,image/webp"
                            onChange={selectAvatar}
                            aria-describedby="avatar-help"
                            className="sr-only"
                          />
                          <div className="flex flex-wrap items-center justify-center gap-2 sm:justify-start">
                            <button
                              type="button"
                              onClick={() => avatarInputRef.current?.click()}
                              disabled={uploadingAvatar}
                              className="rounded-xl border border-bd bg-surface-2 px-4 py-2 text-sm font-bold text-text transition hover:border-prime/50 hover:text-prime disabled:cursor-wait disabled:opacity-60"
                            >
                              Cambiar imagen
                            </button>
                            {avatarFile && (
                              <button
                                type="button"
                                onClick={() => void saveAvatar()}
                                disabled={uploadingAvatar}
                                className="rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] transition hover:bg-prime-dark disabled:cursor-wait disabled:opacity-60"
                              >
                                {uploadingAvatar ? "Subiendo..." : "Subir foto"}
                              </button>
                            )}
                          </div>
                          {avatarFile && (
                            <div className="mt-2 flex flex-wrap items-center justify-center gap-2 sm:justify-start">
                              <p className="min-w-0 flex-1 truncate text-xs text-muted">
                                {avatarFile.name}
                              </p>
                              <button
                                type="button"
                                onClick={cancelAvatar}
                                disabled={uploadingAvatar}
                                className="text-xs font-bold text-muted transition hover:text-text disabled:cursor-wait disabled:opacity-60"
                              >
                                Cancelar
                              </button>
                            </div>
                          )}
                          <p id="avatar-help" className="mt-2 text-xs text-muted">
                            JPG, PNG o WebP de hasta 2 MB.
                          </p>
                        </div>
                      </div>
                    </section>

                    <details className="group overflow-hidden rounded-2xl border border-bd bg-surface-2">
                      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 p-5 sm:p-6 [&::-webkit-details-marker]:hidden">
                        <span className="text-sm font-bold text-text">Nombre</span>
                        <svg
                          aria-hidden="true"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          className="h-5 w-5 shrink-0 text-muted transition-transform group-open:rotate-180"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" d="m6 9 6 6 6-6" />
                        </svg>
                      </summary>
                      <form
                        onSubmit={saveName}
                        noValidate
                        className="border-t border-bd p-5 sm:p-6"
                      >
                        <label htmlFor="profile-name" className="sr-only">
                          Nombre
                        </label>
                        <input
                          id="profile-name"
                          required
                          value={profileName}
                          onChange={(event) => setProfileName(event.target.value)}
                          maxLength={150}
                          className="w-full rounded-xl border border-bd bg-surface px-4 py-3 text-text outline-none transition placeholder:text-muted focus:border-prime"
                          placeholder="Tu nombre"
                        />
                        <div className="mt-5 flex justify-end">
                          <button
                            type="submit"
                            disabled={savingName || !hasNameChanges}
                            className="rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] transition hover:bg-prime-dark disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {savingName ? "Guardando..." : "Guardar nombre"}
                          </button>
                        </div>
                      </form>
                    </details>

                    <details className="group overflow-hidden rounded-2xl border border-bd bg-surface-2">
                      <summary className="flex cursor-pointer list-none items-center justify-between gap-4 p-5 sm:p-6 [&::-webkit-details-marker]:hidden">
                        <span className="text-sm font-bold text-text">Cambiar contraseña</span>
                        <svg
                          aria-hidden="true"
                          viewBox="0 0 24 24"
                          fill="none"
                          stroke="currentColor"
                          strokeWidth="2"
                          className="h-5 w-5 shrink-0 text-muted transition-transform group-open:rotate-180"
                        >
                          <path strokeLinecap="round" strokeLinejoin="round" d="m6 9 6 6 6-6" />
                        </svg>
                      </summary>
                      <form
                        onSubmit={savePassword}
                        noValidate
                        className="border-t border-bd p-5 sm:p-6"
                      >
                        <div className="space-y-4">
                          <PasswordField
                            id="current-password"
                            label="Contraseña actual"
                            value={currentPassword}
                            autoComplete="current-password"
                            placeholder="Tu contraseña actual"
                            visible={passwordVisibility.current}
                            maxLength={1024}
                            onChange={setCurrentPassword}
                            onToggleVisibility={() => togglePasswordVisibility("current")}
                          />
                          <PasswordField
                            id="new-password"
                            label="Contraseña nueva"
                            value={newPassword}
                            autoComplete="new-password"
                            placeholder="Al menos 10 caracteres"
                            visible={passwordVisibility.new}
                            minLength={10}
                            maxLength={1024}
                            onChange={setNewPassword}
                            onToggleVisibility={() => togglePasswordVisibility("new")}
                          />
                          <PasswordField
                            id="confirm-password"
                            label="Confirmar contraseña nueva"
                            value={confirmPassword}
                            autoComplete="new-password"
                            placeholder="Repite la contraseña nueva"
                            visible={passwordVisibility.confirm}
                            minLength={10}
                            maxLength={1024}
                            onChange={setConfirmPassword}
                            onToggleVisibility={() => togglePasswordVisibility("confirm")}
                          />
                        </div>
                        <div className="mt-6 flex justify-end">
                          <button
                            type="submit"
                            disabled={changingPassword || !hasPasswordChanges}
                            className="rounded-xl bg-prime px-4 py-2 text-sm font-extrabold text-[#181714] transition hover:bg-prime-dark disabled:cursor-not-allowed disabled:opacity-60"
                          >
                            {changingPassword ? "Cambiando..." : "Cambiar contraseña"}
                          </button>
                        </div>
                      </form>
                    </details>

                    <section className="rounded-2xl border border-bd bg-surface-2 p-5 sm:p-6">
                      <h2 className="text-sm font-bold text-text">Correo electrónico</h2>
                      <p className="mt-3 break-all text-sm font-semibold text-text">
                        {displayEmail}
                      </p>
                    </section>
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
