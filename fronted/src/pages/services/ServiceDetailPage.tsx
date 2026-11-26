import { useEffect, useRef, useState } from "react";
import type { Page, Service } from "../../types";
import { getServiceById } from "../../services/api/api";
import { formatPrice, getBillingPeriodText } from "../../services/utils/productUtils";
import { Breadcrumb, FavoriteButton, Rating, Skeleton } from "../../components/common/ui";
import PriceHistory from "../../components/products/PriceHistory";

interface Props {
  serviceId: string;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  cartServiceIds: ReadonlySet<string>;
  onToggleFavorite: (id: string, kind?: "product" | "service") => void;
  onToggleCompare: (id: string) => void;
  onAddToCart: (service: Service) => Promise<boolean>;
}

type ServiceSpecification = [name: string, value: string];

/**
 * Selecciona las primeras cuatro características para el resumen del servicio.
 * Recibe el mapa de especificaciones y devuelve pares nombre-valor ordenados.
 */
const getPrimarySpecifications = (specs: Record<string, string>): ServiceSpecification[] =>
  Object.entries(specs).slice(0, 4);

/**
 * Valida el enlace externo del servicio antes de crear un anchor.
 * Recibe la URL opcional entregada por la API y devuelve `null` si está vacía,
 * está mal formada o no usa HTTP o HTTPS.
 */
const getSafeServiceUrl = (url: string | undefined): string | null => {
  if (!url) return null;

  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
};

/**
 * Agrupa las especificaciones por tema para la sección inferior del detalle.
 * Recibe el mapa de especificaciones y devuelve solo los grupos con elementos,
 * asignando cada característica a la primera categoría que coincide.
 */
const groupServiceSpecifications = (specs: Record<string, string>) => {
  const groups = [
    { title: "Conectividad y velocidad", test: /(velocidad|internet|conexi[oó]n|red|banda)/i },
    {
      title: "Seguridad y dispositivos",
      test: /(seguridad|dispositivo|vpn|servidor|protecci[oó]n)/i,
    },
    {
      title: "Educación y modalidad",
      test: /(clase|curso|educaci[oó]n|certificado|modalidad|tipo)/i,
    },
    { title: "Otros", test: /.*/ },
  ];
  const entries = Object.entries(specs) as ServiceSpecification[];
  const assigned = new Set<string>();

  return groups
    .map(({ title, test }) => {
      const items = entries.filter(([name]) => {
        if (assigned.has(name) || !test.test(name)) return false;
        assigned.add(name);
        return true;
      });
      return { title, items };
    })
    .filter((group) => group.items.length > 0);
};

/**
 * Muestra la ficha completa de un servicio y sus acciones de compra.
 * Recibe el identificador, la navegación, los estados de favorito, comparación
 * y cesta, y los callbacks para modificarlos; devuelve la interfaz del detalle.
 */
