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
import { formatPrice } from "../../services/utils/productUtils";

const ranges = [
  { label: "7d", days: 7 },
  { label: "30d", days: 30 },
  { label: "3m", days: 90 },
  { label: "6m", days: 180 },
  { label: "1a", days: 365 },
];

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
      <div className="rounded-xl border border-slate-600 bg-slate-900 px-3 py-2 shadow-xl">
        <p className="mb-1 text-xs text-slate-400">{formatHistoryDate(label)}</p>
        <p className="price text-sm font-semibold text-cyan-200">{formatPrice(payload[0].value)}</p>
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

  const hasOfferHistory = offerHistory.length > 0;
  const prices = activeData.map((d) => d.price);
  const minPrice = prices.length ? Math.min(...prices) : 0;
  const maxPrice = prices.length ? Math.max(...prices) : 0;
  const currentPrice = activeData.length ? activeData[activeData.length - 1].price : 0;

  return (
    <section className="rounded-2xl border border-slate-700/80 bg-gradient-to-br from-slate-900/95 via-slate-950/95 to-indigo-950/30 p-4 shadow-xl shadow-slate-950/20 sm:p-6">
      <div className="mb-5 flex flex-wrap items-start justify-between gap-4">
        <div>
          <h3 className="text-base font-semibold text-white">Historial de precios</h3>
          <p className="mt-1 text-xs text-slate-400">Evolución registrada durante el último año</p>
          <div className="mt-3 flex flex-wrap gap-1.5">
            <button
              type="button"
              onClick={() => setMode("price")}
              className={`rounded-lg px-3 py-1.5 text-xs font-semibold transition ${
                mode === "price"
                  ? "bg-cyan-300 text-slate-950"
                  : "border border-slate-700 bg-slate-900/70 text-slate-300 hover:border-slate-500"
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
                  ? "bg-cyan-300 text-slate-950"
                  : hasOfferHistory
                    ? "border border-slate-700 bg-slate-900/70 text-slate-300 hover:border-slate-500"
                    : "cursor-not-allowed border border-slate-800 bg-slate-950/50 text-slate-600"
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
                  ? "bg-cyan-300 text-slate-950"
                  : "border border-slate-700 bg-slate-900/70 text-slate-300 hover:border-slate-500"
              }`}
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {mode === "offer" && !hasOfferHistory && (
        <div className="rounded-xl border border-slate-700 bg-slate-950/50 p-6 text-center">
          <p className="mb-1 text-sm font-semibold text-slate-100">No hay historial de ofertas</p>
          <p className="mx-auto max-w-xs text-xs text-slate-400">
            Este producto no ha registrado precios de oferta. Puedes agregarlo a favoritos para
            recibir una alerta cuando haya una promoción.
          </p>
        </div>
      )}

      {mode === "offer" && hasOfferHistory && currentOfferPrice && (
        <div className="mb-4 flex flex-wrap items-center gap-3 rounded-xl border border-cyan-300/20 bg-cyan-300/5 px-4 py-2">
          <span className="rounded-md bg-cyan-300 px-2 py-0.5 text-xs font-bold text-slate-950">
            OFERTA ACTIVA
          </span>
          <span className="price text-sm font-bold text-cyan-200">
            {formatPrice(currentOfferPrice)}
          </span>
          <span className="text-xs text-slate-400">precio de oferta actual</span>
        </div>
      )}

      {activeData.length > 0 && (
        <div className="mb-5 grid grid-cols-1 gap-2 sm:grid-cols-3">
          {[
            {
              label: mode === "offer" ? "Última oferta" : "Precio actual",
              value: formatPrice(currentPrice),
              color: "text-cyan-200",
            },
            {
              label: "Mínimo registrado",
              value: formatPrice(minPrice),
              color: "text-emerald-300",
            },
            {
              label: "Máximo registrado",
              value: formatPrice(maxPrice),
              color: "text-slate-100",
            },
          ].map((stat) => (
            <div
              key={stat.label}
              className="rounded-xl border border-slate-700/80 bg-slate-950/55 p-3"
            >
              <div className="mb-1 text-xs text-slate-400">{stat.label}</div>
              <div className={`price text-sm font-semibold ${stat.color}`}>{stat.value}</div>
            </div>
          ))}
        </div>
      )}

      {activeData.length > 0 && (
        <div className="h-52 rounded-xl border border-slate-800/80 bg-slate-950/35 p-2 sm:h-60 sm:p-3">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={activeData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
              <defs>
                <linearGradient id="priceGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#67e8f9" stopOpacity={0.24} />
                  <stop offset="95%" stopColor="#67e8f9" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#334155" vertical={false} />
              <XAxis
                dataKey="date"
                tickFormatter={formatHistoryDate}
                tick={{ fontSize: 10, fill: "#94a3b8" }}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fontSize: 10, fill: "#94a3b8" }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(v) => `$${(v / 1000).toFixed(0)}k`}
                width={48}
              />
              <Tooltip content={<CustomTooltip />} />
              <Area
                type="monotone"
                dataKey="price"
                stroke="#67e8f9"
                strokeWidth={2}
                fill="url(#priceGrad)"
                dot={activeData.length < 24 ? { r: 3, fill: "#67e8f9", strokeWidth: 0 } : false}
                activeDot={{ r: 4, fill: "#67e8f9", stroke: "#0f172a", strokeWidth: 2 }}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
      {activeData.length === 0 && !(mode === "offer" && !hasOfferHistory) && (
        <p className="rounded-xl border border-slate-700 bg-slate-950/50 px-4 py-6 text-center text-sm text-slate-400">
          No hay datos de precios registrados para este período.
        </p>
      )}
    </section>
  );
}
