import { useEffect, useState } from "react";
import type { Page, Product } from "../../types";
import {
  getProductComparison,
  type ProductComparisonRecommendation,
} from "../../services/api/products";
import { formatPrice } from "../../services/utils/productUtils";
import { Breadcrumb, Badge } from "../../components/common/ui";

interface Props {
  productIds: string[];
  navigate: (page: Page) => void;
}

export default function ProductComparisonPage({ productIds, navigate }: Props) {
  const [selected, setSelected] = useState<Product[]>([]);
  const [recommendation, setRecommendation] = useState<ProductComparisonRecommendation | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadProducts() {
      if (productIds.length === 0) {
        setSelected([]);
        setRecommendation(null);
        setLoading(false);
        return;
      }

      setLoading(true);
      setError(null);
      try {
        const comparison = await getProductComparison(productIds);
        if (cancelled) return;
        setSelected(comparison.products);
        setRecommendation(comparison.recommendation);
      } catch (err) {
        if (cancelled) return;
        setError(err instanceof Error ? err.message : "Error al cargar los productos");
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    loadProducts();

    return () => {
      cancelled = true;
    };
  }, [productIds]);

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando productos...</p>
      </div>
    );
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn mb-4">{error}</p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "#E8001B", color: "#0A0A0A" }}
          className="px-5 py-2 rounded-xl text-sm font-semibold"
        >
          Buscar productos
        </button>
      </div>
    );
  }

  if (selected.length < 2) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted mb-4">Selecciona al menos 2 productos para comparar.</p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "#E8001B", color: "#0A0A0A" }}
          className="px-5 py-2 rounded-xl text-sm font-semibold"
        >
          Buscar productos
        </button>
      </div>
    );
  }

  const minPrices = selected.map((product) => {
    const prices = product.offers
      .filter((offer) => offer.available && offer.price > 0)
      .map((offer) => offer.price);
    return prices.length > 0 ? Math.min(...prices) : null;
  });
  const availableStoreCounts = selected.map(
    (product) => product.offers.filter((offer) => offer.available).length,
  );
  const allSpecKeys = Array.from(new Set(selected.flatMap((product) => Object.keys(product.specs)))).sort();
  const differs = (values: (string | number | undefined)[]) =>
    new Set(values.map((value) => String(value ?? "").trim().toLocaleLowerCase())).size > 1;
  const winnerIds = new Set(recommendation?.winnerIds ?? []);
  const scoresByProductId = new Map(
    (recommendation?.scores ?? []).map((score) => [score.productId, score]),
  );
  const bestIndexes = selected.flatMap((product, index) => winnerIds.has(product.id) ? [index] : []);
  const hasRecommendation = bestIndexes.length > 0;
  const lowestPrice = minPrices.filter((price): price is number => price !== null).reduce(
    (lowest, price) => Math.min(lowest, price),
    Number.POSITIVE_INFINITY,
  );

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Productos", onClick: () => navigate({ id: "search-products", query: "" }) },
          { label: "Comparación" },
        ]}
      />

      <h1 className="text-2xl font-bold text-text mb-2">Comparación de productos</h1>
      <p className="text-sm text-muted mb-8">Comparando {selected.length} productos</p>

      <section
        style={{ background: "#111111", border: "1px solid #2A2A2A" }}
        className="mb-6 rounded-xl p-4"
        aria-live="polite"
      >
        <h2 className="text-sm font-semibold text-text">Recomendación</h2>
        {hasRecommendation ? (
          <div className="mt-1 text-sm text-muted">
            {bestIndexes.length === 1 ? (
              <>
                <span className="font-semibold text-prime">{selected[bestIndexes[0]].name}</span>
                {" "}es la mejor opción según {scoresByProductId.get(selected[bestIndexes[0]].id)?.reasons.join(", ")} ({recommendation?.score} de {recommendation?.totalCriteria} criterios).
              </>
            ) : (
              <>
                Empate entre {bestIndexes.map((index) => selected[index].name).join(" y ")} con {recommendation?.score} de {recommendation?.totalCriteria} criterios.
              </>
            )}
          </div>
        ) : (
          <p className="mt-1 text-sm text-muted">La API no entrega datos suficientes para elegir una opción.</p>
        )}
        <p className="mt-2 text-xs text-muted">Criterios: menor precio disponible, valoración y tiendas con stock.</p>
      </section>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[560px]">
          {/* Product headers */}
          <thead>
            <tr>
              <th
                style={{
                  background: "#111111",
                  borderBottom: "1px solid #2A2A2A",
                  borderRight: "1px solid #2A2A2A",
                }}
                className="text-left text-xs font-semibold text-muted-2 uppercase tracking-widest p-4 w-40"
              >
                Característica
              </th>
              {selected.map((p) => (
                <th
                  key={p.id}
                  style={{
                    background: "#111111",
                    borderBottom: "1px solid #2A2A2A",
                    borderRight: "1px solid #1A1A1A",
                  }}
                  className="p-4 text-center"
                >
                  <div className="flex flex-col items-center gap-2">
                    <img src={p.image} alt={p.name} className="w-20 h-14 object-cover rounded-xl" />
                    <div>
                      <div className="text-xs text-prime font-semibold">{p.brand}</div>
                      <div className="text-sm font-semibold text-text leading-tight">{p.name}</div>
                    </div>
                    {hasRecommendation && bestIndexes.includes(selected.indexOf(p)) && (
                      <Badge variant="best">Mejor opción · {scoresByProductId.get(p.id)?.score ?? 0} pts</Badge>
                    )}
                    <button
                      onClick={() => navigate({ id: "product-detail", productId: p.id })}
                      style={{
                        background: "#1A1A1A",
                        border: "1px solid #2A2A2A",
                        color: "#94A3B8",
                      }}
                      className="text-xs px-3 py-1 rounded-lg hover:border-prime hover:text-prime transition-all"
                    >
                      Ver detalle
                    </button>
                  </div>
                </th>
              ))}
            </tr>
          </thead>

          <tbody>
            {/* Price row */}
            <tr style={{ background: "rgba(34,211,160,0.04)" }}>
              <td
                style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #2A2A2A" }}
                className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
              >
                Precio mínimo
              </td>
              {selected.map((p, i) => (
                <td
                  key={p.id}
                  style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #1A1A1A" }}
                  className="p-4 text-center"
                >
                  <div
                    className={`price text-lg font-bold ${minPrices[i] !== null && minPrices[i] === lowestPrice ? "text-prime" : "text-text"}`}
                  >
                    {minPrices[i] === null ? "Sin oferta" : formatPrice(minPrices[i])}
                  </div>
                  {minPrices[i] !== null && minPrices[i] === lowestPrice && <Badge variant="best">Mejor precio</Badge>}
                </td>
              ))}
            </tr>

            {/* Rating row */}
            <tr>
              <td
                style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #2A2A2A" }}
                className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
              >
                Valoración
              </td>
              {selected.map((p) => {
                const bestRating = Math.max(...selected.map((product) => product.rating));
                return (
                  <td
                    key={p.id}
                    style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #1A1A1A" }}
                    className="p-4 text-center"
                  >
                    <span
                      className={`text-sm font-bold ${p.rating > 0 && p.rating === bestRating ? "text-warn" : "text-text"}`}
                    >
                      ★ {p.rating.toFixed(1)}
                    </span>
                    <div className="text-xs text-muted">
                      ({p.reviewCount.toLocaleString("es-CL")})
                    </div>
                  </td>
                );
              })}
            </tr>

            {/* Spec rows */}
            {allSpecKeys.map((key, ri) => {
              const values = selected.map((product) => product.specs[key]);
              const hasDifference = differs(values);
              return (
              <tr key={key} style={{ background: ri % 2 === 0 ? "#0A0A0A" : "transparent" }}>
                <td
                  style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #2A2A2A" }}
                  className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
                >
                  {key}
                </td>
                {selected.map((p) => (
                  <td
                    key={p.id}
                    style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #1A1A1A" }}
                    className={`p-4 text-center ${hasDifference ? "bg-warn/5" : ""}`}
                  >
                    <span className={`text-sm ${hasDifference ? "font-semibold text-warn" : "text-text"}`}>
                      {p.specs[key] ?? <span className="text-muted">Sin dato</span>}
                    </span>
                  </td>
                ))}
              </tr>
              );
            })}

            {/* Availability */}
            <tr>
              <td
                style={{ borderRight: "1px solid #2A2A2A" }}
                className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
              >
                Tiendas
              </td>
              {selected.map((p) => (
                <td
                  key={p.id}
                  style={{ borderRight: "1px solid #1A1A1A" }}
                  className="p-4 text-center"
                >
                  <span className={`text-sm font-semibold ${availableStoreCounts[selected.indexOf(p)] > 0 ? "text-prime" : "text-muted"}`}>
                    {availableStoreCounts[selected.indexOf(p)]} disponibles
                  </span>
                </td>
              ))}
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  );
}



