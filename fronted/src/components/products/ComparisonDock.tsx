import { useEffect, useState } from "react";
import type { Page, Product } from "../../types";
import { getProductById } from "../../services/api/api";
import { Skeleton } from "../common/ui";

interface Props {
  productIds: string[];
  onClear: () => void;
  onRemove: (product: Product) => void;
  onNavigate: (page: Page) => void;
}

export default function ComparisonDock({ productIds, onClear, onRemove, onNavigate }: Props) {
  const [products, setProducts] = useState<Product[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [expanded, setExpanded] = useState(true);
  const productKey = productIds.join(",");

  useEffect(() => {
    let cancelled = false;

    async function loadProducts() {
      setLoading(true);
      setError(null);

      try {
        const ids = productKey ? productKey.split(",") : [];
        const results = await Promise.all(ids.map((id) => getProductById(id)));
        if (cancelled) return;

        const availableProducts = results.filter((product): product is Product => product !== null);
        setProducts(availableProducts);
        if (availableProducts.length !== ids.length) {
          setError("No se pudo encontrar uno o más productos seleccionados.");
        }
      } catch (loadError) {
        if (cancelled) return;
        setError(
          loadError instanceof Error
            ? loadError.message
            : "No se pudieron cargar los productos seleccionados.",
        );
      } finally {
        if (!cancelled) setLoading(false);
      }
    }

    void loadProducts();

    return () => {
      cancelled = true;
    };
  }, [productKey]);

  return (
    <section
      aria-label="Productos seleccionados para comparar"
      className="w-[min(22rem,calc(100vw-2rem))] overflow-hidden rounded-2xl border border-bd bg-surface text-text shadow-[0_20px_60px_rgba(0,0,0,0.35)] backdrop-blur-xl"
    >
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((open) => !open)}
        className="flex w-full items-center justify-between gap-3 bg-prime px-4 py-3 text-left text-[#181714] transition-colors hover:bg-prime-dark"
      >
        <span className="flex min-w-0 items-center gap-3">
          <span className="flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-black/10 text-lg">
            ⚖
          </span>
          <span className="min-w-0">
            <span className="block text-[10px] font-bold uppercase tracking-[0.16em] text-[#181714]/75">
              {expanded ? "¿Qué estás comparando?" : "Comparador"}
            </span>
            <span className="block truncate text-sm font-extrabold">
              {productIds.length} producto{productIds.length === 1 ? "" : "s"} seleccionado
              {productIds.length === 1 ? "" : "s"}
            </span>
          </span>
        </span>
        <span aria-hidden="true" className="text-lg font-bold">
          {expanded ? "−" : "+"}
        </span>
      </button>

      {expanded && (
        <div className="space-y-3 p-3">
          {loading ? (
            <div role="status" aria-label="Cargando productos" className="space-y-2">
              {[0, 1].map((item) => (
                <div
                  key={item}
                  className="flex items-center gap-2 rounded-xl border border-bd bg-surface-2 p-2"
                >
                  <Skeleton className="h-10 w-10 shrink-0 rounded-lg" />
                  <div className="min-w-0 flex-1 space-y-2">
                    <Skeleton className="h-3 w-3/4" />
                    <Skeleton className="h-2 w-1/2" />
                  </div>
                </div>
              ))}
            </div>
          ) : (
            <ul className="space-y-2">
              {products.map((product) => (
                <li
                  key={product.id}
                  className="flex items-center gap-2 rounded-xl border border-bd bg-surface-2 p-2"
                >
                  <button
                    type="button"
                    onClick={() => onNavigate({ id: "product-detail", productId: product.id })}
                    className="flex min-w-0 flex-1 items-center gap-2 text-left"
                    title={`Ver ${product.name}`}
                  >
                    {product.image ? (
                      <img
                        src={product.image}
                        alt=""
                        className="h-10 w-10 shrink-0 rounded-lg bg-surface-3 object-contain"
                      />
                    ) : (
                      <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-lg bg-prime-muted text-xs text-prime">
                        {product.brand.slice(0, 2).toUpperCase() || "PR"}
                      </span>
                    )}
                    <span className="min-w-0">
                      <span className="block truncate text-xs font-bold text-text">
                        {product.name}
                      </span>
                      <span className="block truncate text-[10px] text-muted">
                        {product.brand} · {product.category}
                      </span>
                    </span>
                  </button>
                  <button
                    type="button"
                    onClick={() => onRemove(product)}
                    aria-label={`Quitar ${product.name} de la comparación`}
                    title="Quitar de la comparación"
                    className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg text-muted transition hover:bg-danger/10 hover:text-danger"
                  >
                    ×
                  </button>
                </li>
              ))}
            </ul>
          )}

          {error && (
            <p role="alert" className="rounded-lg bg-danger/10 px-2 py-1.5 text-[11px] text-danger">
              {error}
            </p>
          )}

          <div className="flex gap-2">
            {productIds.length >= 2 && (
              <button
                type="button"
                onClick={() => onNavigate({ id: "product-comparison", productIds })}
                className="flex-1 rounded-xl bg-prime px-3 py-2 text-xs font-extrabold text-[#181714] shadow-lg shadow-black/20 transition hover:bg-prime-dark"
              >
                Comparar productos
              </button>
            )}
            <button
              type="button"
              onClick={onClear}
              className="rounded-xl border border-bd px-3 py-2 text-xs font-semibold text-muted transition hover:border-danger/50 hover:bg-danger/10 hover:text-danger"
            >
              Limpiar
            </button>
          </div>
        </div>
      )}
    </section>
  );
}
