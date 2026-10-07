import { useState } from "react";
import type { Page, Product, StoreOffer } from "../../types";
import { formatPrice } from "../../services/utils/productUtils";
import { Breadcrumb } from "../../components/common/ui";

interface Props {
  product: Product;
  onAddToCart: (product: Product, offer: StoreOffer) => void;
  navigate: (page: Page) => void;
  comparisonOnly?: boolean;
  initialSelectedStoreIds?: string[];
}

const getTotal = (offer: StoreOffer): number | null => {
  const shipping = offer.shippingFree ? 0 : offer.shipping;
  return shipping === null ? null : offer.price + shipping;
};

const getSafeOfferUrl = (url: string | undefined): string | null => {
  if (!url) return null;

  try {
    const parsed = new URL(url);
    return parsed.protocol === "http:" || parsed.protocol === "https:" ? url : null;
  } catch {
    return null;
  }
};

const shippingLabel = (offer: StoreOffer): string => {
  if (offer.shippingFree) return "Gratis";
  if (offer.shipping === null) return "No informado";
  return formatPrice(offer.shipping);
};

const conditionLabel = (condition: string | undefined): string => {
  if (!condition) return "No informado";

  const labels: Record<string, string> = {
    new: "Nuevo",
    used: "Usado",
    refurbished: "Reacondicionado",
  };

  return labels[condition.toLowerCase()] ?? condition;
};

const relativeUpdate = (value: string | undefined): string => {
  if (!value) return "Actualización no informada";

  const timestamp = Date.parse(value);
  if (!Number.isFinite(timestamp)) return `Actualización: ${value}`;

  const elapsedSeconds = Math.round((Date.now() - timestamp) / 1000);
  if (Math.abs(elapsedSeconds) < 60) {
    return elapsedSeconds < 0 ? "Actualización futura" : "Actualizado hace menos de un minuto";
  }

  const units: [Intl.RelativeTimeFormatUnit, number][] = [
    ["year", 31_536_000],
    ["month", 2_592_000],
    ["day", 86_400],
    ["hour", 3_600],
    ["minute", 60],
  ];
  const [unit, secondsPerUnit] = units.find(([, seconds]) => Math.abs(elapsedSeconds) >= seconds)!;
  const count = Math.round(elapsedSeconds / secondsPerUnit);
  return `Actualizado ${new Intl.RelativeTimeFormat("es", { numeric: "auto" }).format(-count, unit)}`;
};

interface SelectedOffersTableProps {
  product: Product;
  offers: StoreOffer[];
  bestOffers: StoreOffer[];
  onAddToCart: (product: Product, offer: StoreOffer) => void;
  onRemoveOffer: (storeId: string) => void;
}

