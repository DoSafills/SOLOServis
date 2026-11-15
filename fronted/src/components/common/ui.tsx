export function Badge({
  variant = "default",
  children,
}: {
  variant?: "available" | "unavailable" | "offer" | "best" | "default";
  children: React.ReactNode;
}) {
  const styles = {
    available: {
      background: "rgba(3,105,161,0.12)",
      color: "#0369A1",
      border: "1px solid rgba(3,105,161,0.3)",
    },
    unavailable: {
      background: "rgba(248,113,113,0.12)",
      color: "#F87171",
      border: "1px solid rgba(248,113,113,0.25)",
    },
    offer: {
      background: "rgba(251,191,36,0.12)",
      color: "#FBBF24",
      border: "1px solid rgba(251,191,36,0.25)",
    },
    best: { background: "#0369A1", color: "#FFFFFF", border: "none" },
    default: { background: "#E2E8F0", color: "#64748B", border: "1px solid #CBD5E1" },
  };
  return (
    <span
      style={styles[variant]}
      className="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-semibold"
    >
      {children}
    </span>
  );
}

export function Rating({ value, count }: { value: number; count?: number }) {
  // La API puede devolver rating/count como string (p. ej. campos DECIMAL
  // serializados desde el backend), así que forzamos number antes de operar.
  const safeValue = Number(value) || 0;
  const safeCount = count !== undefined ? Number(count) || 0 : undefined;
  const scoreColor =
    safeValue >= 4
      ? "text-emerald-400"
      : safeValue >= 3
        ? "text-amber-400"
        : safeValue > 0
          ? "text-rose-400"
          : "text-muted";

  return (
    <div className="flex items-center gap-1.5">
      <div className="flex">
        {[1, 2, 3, 4, 5].map((i) => (
          <svg
            key={i}
            width="12"
            height="12"
            viewBox="0 0 24 24"
            fill={i <= Math.round(safeValue) ? "#FBBF24" : "#2A2A2A"}
          >
            <path d="M12 2l3.09 6.26L22 9.27l-5 4.87 1.18 6.88L12 17.77l-6.18 3.25L7 14.14 2 9.27l6.91-1.01L12 2z" />
          </svg>
        ))}
      </div>
      <span className={`text-xs font-semibold ${scoreColor}`}>{safeValue.toFixed(1)}</span>
      {safeCount !== undefined && (
        <span className="text-xs text-muted">({safeCount.toLocaleString("es-CL")})</span>
      )}
    </div>
  );
}

export function FavoriteButton({
  active,
  onClick,
}: {
  active: boolean;
  onClick: (e: React.MouseEvent) => void;
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      aria-label={active ? "Quitar de favoritos" : "Agregar a favoritos"}
      aria-pressed={active}
      className="rounded-lg p-2 transition-all duration-200 hover:bg-red-500/10"
      title={active ? "Quitar de favoritos" : "Agregar a favoritos"}
    >
      <svg
        width="18"
        height="18"
        viewBox="0 0 24 24"
        fill={active ? "#EF4444" : "none"}
        stroke={active ? "#EF4444" : "#64748B"}
        strokeWidth="2"
      >
        <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
      </svg>
    </button>
  );
}

export function Breadcrumb({ items }: { items: { label: string; onClick?: () => void }[] }) {
  return (
    <nav className="flex items-center gap-1.5 text-xs text-muted mb-6 flex-wrap">
      {items.map((item, i) => (
        <span key={i} className="flex items-center gap-1.5">
          {i > 0 && <span className="text-surface-3">/</span>}
          {item.onClick ? (
            <button onClick={item.onClick} className="hover:text-prime transition-colors">
              {item.label}
            </button>
          ) : (
            <span className="text-muted-2">{item.label}</span>
          )}
        </span>
      ))}
    </nav>
  );
}

export function LoadingState() {
  return (
    <div className="flex flex-col items-center justify-center py-24 gap-4">
      <div className="w-10 h-10 border-2 border-prime border-t-transparent rounded-full animate-spin" />
      <p className="text-sm text-muted">Buscando ofertas...</p>
    </div>
  );
}

export function Skeleton({ className = "" }: { className?: string }) {
  return (
    <div aria-hidden="true" className={`animate-pulse rounded-lg bg-surface-2 ${className}`} />
  );
}

export function SkeletonCard() {
  return (
    <article
      aria-hidden="true"
      className="overflow-hidden rounded-xl border border-bd bg-surface p-4"
    >
      <Skeleton className="mb-4 h-40 w-full" />
      <Skeleton className="mb-2 h-5 w-3/4" />
      <Skeleton className="mb-2 h-3 w-1/2" />
      <div className="mt-5 flex items-center justify-between gap-3">
        <Skeleton className="h-6 w-1/3" />
        <Skeleton className="h-9 w-24 rounded-lg" />
      </div>
    </article>
  );
}

export function SkeletonGrid({
  count = 4,
  className = "grid grid-cols-1 gap-4 sm:grid-cols-2",
}: {
  count?: number;
  className?: string;
}) {
  return (
    <div role="status" aria-label="Cargando contenido" className={className}>
      {Array.from({ length: count }, (_, index) => (
        <SkeletonCard key={index} />
      ))}
    </div>
  );
}

