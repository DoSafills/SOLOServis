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
import type { PricePoint } from "../types";
import { formatPrice } from "../Services/api/frontend-src/api";

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
      <div
        style={{ background: "#F1F5F9", border: "1px solid #CBD5E1" }}
        className="px-3 py-2 rounded-xl"
      >
        <p className="text-xs text-muted mb-1">{label}</p>
        <p className="price text-sm font-semibold text-prime">{formatPrice(payload[0].value)}</p>
      </div>
    );
  }
  return null;
}

export default function PriceHistory({ history, offerHistory, currentOfferPrice }: Props) {
  const [range, setRange] = useState(90);
  const [mode, setMode] = useState<"price" | "offer">("price");

  const cutoff = new Date();
  cutoff.setDate(cutoff.getDate() - range);

  const activeData = (mode === "price" ? history : offerHistory).filter(
    (p) => new Date(p.date) >= cutoff,
  );

  const hasOfferHistory = offerHistory.length > 0;
  const prices = activeData.map((d) => d.price);
  const minPrice = prices.length ? Math.min(...prices) : 0;
  const maxPrice = prices.length ? Math.max(...prices) : 0;
  const currentPrice = activeData.length ? activeData[activeData.length - 1].price : 0;

  return (
    <div style={{ background: "#F1F5F9", border: "1px solid #CBD5E1" }} className="rounded-2xl p-6">
      {/* Header */}
      <div className="flex items-start justify-between mb-4 flex-wrap gap-3">
        <div>
          <h3 className="text-base font-semibold text-text">Historial de precios</h3>
          <div className="flex gap-1 mt-2">
            <button
              onClick={() => setMode("price")}
              style={
                mode === "price"
                  ? { background: "#0369A1", color: "#FFFFFF" }
                  : { background: "#F1F5F9", border: "1px solid #CBD5E1", color: "#64748B" }
              }
              className="px-3 py-1 rounded-lg text-xs font-semibold transition-all"
            >
              Precio normal
            </button>
            <button
              onClick={() => setMode("offer")}
              style={
                mode === "offer"
                  ? { background: "#0369A1", color: "#FFFFFF" }
                  : hasOfferHistory
                    ? { background: "#F1F5F9", border: "1px solid #CBD5E1", color: "#64748B" }
                    : {
                        background: "#E2E8F0",
                        border: "1px solid #CBD5E1",
                        color: "#94A3B8",
                        cursor: "not-allowed",
                      }
              }
              className="px-3 py-1 rounded-lg text-xs font-semibold transition-all"
            >
              Precio oferta
            </button>
          </div>
        </div>

        <div className="flex gap-1">
          {ranges.map((r) => (
            <button
              key={r.days}
              onClick={() => setRange(r.days)}
              style={
                range === r.days
                  ? { background: "#0369A1", color: "#FFFFFF" }
                  : { background: "#F1F5F9", border: "1px solid #CBD5E1", color: "#64748B" }
              }
              className="px-2.5 py-1 rounded-lg text-xs font-semibold transition-all"
            >
              {r.label}
            </button>
          ))}
        </div>
      </div>

      {/* No offer history notice */}
      {mode === "offer" && !hasOfferHistory && (
        <div
          style={{ background: "#E2E8F0", border: "1px solid #CBD5E1" }}
          className="rounded-xl p-8 text-center"
        >
          <div className="text-3xl mb-3">🏷️</div>
          <p className="text-sm font-semibold text-text mb-1">No ha habido una oferta</p>
          <p className="text-xs text-muted max-w-xs mx-auto">
            Este producto no ha registrado precios de oferta. Puedes agregarlo a favoritos para
            recibir una alerta cuando haya una promoción.
          </p>
        </div>
      )}

      {/* Active offer badge */}
      {mode === "offer" && hasOfferHistory && currentOfferPrice && (
        <div
          style={{ background: "rgba(3,105,161,0.1)", border: "1px solid rgba(3,105,161,0.3)" }}
          className="rounded-xl px-4 py-2 mb-4 flex items-center gap-3 flex-wrap"
        >
          <span
            style={{ background: "#0369A1", color: "#FFFFFF" }}
            className="text-xs font-bold px-2 py-0.5 rounded-md"
          >
            OFERTA ACTIVA
          </span>
          <span className="price text-sm font-bold text-prime">
            {formatPrice(currentOfferPrice)}
          </span>
          <span className="text-xs text-muted">precio de oferta actual</span>
        </div>
      )}

      {/* Stats */}
      {activeData.length > 0 && (
        <div className="grid grid-cols-3 gap-3 mb-5">
          {[
            {
              label: mode === "offer" ? "Última oferta" : "Precio actual",
              value: formatPrice(currentPrice),
              color: "text-prime",
            },
            { label: "Mínimo registrado", value: formatPrice(minPrice), color: "text-success" },
            { label: "Máximo registrado", value: formatPrice(maxPrice), color: "text-prime" },
          ].map((stat) => (
            <div key={stat.label} style={{ background: "#E2E8F0", border: "1px solid #CBD5E1" }} className="rounded-xl p-3">
              <div className="text-xs text-muted mb-1">{stat.label}</div>
              <div className={`price text-sm font-semibold ${stat.color}`}>{stat.value}</div>
            </div>
          ))}
        </div>
      )}

      {/* Chart */}
      {activeData.length > 0 && (
        <div className="h-40">
          <ResponsiveContainer width="100%" height="100%">
            <AreaChart data={activeData} margin={{ top: 4, right: 4, bottom: 0, left: 0 }}>
              <defs>
                <linearGradient id="priceGrad" x1="0" y1="0" x2="0" y2="1">
                  <stop offset="5%" stopColor="#0369A1" stopOpacity={0.25} />
                  <stop offset="95%" stopColor="#0369A1" stopOpacity={0} />
                </linearGradient>
              </defs>
              <CartesianGrid strokeDasharray="3 3" stroke="#CBD5E1" vertical={false} />
              <XAxis
                dataKey="date"
                tick={{ fontSize: 10, fill: "#64748B" }}
                tickLine={false}
                axisLine={false}
                interval="preserveStartEnd"
              />
              <YAxis
                tick={{ fontSize: 10, fill: "#64748B" }}
                tickLine={false}
                axisLine={false}
                tickFormatter={(v) => `$${(v / 1000).toFixed(0)}k`}
                width={48}
              />
              <Tooltip content={<CustomTooltip />} />
              <Area
                type="monotone"
                dataKey="price"
                stroke="#0369A1"
                strokeWidth={2}
                fill="url(#priceGrad)"
                dot={false}
                activeDot={{ r: 4, fill: "#0369A1", stroke: "#F1F5F9", strokeWidth: 2 }}
              />
            </AreaChart>
          </ResponsiveContainer>
        </div>
      )}
    </div>
  );
}
