import type { Page, Product } from "../../types";
import { getProductById } from "../../services/api/products";
import {
  formatPrice,
  getAvailableStoreCount,
  getMinPrice,
} from "../../services/utils/productUtils";
import { useFetch } from "../../hooks/useFetch";
import { Breadcrumb, Badge, PageMessage } from "../../components/common/ui";

interface Props {
  productIds: string[];
  navigate: (page: Page) => void;
}

export default function ProductComparisonPage({ productIds, navigate }: Props) {
  const {
    data: selected = [],
    loading,
    error,
  } = useFetch(`compare:${productIds.join(",")}`, async () => {
    const results = await Promise.all(productIds.map((id) => getProductById(id)));
    return results.filter((p): p is Product => p !== null);
  });

  const searchAction = {
    label: "Buscar productos",
    onClick: () => navigate({ id: "search-products", query: "" }),
  };

  if (loading) return <PageMessage>Cargando productos…</PageMessage>;

  if (error) {
    return (
      <PageMessage tone="warn" action={searchAction}>
        {error}
      </PageMessage>
    );
  }

  if (selected.length < 2) {
    return (
      <PageMessage action={searchAction}>
        Selecciona al menos 2 productos para comparar.
      </PageMessage>
    );
  }

  const minPrices = selected.map((p) => getMinPrice(p));
  const lowestPrice = Math.min(...minPrices);
  const bestRating = Math.max(...selected.map((p) => p.rating));

  // Filas dinámicas: unión de las especificaciones de los productos seleccionados.
  const allSpecKeys = [...new Set(selected.flatMap((p) => Object.keys(p.specs)))];

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
              {selected.map((p) => (
                <td
                  key={p.id}
                  style={{ borderBottom: "1px solid #1A1A1A", borderRight: "1px solid #1A1A1A" }}
                  className="p-4 text-center"
                >
                  <span
                    className={`text-sm font-bold ${p.rating === bestRating ? "text-warn" : "text-text"}`}
                  >
                    ★ {p.rating.toFixed(1)}
                  </span>
                  <div className="text-xs text-muted">
                    ({p.reviewCount.toLocaleString("es-CL")})
                  </div>
                </td>
              ))}
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
                    {getAvailableStoreCount(p)} disponibles
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