export function ProductSkeletonGrid({
  count = 4,
  compact = false,
  className = "grid grid-cols-1 gap-4 sm:grid-cols-2",
}: {
  count?: number;
  compact?: boolean;
  className?: string;
}) {
  return (
    <div role="status" aria-label="Cargando productos" className={className}>
      {Array.from({ length: count }, (_, index) => (
        <article
          key={index}
          aria-hidden="true"
          className={`flex ${
            compact ? "min-h-[11rem] sm:min-h-[10.5rem]" : "min-h-[14.4rem] sm:min-h-[13.2rem]"
          } overflow-hidden rounded-[1.75rem] border border-bd bg-surface`}
        >
          <div
            className="h-full w-[58%] shrink-0"
            style={{ clipPath: "polygon(0 0, 100% 0, 88% 100%, 0 100%)" }}
          >
            <Skeleton className="h-full w-full rounded-none" />
          </div>
          <div className="flex h-full min-w-0 flex-1 flex-col justify-center gap-1 bg-surface px-2 py-1.5">
            <Skeleton className="h-2 w-12" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-4 w-20" />
            <div className="flex flex-col gap-1.5">
              <div className="grid grid-cols-2 gap-1.5">
                <Skeleton className="h-8 rounded-lg" />
                <Skeleton className="h-8 rounded-lg" />
              </div>
              <Skeleton className="h-8 rounded-lg" />
            </div>
          </div>
        </article>
      ))}
    </div>
  );
}

export function ServiceSkeletonGrid({
  count = 4,
  className = "grid grid-cols-1 gap-4 sm:grid-cols-2",
}: {
  count?: number;
  className?: string;
}) {
  return (
    <div role="status" aria-label="Cargando servicios" className={className}>
      {Array.from({ length: count }, (_, index) => (
        <article
          key={index}
          aria-hidden="true"
          className="flex min-h-[14.4rem] overflow-hidden rounded-[1.75rem] border border-bd bg-surface sm:min-h-[13.2rem]"
        >
          <div
            className="h-full w-[58%] shrink-0"
            style={{ clipPath: "polygon(0 0, 100% 0, 88% 100%, 0 100%)" }}
          >
            <Skeleton className="h-full w-full rounded-none" />
          </div>
          <div className="flex h-full min-w-0 flex-1 flex-col justify-center gap-1 bg-surface px-2 py-1.5">
            <Skeleton className="h-2 w-14" />
            <Skeleton className="h-4 w-full" />
            <Skeleton className="h-3 w-16" />
            <Skeleton className="h-4 w-20" />
            <div className="flex flex-col gap-1.5">
              <div className="grid grid-cols-2 gap-1.5">
                <Skeleton className="h-8 rounded-lg" />
                <Skeleton className="h-8 rounded-lg" />
              </div>
              <Skeleton className="h-8 rounded-lg" />
            </div>
          </div>
        </article>
      ))}
    </div>
  );
}

export function EmptyState({
  title,
  description,
  action,
}: {
  title: string;
  description: string;
  action?: { label: string; onClick: () => void };
}) {
  return (
    <div className="flex flex-col items-center justify-center py-24 gap-4 text-center">
      <div className="w-16 h-16 rounded-2xl bg-surface-2 flex items-center justify-center mb-2">
        <svg
          width="28"
          height="28"
          viewBox="0 0 24 24"
          fill="none"
          stroke="#64748B"
          strokeWidth="1.5"
        >
          <circle cx="11" cy="11" r="8" />
          <path d="m21 21-4.35-4.35" />
          <path d="M11 8v3M11 14h.01" />
        </svg>
      </div>
      <h3 className="text-lg font-semibold text-text">{title}</h3>
      <p className="text-sm text-muted max-w-xs">{description}</p>
      {action && (
        <button
          onClick={action.onClick}
          style={{ background: "#0369A1", color: "#FFFFFF" }}
          className="mt-2 px-5 py-2 rounded-xl text-sm font-semibold transition-opacity hover:opacity-90"
        >
          {action.label}
        </button>
      )}
    </div>
  );
}

export function Pagination({
  page,
  total,
  perPage,
  onChange,
}: {
  page: number;
  total: number;
  perPage: number;
  onChange: (p: number) => void;
}) {
  const pages = Math.ceil(total / perPage);
  if (pages <= 1) return null;
  return (
    <div className="flex items-center justify-center gap-1 mt-8">
      {Array.from({ length: Math.min(pages, 7) }, (_, i) => i + 1).map((p) => (
        <button
          key={p}
          onClick={() => onChange(p)}
          aria-current={p === page ? "page" : undefined}
          className={`flex h-9 w-9 items-center justify-center rounded-lg border text-sm font-medium transition-colors focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime ${
            p === page
              ? "border-prime bg-prime text-[#181714]"
              : "border-bd bg-surface-2 text-muted hover:border-prime/60 hover:bg-prime-muted hover:text-prime"
          }`}
        >
          {p}
        </button>
      ))}
    </div>
  );
}