function SelectedOffersTable({
  product,
  offers,
  bestOffers,
  onAddToCart,
  onRemoveOffer,
}: SelectedOffersTableProps) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[760px] border-separate border-spacing-1.5 text-left">
        <thead>
          <tr>
            <th className="w-44 rounded-xl border border-slate-700 bg-slate-800/90 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.18em] text-slate-200">
              Tienda
            </th>
            {offers.map((offer) => {
              const isBest = bestOffers.some((best) => best.storeId === offer.storeId);
              return (
                <th
                  key={offer.storeId}
                  className="min-w-40 rounded-xl border border-slate-700 bg-gradient-to-br from-slate-800/95 to-slate-900/95 px-4 py-5 text-center"
                >
                  <span className="block text-sm font-bold text-slate-100">{offer.storeName}</span>
                  {isBest && (
                    <span className="mt-2 inline-flex rounded-full border border-cyan-200/50 bg-cyan-200 px-2 py-0.5 text-[9px] font-extrabold uppercase tracking-wider text-slate-950">
                      Menor total
                    </span>
                  )}
                </th>
              );
            })}
          </tr>
        </thead>
        <tbody>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Precio
            </th>
            {offers.map((offer) => {
              const discounted =
                offer.listPrice !== null &&
                offer.listPrice !== undefined &&
                offer.listPrice > offer.price;
              const isBest = bestOffers.some((best) => best.storeId === offer.storeId);
              return (
                <td
                  key={offer.storeId}
                  className={`rounded-xl border p-3 text-center ${
                    isBest
                      ? "border-cyan-300/70 bg-gradient-to-br from-cyan-400/20 via-cyan-300/10 to-violet-500/20 shadow-[0_0_22px_rgba(34,211,238,0.18)]"
                      : "border-slate-800 bg-slate-950/65"
                  }`}
                >
                  {discounted ? (
                    <>
                      <span className="block text-[11px] font-medium text-cyan-200">
                        Precio oferta
                      </span>
                      <span className="text-sm font-bold text-slate-100">
                        {formatPrice(offer.price)}
                      </span>
                      <span className="block text-xs text-slate-400">
                        Normal <span className="line-through">{formatPrice(offer.listPrice!)}</span>
                      </span>
                    </>
                  ) : (
                    <>
                      <span className="block text-[11px] text-slate-400">Precio normal</span>
                      <span className="text-sm font-bold text-slate-100">
                        {formatPrice(offer.price)}
                      </span>
                    </>
                  )}
                </td>
              );
            })}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Envío
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center text-sm text-slate-200"
              >
                {shippingLabel(offer)}
              </td>
            ))}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Total
            </th>
            {offers.map((offer) => {
              const total = getTotal(offer);
              const isBest = bestOffers.some((best) => best.storeId === offer.storeId);
              return (
                <td
                  key={offer.storeId}
                  className={`rounded-xl border p-3 text-center text-base font-extrabold ${
                    isBest
                      ? "border-cyan-300/70 bg-gradient-to-br from-cyan-400/20 via-cyan-300/10 to-violet-500/20 text-cyan-100 shadow-[0_0_22px_rgba(34,211,238,0.18)]"
                      : "border-slate-800 bg-slate-950/65 text-slate-100"
                  }`}
                >
                  {total === null ? "No informado" : formatPrice(total)}
                </td>
              );
            })}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Entrega
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center text-sm text-slate-200"
              >
                {offer.deliveryTime || "No informado"}
              </td>
            ))}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Garantía
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center text-sm text-slate-200"
              >
                {offer.warranty || "No informado"}
              </td>
            ))}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Disponibilidad
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center text-sm"
              >
                <span className={offer.available ? "text-emerald-300" : "text-slate-400"}>
                  {offer.available ? "Disponible" : "No disponible"}
                </span>
              </td>
            ))}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Actualización
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center text-sm text-slate-400"
              >
                {relativeUpdate(offer.lastUpdated)}
              </td>
            ))}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Comprar
            </th>
            {offers.map((offer) => {
              const safeUrl = getSafeOfferUrl(offer.url);
              return (
                <td
                  key={offer.storeId}
                  className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center"
                >
                  {safeUrl ? (
                    <a
                      href={safeUrl}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="text-sm font-bold text-violet-300 hover:text-cyan-200 hover:underline"
                    >
                      Comprar oferta
                    </a>
                  ) : (
                    <span className="text-sm text-slate-400">Enlace no disponible</span>
                  )}
                </td>
              );
            })}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Carrito
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center"
              >
                <button
                  type="button"
                  disabled={!offer.available}
                  onClick={() => onAddToCart(product, offer)}
                  className="rounded-lg bg-gradient-to-r from-[#ff9878] to-[#fb7185] px-3 py-2 text-xs font-semibold text-white shadow-md shadow-rose-500/15 transition hover:brightness-105 disabled:cursor-not-allowed disabled:from-slate-700 disabled:to-slate-700 disabled:text-slate-400 disabled:shadow-none"
                >
                  Agregar al carrito
                </button>
              </td>
            ))}
          </tr>
          <tr>
            <th
              scope="row"
              className="rounded-xl border border-slate-700 bg-slate-800/70 p-4 text-left text-[11px] font-semibold uppercase tracking-[0.12em] text-slate-200"
            >
              Acciones
            </th>
            {offers.map((offer) => (
              <td
                key={offer.storeId}
                className="rounded-xl border border-slate-800 bg-slate-950/65 p-3 text-center"
              >
                <button
                  type="button"
                  onClick={() => onRemoveOffer(offer.storeId)}
                  aria-label={`Quitar producto de ${offer.storeName}`}
                  className="rounded-lg border border-rose-300/40 bg-rose-950/40 px-3 py-1.5 text-xs font-semibold text-rose-200 transition hover:border-rose-200 hover:bg-rose-900/60 hover:text-white"
                >
                  Quitar producto
                </button>
              </td>
            ))}
          </tr>
        </tbody>
      </table>
    </div>
  );
}

