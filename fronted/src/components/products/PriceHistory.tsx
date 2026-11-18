import { useState } from "react";
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  Tooltip,
  ResponsiveContainer,
  CartesianGrid,
} from "recharts";
import type { PricePoint } from "../../types";
import { detectPriceAnomalies, formatPrice } from "../../services/utils/productUtils";

const ranges = [
  { label: "7d", days: 7 },
  { label: "30d", days: 30 },
  { label: "3m", days: 90 },
  { label: "6m", days: 180 },
  { label: "1a", days: 365 },
];

const chartColors = {
  accent: "var(--color-prime)",
  border: "var(--color-bd)",
  muted: "var(--color-muted)",
  surface: "var(--color-surface)",
};

interface Props {
  history: PricePoint[];
  offerHistory: PricePoint[];
  currentOfferPrice?: number;
}

function CustomTooltip({
  active,
  payload,
  label,
}: {
  active?: boolean;
  payload?: Array<{ value: number }>;
  label?: string;
}) {
  if (active && payload && payload.length) {
    return (
      <div className="rounded-xl border border-bd bg-surface px-3 py-2 shadow-xl">
        <p className="mb-1 text-xs text-muted">{formatHistoryDate(label)}</p>
        <p className="price text-sm font-semibold text-prime">{formatPrice(payload[0].value)}</p>
      </div>
    );
  }
  return null;
}

function formatHistoryDate(value: string | undefined) {
  if (!value) return "";
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : date.toLocaleDateString("es-CL", { day: "2-digit", month: "short", year: "numeric" });
}

export default function PriceHistory({ history, offerHistory, currentOfferPrice }: Props) {
  const [range, setRange] = useState(365);
  const [mode, setMode] = useState<"price" | "offer">("price");

  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - range);

  const activeData = (mode === "price" ? history : offerHistory)
    .filter((point) => new Date(point.date) >= cutoff)
    .sort((first, second) => Date.parse(first.date) - Date.parse(second.date));
  const { hasAbruptHistoryDrop } = detectPriceAnomalies([], activeData);

  const hasOfferHistory = offerHistory.length > 0;
  const prices = activeData.map((d) => d.price);
  const minPrice = prices.length ? Math.min(...prices) : 0;
  const maxPrice = prices.length ? Math.max(...prices) : 0;
  const currentPrice = activeData.length ? activeData[activeData.length - 1].price : 0;

  return (
    <section className="rounded-2xl border border-bd bg-surface p-4 shadow-xl shadow-black/20 sm:p-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-semibold text-text">Historial de precios</h3>
          <p className="mt-1 text-xs text-muted">Evolución registrada durante el último año</p>
          <div className="mt-3 flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => setMode("price")}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition ${
                mode === "price"
                  ? "bg-prime text-[#181714]"
                  : "border border-bd bg-surface-2 text-muted hover:border-prime/60"
              }`}
            >
              Precio normal
            </button>
            <button
              type="button"
              onClick={() => setMode("offer")}
              disabled={!hasOfferHistory}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition ${
                mode === "offer"
                  ? "bg-prime text-[#181714]"
                  : hasOfferHistory
                    ? "border border-bd bg-surface-2 text-muted hover:border-prime/60"
                    : "cursor-not-allowed border border-bd bg-surface-2/50 text-muted/50"
              }`}
            >
              Precio oferta
            </button>
          </div>
        </div>

        <div className="flex flex-wrap gap-1.5">
          {ranges.map((r) => (
            <button
              key={r.days}
              type="button"
              onClick={() => setRange(r.days)}
              aria-pressed={range === r.days}
              className={`rounded-lg px-2.5 py-1.5 text-xs font-semibold transition ${
                range === r.days
                  ? "bg-prime text-[#181714]"
                  : "border border-bd bg-surface-2 text-muted hover:border-prime/60"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {hasAbruptHistoryDrop && (
        <p
          className="mb-4 inline-flex rounded-full border border-amber-400 bg-amber-900 px-3 py-1.5 text-xs font-semibold text-amber-50"
          title="El precio más reciente cayó a la mitad o menos respecto al registro anterior."
        >
          Precio inusual
        </p>
      )}

      {mode === "offer" && !hasOfferHistory && (
        <div className="rounded-xl border border-bd bg-surface-2 p-6 text-center">
          <p className="mb-1 text-sm font-semibold text-text">No hay historial de ofertas</p>
          <p className="mx-auto max-w-xs text-xs text-muted">
            Este producto no ha registrado precios de oferta. Puedes agregarlo a favoritos para
            recibir una alerta cuando haya una promoción.
          </p>
        </div>
      )}

      {mode === "offer" && hasOfferHistory && currentOfferPrice && (
        <div className="mb-4 flex flex-wrap items-center gap-3 rounded-xl border border-prime/30 bg-prime-muted px-4 py-2">
          <span className="rounded-md bg-prime px-2 py-0.5 text-xs font-bold text-[#181714]">
            OFERTA ACTIVA
          </span>
          <span className="price text-sm font-bold text-prime">
            {formatPrice(currentOfferPrice)}
          </span>
          <span className="text-xs text-muted">precio de oferta actual</span>
        </div>
      )}

      {activeData.length > 0 && (
        <div className="mb-5 grid grid-cols-1 gap-2 sm:grid-cols-3">
          {[
            {
              label: mode === "offer" ? "Última oferta" : "Precio actual",
              value: formatPrice(currentPrice),
              color: "text-prime",
            },
            {
              label: "Mínimo registrado",
              value: formatPrice(minPrice),
              color: "text-text",
            },
            {
              label: "Máximo registrado",
              value: formatPrice(maxPrice),
              color: "text-muted",
            },
          ].map((stat) => (
            <div key={stat.label} className="rounded-xl border border-bd bg-surface-2 p-3">
              <div className="mb-1 text-xs text-muted">{stat.label}</div>
              <div className={`price text-sm font-semibold ${stat.color}`}>{stat.value}</div>
            </div>
          ))}
        </div>
      )}

      {activeData.length > 0 && (
        <div className="h-52 rounded-xl border border-bd bg-surface-2 p-2 sm:h-60 sm:p-3">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={activeData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
              <defs>
                <linearGradient id="priceGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor={chartColors.accent} stopOpacity={0.24} />
                  <stop offset="95%" stopColor={chartColors.accent} stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke={chartColors.border} vertical={false} />
              <XAxis
                dataKey="date"
                tickFormatter={formatHistoryDate}
                tick={{ fontSize: 10, fill: chartColors.muted }}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fontSize: 10, fill: chartColors.muted }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(v) => `$${(v / 1000).toFixed(0)}k`}
                width={48}
              />
              <Tooltip content={<CustomTooltip />} />
              <Area
                type="monotone"
                dataKey="price"
                stroke={chartColors.accent}
                strokeWidth={2}
                fill="url(#priceGrad)"
                dot={
                  activeData.length < 24
                    ? { r: 3, fill: chartColors.accent, strokeWidth: 0 }
                    : false
                }
                activeDot={{
                  r: 4,
                  fill: chartColors.accent,
                  stroke: chartColors.surface,
                  strokeWidth: 2,
                }}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
      {activeData.length === 0 && !(mode === "offer" && !hasOfferHistory) && (
        <p className="rounded-xl border border-bd bg-surface-2 px-4 py-6 text-center text-sm text-muted">
          No hay datos de precios registrados para este período.
        </p>
      )}
    </section>
  );
}
