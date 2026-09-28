import { useEffect, useState } from "react";
import type { Page, Product } from "../../types";
import { formatPrice } from "../../data/mockData";
import { getProduct, toProduct } from "../../Services/api/products-client";
import { Breadcrumb, Badge } from "../../components/ui";

interface Props {
  productIds: string[];
  navigate: (page: Page) => void;
}

type ComparisonState =
  | { productIdKey: string; status: "loading" }
  | { productIdKey: string; status: "success"; products: Product[] }
  | { productIdKey: string; status: "error" };

const specRows = [
  "Modelo",
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
  const productIdKey = productIds.join(",");
  const [comparison, setComparison] = useState<ComparisonState>({
    productIdKey,
    status: "loading",
  });

  useEffect(() => {
    let cancelled = false;
    const ids = [...new Set(productIdKey.split(",").filter(Boolean))];

    Promise.all(ids.map((id) => getProduct(id)))
      .then((items) => {
        if (!cancelled) {
          setComparison({
            productIdKey,
            status: "success",
            products: items.map(toProduct),
          });
        }
      })
      .catch(() => {
        if (!cancelled) setComparison({ productIdKey, status: "error" });
      });

    return () => {
      cancelled = true;
    };
  }, [productIdKey]);

  if (comparison.productIdKey !== productIdKey || comparison.status === "loading") {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center" role="status">
        <p className="text-muted">Cargando productos para comparar...</p>
      </div>
    );
  }

  if (comparison.status === "error") {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center" role="alert">
        <p className="text-muted mb-4">
          No fue posible obtener los productos. Comprueba la conexión con la API e inténtalo de nuevo.
        </p>
        <button
          onClick={() => navigate({ id: "search-products", query: "" })}
          style={{ background: "#E8001B", color: "#0A0A0A" }}
          className="px-5 py-2 rounded-xl text-sm font-semibold"
        >
          Volver a productos
        </button>
      </div>
    );
  }

  const selected = comparison.products;

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
    const availablePrices = product.offers
      .filter((offer) => offer.available)
      .map((offer) => offer.price);
    return availablePrices.length ? Math.min(...availablePrices) : null;
  });
  const availableMinPrices = minPrices.filter((price): price is number => price !== null);
  const lowestPrice = availableMinPrices.length ? Math.min(...availableMinPrices) : null;

  const allSpecKeys = specRows.filter((key) => selected.some((p) => p.specs[key] !== undefined));

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Productos",
            onClick: () => navigate({ id: "search-products", query: "" }),
          },
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
                style={{
                  borderBottom: "1px solid #1A1A1A",
                  borderRight: "1px solid #2A2A2A",
                }}
                className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
              >
                Precio mínimo
              </td>
              {selected.map((p, i) => (
                <td
                  key={p.id}
                  style={{
                    borderBottom: "1px solid #1A1A1A",
                    borderRight: "1px solid #1A1A1A",
                  }}
                  className="p-4 text-center"
                >
                  <div
                    className={`price text-lg font-bold ${
                      minPrices[i] !== null && minPrices[i] === lowestPrice
                        ? "text-prime"
                        : "text-text"
                    }`}
                  >
                    {minPrices[i] === null ? "Sin ofertas" : formatPrice(minPrices[i])}
                  </div>
                  {minPrices[i] !== null && minPrices[i] === lowestPrice && (
                    <Badge variant="best">Mejor precio</Badge>
                  )}
                </td>
              ))}
            </tr>

            {/* Rating row */}
            <tr>
              <td
                style={{
                  borderBottom: "1px solid #1A1A1A",
                  borderRight: "1px solid #2A2A2A",
                }}
                className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
              >
                Valoración
              </td>
              {selected.map((p) => {
                const best = Math.max(...selected.map((s) => s.rating));
                return (
                  <td
                    key={p.id}
                    style={{
                      borderBottom: "1px solid #1A1A1A",
                      borderRight: "1px solid #1A1A1A",
                    }}
                    className="p-4 text-center"
                  >
                    <span
                      className={`text-sm font-bold ${
                        p.rating === best ? "text-warn" : "text-text"
                      }`}
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
                  style={{
                    borderBottom: "1px solid #1A1A1A",
                    borderRight: "1px solid #2A2A2A",
                  }}
                  className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide"
                >
                  {key}
                </td>
                {selected.map((p) => (
                  <td
                    key={p.id}
                    style={{
                      borderBottom: "1px solid #1A1A1A",
                      borderRight: "1px solid #1A1A1A",
                    }}
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