export default function PriceOfferComparison({
  product,
  onAddToCart,
  navigate,
  comparisonOnly = false,
  initialSelectedStoreIds = [],
}: Props) {
  const [selectedStoreIds, setSelectedStoreIds] = useState<string[]>(initialSelectedStoreIds);
  const offers = product.offers;
  const selectedOffers = offers.filter((offer) => selectedStoreIds.includes(offer.storeId));
  const hasUnknownShipping = offers.some((offer) => offer.available && getTotal(offer) === null);
  const selectedAvailableOffers = selectedOffers.filter((offer) => offer.available);
  const selectedHasUnknownShipping = selectedAvailableOffers.some(
    (offer) => getTotal(offer) === null,
  );

  const selectedAvailableTotals = selectedAvailableOffers
    .map((offer) => ({ offer, total: getTotal(offer) }))
    .filter((entry): entry is { offer: StoreOffer; total: number } => entry.total !== null);
  const selectedLowestTotal = selectedAvailableTotals.reduce<number | null>(
    (lowest, entry) => (lowest === null || entry.total < lowest ? entry.total : lowest),
    null,
  );
  const selectedBestOffers = selectedHasUnknownShipping
    ? []
    : selectedAvailableTotals
        .filter((entry) => entry.total === selectedLowestTotal)
        .map((entry) => entry.offer);
  const availableTotals = offers
    .filter((offer) => offer.available)
    .map((offer) => ({ offer, total: getTotal(offer) }))
    .filter((entry): entry is { offer: StoreOffer; total: number } => entry.total !== null);
  const lowestTotal = availableTotals.reduce<number | null>(
    (lowest, entry) => (lowest === null || entry.total < lowest ? entry.total : lowest),
    null,
  );
  const lowestOffers = hasUnknownShipping
    ? []
    : availableTotals.filter((entry) => entry.total === lowestTotal).map((entry) => entry.offer);

  const toggleOffer = (offer: StoreOffer) => {
    setSelectedStoreIds((current) =>
      current.includes(offer.storeId)
        ? current.filter((storeId) => storeId !== offer.storeId)
        : [...current, offer.storeId],
    );
  };

  if (comparisonOnly) {
    return (
      <div className="mx-auto max-w-7xl px-4 py-8">
        <Breadcrumb
          items={[
            { label: "Inicio", onClick: () => navigate({ id: "home" }) },
            { label: "Productos", onClick: () => navigate({ id: "search-products", query: "" }) },
            {
              label: product.name,
              onClick: () => navigate({ id: "product-detail", productId: product.id }),
            },
            { label: "Comparación" },
          ]}
        />

        <h1 className="mb-2 mt-6 text-2xl font-bold text-slate-100">Comparación de ofertas</h1>
        <p className="mb-8 text-sm text-slate-300">
          Comparando {selectedOffers.length} ofertas de {product.name}
        </p>

        <section className="rounded-[28px] border border-slate-600/80 bg-gradient-to-br from-slate-900/95 via-slate-950/90 to-indigo-950/45 p-3 shadow-[0_24px_70px_rgba(2,6,23,0.55)] backdrop-blur-sm sm:p-5">
          <p className="mb-4 flex flex-wrap items-center gap-2 rounded-xl border border-slate-700/70 bg-slate-950/45 px-3 py-2 text-[11px] text-slate-200">
            <span className="inline-flex items-center gap-1 rounded-full border border-cyan-200/50 bg-cyan-300 px-2 py-1 font-extrabold text-slate-950">
              ✦ Menor total
            </span>
            La mejor oferta considera el precio del producto y su costo de envío.
          </p>
          {selectedOffers.length > 0 ? (
            <SelectedOffersTable
              product={product}
              offers={selectedOffers}
              bestOffers={selectedBestOffers}
              onAddToCart={onAddToCart}
              onRemoveOffer={(storeId) =>
                setSelectedStoreIds((current) => current.filter((id) => id !== storeId))
              }
            />
          ) : (
            <div className="py-8 text-center">
              <p className="mb-4 text-sm text-slate-300">
                Ya no hay ofertas seleccionadas para comparar.
              </p>
              <button
                type="button"
                onClick={() => navigate({ id: "product-detail", productId: product.id })}
                className="rounded-lg border border-slate-600 bg-slate-800 px-4 py-2 text-sm font-semibold text-slate-100 transition hover:border-cyan-400 hover:text-cyan-200"
              >
                Volver al producto
              </button>
            </div>
          )}
        </section>
      </div>
    );
  }

  return (
    <div className="space-y-5 mb-6">
      <section
        aria-labelledby="lowest-offer-heading"
        style={{
          background: "linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.9))",
          border: "1px solid rgba(148, 163, 184, 0.2)",
          boxShadow: "0 12px 30px rgba(15, 23, 42, 0.08)",
        }}
        className="rounded-xl p-4 sm:p-5"
      >
        <div className="mb-4 flex flex-wrap items-center justify-between gap-2">
          <h2 id="lowest-offer-heading" className="text-base font-bold text-white">
            Precio oferta más bajo
          </h2>
          {lowestOffers.length > 1 && (
            <span className="rounded-md border border-violet-400/30 bg-violet-500/10 px-2 py-1 text-xs font-semibold text-violet-200">
              Empate entre {lowestOffers.length} ofertas
            </span>
          )}
        </div>

        {lowestOffers.length > 0 && lowestTotal !== null ? (
          <div className="grid grid-cols-1 gap-3 md:grid-cols-2">
            {lowestOffers.map((offer) => {
              const safeUrl = getSafeOfferUrl(offer.url);
              const discounted =
                offer.listPrice !== null &&
                offer.listPrice !== undefined &&
                offer.listPrice > offer.price;

              return (
                <article
                  key={offer.storeId}
                  className="rounded-lg border border-slate-700/80 bg-slate-900/70 p-4 shadow-[0_12px_30px_rgba(15,23,42,0.1)]"
                >
                  <div className="flex flex-wrap items-start justify-between gap-3">
                    <div>
                      <p className="text-xs text-slate-400">Tienda</p>
                      <h3 className="text-base font-semibold text-white">{offer.storeName}</h3>
                    </div>
                    <div className="text-right">
                      <p className="text-xs text-slate-400">Precio total</p>
                      <p className="price text-xl font-bold text-violet-200">
                        {formatPrice(getTotal(offer)!)}
                      </p>
                    </div>
                  </div>

                  <dl className="mt-4 grid grid-cols-2 gap-x-4 gap-y-3 sm:grid-cols-3">
                    <div>
                      <dt className="text-xs text-slate-400">Precio del producto</dt>
                      <dd className="text-sm font-medium text-slate-100">
                        {formatPrice(offer.price)}
                      </dd>
                      {discounted && (
                        <dd className="text-xs text-slate-400 line-through">
                          Normal {formatPrice(offer.listPrice!)}
                        </dd>
                      )}
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Costo de envío</dt>
                      <dd
                        className={
                          offer.shippingFree
                            ? "text-sm font-medium text-emerald-300"
                            : "text-sm text-slate-100"
                        }
                      >
                        {shippingLabel(offer)}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Envío gratis</dt>
                      <dd className="text-sm text-slate-100">{offer.shippingFree ? "Sí" : "No"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Entrega</dt>
                      <dd className="text-sm text-slate-100">
                        {offer.deliveryTime || "No informado"}
                      </dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Garantía</dt>
                      <dd className="text-sm text-slate-100">{offer.warranty || "No informado"}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Disponibilidad</dt>
                      <dd
                        className={
                          offer.available
                            ? "text-sm font-medium text-emerald-300"
                            : "text-sm text-slate-400"
                        }
                      >
                        {offer.available ? "Disponible" : "No disponible"}
                      </dd>
                    </div>
                    {offer.stock !== null && offer.stock !== undefined && (
                      <div>
                        <dt className="text-xs text-slate-400">Stock</dt>
                        <dd className="text-sm text-slate-100">{offer.stock} unidades</dd>
                      </div>
                    )}
                    <div>
                      <dt className="text-xs text-slate-400">Condición</dt>
                      <dd className="text-sm text-slate-100">{conditionLabel(offer.condition)}</dd>
                    </div>
                    <div>
                      <dt className="text-xs text-slate-400">Última actualización</dt>
                      <dd className="text-sm text-slate-100">
                        {relativeUpdate(offer.lastUpdated)}
                      </dd>
                    </div>
                  </dl>

                  <div className="mt-4 flex flex-wrap gap-2 border-t border-slate-700/80 pt-4">
                    {safeUrl ? (
                      <a
                        href={safeUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="inline-flex items-center justify-center rounded-md bg-gradient-to-r from-[#ff7a59] to-[#8b5cf6] px-3 py-2 text-sm font-semibold text-white hover:opacity-90"
                      >
                        Comprar ahora
                      </a>
                    ) : (
                      <button
                        type="button"
                        disabled
                        className="rounded-md bg-slate-700 px-3 py-2 text-sm font-semibold text-slate-400"
                        title="La tienda no proporcionó una URL de compra"
                      >
                        Comprar ahora
                      </button>
                    )}
                    <button
                      type="button"
                      disabled={!offer.available}
                      onClick={() => onAddToCart(product, offer)}
                      className="rounded-md border border-slate-600 bg-slate-800 px-3 py-2 text-sm font-semibold text-slate-100 hover:border-violet-400 hover:text-violet-200 disabled:cursor-not-allowed disabled:opacity-40"
                    >
                      Agregar al carrito
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        ) : (
          <p className="text-sm text-muted">
            {offers.some((offer) => offer.available)
              ? "No determinable: falta el costo de envío de una oferta disponible."
              : "No hay ofertas disponibles."}
          </p>
        )}
      </section>

      <section
        aria-labelledby="compare-offers-heading"
        style={{
          background: "linear-gradient(180deg, rgba(15, 23, 42, 0.96), rgba(30, 41, 59, 0.9))",
          border: "1px solid rgba(148, 163, 184, 0.2)",
          boxShadow: "0 12px 30px rgba(15, 23, 42, 0.08)",
        }}
        className="rounded-2xl p-5 sm:p-6"
      >
        <div className="flex flex-wrap items-end justify-between gap-3 mb-4">
          <div>
            <h2 id="compare-offers-heading" className="text-lg font-bold text-white">
              Comparar ofertas
            </h2>
            <p className="text-sm text-slate-300 mt-1">
              Selecciona una o varias tiendas para comparar sus condiciones.
            </p>
          </div>
          <div className="flex flex-wrap items-center gap-3">
            <span className="text-xs text-slate-300">
              {selectedOffers.length > 0
                ? `${selectedOffers.length} seleccionada${selectedOffers.length === 1 ? "" : "s"}`
                : "Mostrando todas"}
            </span>
            <button
              type="button"
              disabled={selectedOffers.length < 2}
              onClick={() =>
                navigate({
                  id: "offer-comparison",
                  product,
                  storeIds: selectedStoreIds,
                })
              }
              className="rounded-md bg-gradient-to-r from-[#ff7a59] to-[#8b5cf6] px-3 py-2 text-xs font-semibold text-white transition-opacity hover:opacity-90 disabled:cursor-not-allowed disabled:opacity-40"
            >
              Comparar seleccionadas
            </button>
          </div>
        </div>

        {offers.length === 0 ? (
          <p className="text-sm text-muted py-4">Este producto todavía no tiene ofertas.</p>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full min-w-[1120px] border-collapse text-left">
              <thead>
                <tr className="border-b border-slate-700/80">
                  <th className="py-3 pr-4 text-xs font-semibold uppercase text-slate-300">
                    Tienda
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Precio
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Envío
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Total
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Entrega
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Garantía
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Disponibilidad
                  </th>
                  <th className="px-4 py-3 text-xs font-semibold uppercase text-slate-300">
                    Actualización
                  </th>
                  <th className="py-3 pl-3 text-xs font-semibold uppercase text-slate-300">
                    Comprar
                  </th>
                </tr>
              </thead>
              <tbody>
                {offers.map((offer) => {
                  const isSelected = selectedStoreIds.includes(offer.storeId);
                  const total = getTotal(offer);
                  const isLowest = lowestOffers.some(
                    (lowestOffer) => lowestOffer.storeId === offer.storeId,
                  );
                  const isDiscounted =
                    offer.listPrice !== null &&
                    offer.listPrice !== undefined &&
                    offer.listPrice > offer.price;
                  const safeUrl = getSafeOfferUrl(offer.url);

                  return (
                    <tr
                      key={offer.storeId}
                      className={`border-b border-slate-700/80 last:border-0 ${isSelected ? "bg-slate-800/60" : "bg-slate-900/20"}`}
                    >
                      <th scope="row" className="py-4 pr-4 font-normal">
                        <label className="flex items-center gap-3 text-sm text-slate-100 cursor-pointer">
                          <input
                            type="checkbox"
                            checked={isSelected}
                            onChange={() => toggleOffer(offer)}
                            className="h-4 w-4 accent-violet-500"
                            aria-label={`Comparar ${offer.storeName}`}
                          />
                          <span className="whitespace-nowrap">{offer.storeName}</span>
                          {isLowest && (
                            <span className="text-[11px] font-semibold text-violet-200">
                              Menor total
                            </span>
                          )}
                        </label>
                      </th>
                      <td className="px-4 py-4">
                        {isDiscounted ? (
                          <div className="min-w-36">
                            <span className="block text-[11px] text-violet-200">Precio oferta</span>
                            <span className="price text-sm font-bold text-white">
                              {formatPrice(offer.price)}
                            </span>
                            <span className="mt-0.5 flex items-center gap-1.5 text-[11px] text-slate-400">
                              <span>Normal</span>
                              <span className="line-through">{formatPrice(offer.listPrice!)}</span>
                            </span>
                          </div>
                        ) : (
                          <div className="whitespace-nowrap">
                            <span className="block text-[11px] text-slate-400">Precio normal</span>
                            <span className="price text-sm font-semibold text-white">
                              {formatPrice(offer.price)}
                            </span>
                          </div>
                        )}
                      </td>
                      <td className="px-4 py-4 text-sm text-slate-100 whitespace-nowrap">
                        {shippingLabel(offer)}
                      </td>
                      <td
                        className={`px-4 py-4 text-sm font-bold whitespace-nowrap ${isLowest ? "text-violet-200" : "text-white"}`}
                      >
                        {total === null ? "No informado" : formatPrice(total)}
                      </td>
                      <td className="px-4 py-4 text-sm text-slate-300 whitespace-nowrap">
                        {offer.deliveryTime || "No informado"}
                      </td>
                      <td className="px-4 py-4 text-sm text-slate-300 whitespace-nowrap">
                        {offer.warranty || "No informado"}
                      </td>
                      <td className="px-4 py-4 text-sm whitespace-nowrap">
                        <span className={offer.available ? "text-emerald-300" : "text-slate-400"}>
                          {offer.available ? "Disponible" : "No disponible"}
                        </span>
                      </td>
                      <td className="px-4 py-4 text-sm text-slate-300 whitespace-nowrap">
                        {relativeUpdate(offer.lastUpdated)}
                      </td>
                      <td className="py-4 pl-3">
                        {safeUrl ? (
                          <a
                            href={safeUrl}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="inline-flex items-center gap-1.5 rounded-md border border-slate-600 bg-slate-800 px-2.5 py-1.5 text-xs font-semibold text-slate-100 transition-colors hover:border-violet-400 hover:text-violet-200 whitespace-nowrap"
                          >
                            Comprar oferta
                            <span aria-hidden="true">↗</span>
                          </a>
                        ) : (
                          <span className="text-xs text-slate-400 whitespace-nowrap">
                            Enlace no disponible
                          </span>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        )}
      </section>
    </div>
  );
}