export default function ServiceDetailPage({
  serviceId,
  navigate,
  isFavorite,
  isComparing,
  cartServiceIds,
  onToggleFavorite,
  onToggleCompare,
  onAddToCart,
}: Props) {
  const [result, setResult] = useState<{ serviceId: string; service: Service | null } | null>(null);
  const [adding, setAdding] = useState(false);
  const addingRef = useRef(false);

  useEffect(() => {
    let cancelled = false;

    getServiceById(serviceId)
      .then((service) => {
        if (!cancelled) {
          setResult({ serviceId, service });
        }
      })
      .catch((error) => {
        console.error(error);
        if (!cancelled) {
          setResult({ serviceId, service: null });
        }
      });

    return () => {
      cancelled = true;
    };
  }, [serviceId]);

  const loading = result?.serviceId !== serviceId;
  const service = result?.serviceId === serviceId ? result.service : null;
  const isInCart = Boolean(service && cartServiceIds.has(service.id));

  /**
   * Agrega el servicio actual a la cesta e impide clics duplicados.
   * No recibe argumentos y devuelve una promesa que termina con el callback.
   */
  const addServiceToCart = async () => {
    if (!service || addingRef.current || isInCart) return;
    addingRef.current = true;
    setAdding(true);
    try {
      await onAddToCart(service);
    } finally {
      addingRef.current = false;
      setAdding(false);
    }
  };

  if (loading) {
    return (
      <div role="status" aria-label="Cargando servicio" className="mx-auto max-w-7xl px-4 py-8">
        <Skeleton className="mb-6 h-4 w-44" />
        <div className="grid gap-8 lg:grid-cols-2">
          <Skeleton className="min-h-80 rounded-2xl sm:min-h-[28rem]" />
          <div className="space-y-4">
            <Skeleton className="h-8 w-4/5" />
            <Skeleton className="h-5 w-2/5" />
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-10 w-1/2" />
            <Skeleton className="h-14 w-full rounded-xl" />
          </div>
        </div>
        <Skeleton className="mt-10 h-48 w-full rounded-2xl" />
      </div>
    );
  }

  if (!service) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Servicio no encontrado.</p>
      </div>
    );
  }

  const primarySpecifications = getPrimarySpecifications(service.specs);
  const specificationGroups = groupServiceSpecifications(service.specs);
  const serviceUrl = getSafeServiceUrl(service.url);

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Servicios",
            onClick: () => navigate({ id: "search-services", query: "" }),
          },
          {
            label: service.category,
            onClick: () =>
              navigate({
                id: "search-services",
                query: "",
                category: service.category,
              }),
          },
          { label: service.name },
        ]}
      />

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        <div
          style={{
            background: "linear-gradient(135deg, var(--color-surface-3), var(--color-surface))",
            border: "1px solid var(--color-bd)",
            boxShadow: "0 18px 40px rgba(0, 0, 0, 0.25)",
          }}
          className="rounded-2xl overflow-hidden h-[420px] md:h-[520px]"
        >
          {service.image ? (
            <img
              src={service.image}
              alt={service.name}
              className="w-full h-full object-contain p-4 transition-all duration-200"
              style={{
                background:
                  "radial-gradient(circle at top, rgba(209,168,90,0.12), transparent 40%)",
              }}
            />
          ) : (
            <div className="flex h-full items-center justify-center text-sm text-muted">
              Sin imagen disponible
            </div>
          )}
        </div>

        <div className="flex flex-col gap-4">
          <div>
            <div className="flex items-center justify-between mb-2">
              <span className="inline-flex items-center gap-2 rounded-full border border-prime/40 bg-prime-muted px-3 py-1 text-[10px] font-bold uppercase tracking-[0.18em] text-prime">
                {service.provider} · {service.category}
              </span>
              <FavoriteButton
                active={isFavorite}
                onClick={() => onToggleFavorite(service.id, "service")}
              />
            </div>

            <h1 className="text-2xl md:text-3xl font-black text-text leading-tight tracking-[-0.04em]">
              {service.name}
            </h1>

            <p className="mt-2 text-xs font-semibold uppercase tracking-[0.14em] text-prime">
              {service.subcategory}
            </p>

            <div className="mt-3">
              <Rating value={service.rating} count={service.reviewCount} />
            </div>
          </div>

          <p className="text-sm text-muted leading-relaxed">{service.description}</p>

          <div
            style={{
              background: "linear-gradient(180deg, var(--color-surface-2), var(--color-surface))",
              border: "1px solid var(--color-bd)",
              boxShadow: "0 14px 30px rgba(0, 0, 0, 0.2)",
            }}
            className="rounded-xl p-4"
          >
            <h2 className="text-sm font-semibold text-text mb-3">Características principales</h2>

            {primarySpecifications.length > 0 ? (
              <dl className="grid grid-cols-2 xl:grid-cols-4 gap-x-4 gap-y-3">
                {primarySpecifications.map(([name, value]) => (
                  <div
                    key={name}
                    className="min-w-0 rounded-xl border border-bd bg-surface-2 px-3 py-2"
                  >
                    <dt className="text-[10px] uppercase tracking-[0.14em] text-muted">{name}</dt>
                    <dd className="mt-2 text-sm font-semibold text-text break-words">{value}</dd>
                  </div>
                ))}
              </dl>
            ) : (
              <p className="text-sm text-muted">No hay características informadas.</p>
            )}
          </div>

          <div
            style={{
              background: "linear-gradient(180deg, var(--color-surface-2), var(--color-surface))",
              border: "1px solid var(--color-bd)",
            }}
            className="rounded-xl p-4"
          >
            <div className="text-xs font-semibold uppercase tracking-[0.14em] text-prime">
              Precio del servicio
            </div>
            <div className="mt-2 text-3xl font-black text-prime">
              {formatPrice(service.monthlyPrice)}
              <span className="text-sm font-medium text-muted">
                {" "}
                {getBillingPeriodText(service.billingPeriod)}
              </span>
            </div>
            <dl className="mt-3 grid grid-cols-2 gap-3 border-t border-bd pt-3 text-xs">
              <div>
                <dt className="text-muted">Instalación</dt>
                <dd className="mt-1 font-semibold text-text">
                  {service.installationCost === null || service.installationCost === 0
                    ? "Gratis"
                    : formatPrice(service.installationCost)}
                </dd>
              </div>
              <div>
                <dt className="text-muted">Contrato</dt>
                <dd className="mt-1 font-semibold text-text">
                  {service.contractMonths ? `${service.contractMonths} meses` : "Sin permanencia"}
                </dd>
              </div>
            </dl>
          </div>

          {serviceUrl && (
            <a
              href={serviceUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="flex items-center justify-center gap-2 rounded-xl border border-prime/50 bg-prime-muted px-5 py-3 text-sm font-semibold text-prime transition hover:bg-prime/15 focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-prime"
            >
              Ver en la tienda
              <svg aria-hidden="true" viewBox="0 0 20 20" fill="none" className="h-4 w-4">
                <path
                  d="M11 4h5v5m0-5-7 7M15 11v4a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V6a1 1 0 0 1 1-1h4"
                  stroke="currentColor"
                  strokeWidth="1.7"
                  strokeLinecap="round"
                  strokeLinejoin="round"
                />
              </svg>
            </a>
          )}

          <button
            type="button"
            onClick={() => void addServiceToCart()}
            aria-live="polite"
            aria-disabled={isInCart || adding}
            className={`rounded-xl px-5 py-3 text-sm font-semibold shadow-md transition ${
              isInCart
                ? "bg-emerald-700 text-emerald-100 shadow-emerald-950/30"
                : adding
                  ? "bg-emerald-800 text-emerald-100 shadow-emerald-950/30"
                  : "bg-prime text-[#181714] shadow-black/20 hover:bg-prime-dark"
            } ${isInCart || adding ? "cursor-default" : ""}`}
          >
            {isInCart
              ? "Agregado en la cesta"
              : adding
                ? "Agregando al carrito..."
                : "Agregar al carrito"}
          </button>

          <button
            onClick={() => onToggleCompare(service.id)}
            style={
              isComparing
                ? {
                    background: "var(--color-action)",
                    color: "#181714",
                  }
                : {
                    background: "var(--color-surface)",
                    border: "1px solid var(--color-bd)",
                    color: "var(--color-text)",
                  }
            }
            className="py-3 rounded-2xl text-sm font-semibold transition-all hover:opacity-95"
          >
            {isComparing ? "✓ Agregado al comparador" : "Agregar al comparador"}
          </button>
        </div>
      </div>

      <PriceHistory history={service.priceHistory} offerHistory={[]} />

      <section
        style={{
          background: "linear-gradient(180deg, var(--color-surface-2), var(--color-surface))",
          border: "1px solid var(--color-bd)",
        }}
        className="rounded-xl p-5 sm:p-6 mt-6"
      >
        <h2 className="text-lg font-semibold text-text">Especificaciones del servicio</h2>

        {specificationGroups.length > 0 ? (
          <div className="mt-4 space-y-5">
            {specificationGroups.map((group) => (
              <section key={group.title}>
                <h3 className="text-xs font-semibold uppercase tracking-[0.16em] text-prime">
                  {group.title}
                </h3>
                <dl className="mt-2 grid grid-cols-1 md:grid-cols-2 gap-x-8">
                  {group.items.map(([name, value]) => (
                    <div
                      key={name}
                      className="flex items-start justify-between gap-4 border-b border-bd py-3"
                    >
                      <dt className="min-w-0 text-sm text-muted">{name}</dt>
                      <dd className="max-w-[60%] text-right text-sm font-semibold text-text break-words">
                        {value}
                      </dd>
                    </div>
                  ))}
                </dl>
              </section>
            ))}
          </div>
        ) : (
          <p className="mt-4 text-sm text-muted">
            No hay especificaciones informadas para este servicio.
          </p>
        )}
      </section>

      <section
        style={{
          background: "linear-gradient(180deg, var(--color-surface-2), var(--color-surface))",
          border: "1px solid var(--color-bd)",
        }}
        className="rounded-xl p-5 sm:p-6 mt-6"
      >
        <h2 className="text-lg font-semibold text-text">Beneficios y cobertura</h2>
        {service.benefits.length > 0 ? (
          <ul className="mt-4 grid grid-cols-1 gap-3 sm:grid-cols-2">
            {service.benefits.map((benefit) => (
              <li
                key={benefit}
                className="flex items-start gap-2 rounded-xl border border-bd bg-surface-2 px-3 py-3 text-sm text-text"
              >
                <svg
                  className="mt-0.5 shrink-0"
                  width="14"
                  height="14"
                  viewBox="0 0 24 24"
                  fill="none"
                  stroke="#D1A85A"
                  strokeWidth="3"
                  aria-hidden="true"
                >
                  <path d="M20 6 9 17l-5-5" />
                </svg>
                {benefit}
              </li>
            ))}
          </ul>
        ) : (
          <p className="mt-4 text-sm text-muted">No hay beneficios informados.</p>
        )}
        <p className="mt-5 border-t border-bd pt-4 text-sm text-muted">
          <span className="font-semibold text-text">Cobertura: </span>
          {service.coverage || "No informada"}
        </p>
      </section>
    </div>
  );
}
