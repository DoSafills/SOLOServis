import { useEffect, useState } from "react";
import type { Page, Product } from "../types";
import { getProductById, formatPrice, getMinPrice } from "../Services/api/frontend-src/api";
import { Breadcrumb, Badge } from "../components/ui";

interface Props {
  productIds: string[];
  navigate: (page: Page) => void;
}

const specRows = [
  "VRAM",
  "Arquitectura",
  "Núcleos CUDA",
  "Stream Processors",
  "Bus de memoria",
  "TDP",
  "Garantía",
  "Conectores",
  "Procesador",
  "RAM",
  "Almacenamiento",
  "Pantalla",
  "Sistema operativo",
];

export default function ProductComparisonPage({ productIds, navigate }: Props) {
  const [selected, setSelected] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let cancelled = false;

    async function loadProducts() {
      if (productIds.length === 0) {
        setSelected([]);
        setLoading(false);
        return;
      }

      setLoading(true);
      setError(null);
      try {
        const results = await Promise.all(productIds.map((id) => getProductById(id)));
        if (cancelled) return;
        const valid = results.filter((p: Product | null): p is Product => p !== null);
        setSelected(valid);
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
        <p className="text-muted">Cargando productos…</p>
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

  const minPrices = selected.map((p) => getMinPrice(p));
  const lowestPrice = Math.min(...minPrices);

  const allSpecKeys = specRows.filter((key) => selected.some((p) => p.specs[key] !== undefined));

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
                    className={`price text-lg font-bold ${minPrices[i] === lowestPrice ? "text-prime" : "text-text"}`}
                  >
                    {formatPrice(minPrices[i])}
                  </div>
                  {minPrices[i] === lowestPrice && <Badge variant="best">Mejor precio</Badge>}
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
                const best = Math.max(...selected.map((s) => s.rating));
                return (
                  <td
                    key={p.id}
                    style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #1A1A1A" }}
                    className="p-4 text-center"
                  >
                    <span
                      className={`text-sm font-bold ${p.rating === best ? "text-warn" : "text-text"}`}
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
            {allSpecKeys.map((key, ri) => (
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
                    className="p-4 text-center"
                  >
                    <span className="text-sm text-text">
                      {p.specs[key] ?? <span className="text-muted">—</span>}
                    </span>
                  </td>
                ))}
              </tr>
            ))}

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
                  <span className="text-sm font-semibold text-prime">
                    {p.offers.filter((o) => o.available).length} disponibles
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
