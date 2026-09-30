import type { CartItem, Page } from "../../types";
import { Breadcrumb } from "../../components/common/ui";
import { formatPrice, getMinPrice } from "../../Services/utils/productUtils";

interface Props {
  cart: CartItem[];
  navigate: (page: Page) => void;
  onUpdateQuantity: (productId: string, quantity: number) => void;
  onRemove: (productId: string) => void;
}

export default function CartPage({ cart, navigate, onUpdateQuantity, onRemove }: Props) {
  const itemCount = cart.reduce((sum, item) => sum + item.quantity, 0);
  const subtotal = cart.reduce((sum, item) => sum + getMinPrice(item.product) * item.quantity, 0);
  const shipping = cart.length > 0 ? 0 : 0;
  const total = subtotal + shipping;

  if (cart.length === 0) {
    return (
      <div className="max-w-4xl mx-auto px-4 py-16 text-center">
        <Breadcrumb
          items={[
            { label: "Inicio", onClick: () => navigate({ id: "home" }) },
            { label: "Cesta" },
          ]}
        />

        <div
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="rounded-3xl p-10"
        >
          <div className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-prime-muted text-prime">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
              <circle cx="9" cy="19" r="1" />
              <circle cx="18" cy="19" r="1" />
              <path d="M2 3h3l2.5 10.5a1 1 0 0 0 1 .8H18a1 1 0 0 0 1-.8L21 7H6" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-text">Tu cesta está vacía</h1>
          <p className="mt-3 text-sm text-muted max-w-md mx-auto">
            Añade productos para comparar precios y guardar tus mejores opciones.
          </p>
          <button
            onClick={() => navigate({ id: "home" })}
            style={{ background: "#E8001B", color: "#0A0A0A" }}
            className="mt-6 px-5 py-3 rounded-2xl text-sm font-semibold hover:opacity-90 transition-opacity"
          >
            Explorar productos
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          { label: "Cesta" },
        ]}
      />

      <div className="mb-8 flex items-end justify-between gap-3 flex-wrap">
        <div>
          <p className="text-xs font-semibold uppercase tracking-[0.2em] text-prime">Cesta</p>
          <h1 className="text-3xl font-bold text-text mt-2">Tus productos</h1>
        </div>
        <span className="text-sm text-muted-2">{itemCount} artículos</span>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-6">
        <div className="space-y-4">
          {cart.map(({ product, quantity }) => {
            const price = getMinPrice(product) || product.offers[0]?.price || 0;
            const totalItem = price * quantity;

            return (
              <div
                key={product.id}
                style={{ background: "#111111", border: "1px solid #2A2A2A" }}
                className="rounded-3xl p-4 md:p-5"
              >
                <div className="flex flex-col md:flex-row gap-4">
                  <button
                    onClick={() => navigate({ id: "product-detail", productId: product.id })}
                    className="w-full md:w-32 h-28 rounded-2xl overflow-hidden shrink-0"
                  >
                    <img src={product.image} alt={product.name} className="w-full h-full object-cover" />
                  </button>

                  <div className="flex-1 flex flex-col md:flex-row gap-4 md:items-center justify-between">
                    <div>
                      <button
                        onClick={() => navigate({ id: "product-detail", productId: product.id })}
                        className="text-left"
                      >
                        <p className="text-xs font-semibold uppercase tracking-[0.16em] text-prime">
                          {product.brand}
                        </p>
                        <h2 className="mt-1 text-lg font-semibold text-text">{product.name}</h2>
                      </button>

                      <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-muted">
                        <span>{product.category}</span>
                        <span className="text-muted-2">•</span>
                        <span>{product.model}</span>
                      </div>

                      <div className="mt-3 text-sm font-semibold text-text">{formatPrice(price)}</div>
                    </div>

                    <div className="flex items-center justify-between gap-4 md:justify-end">
                      <div className="flex items-center gap-2 rounded-xl border border-surface-3 bg-surface-2 px-2 py-1.5">
                        <button
                          onClick={() => onUpdateQuantity(product.id, quantity - 1)}
                          className="h-7 w-7 rounded-lg text-lg text-text hover:bg-surface-3"
                          aria-label={`Disminuir cantidad de ${product.name}`}
                        >
                          −
                        </button>
                        <span className="min-w-6 text-center text-sm font-semibold text-text">
                          {quantity}
                        </span>
                        <button
                          onClick={() => onUpdateQuantity(product.id, quantity + 1)}
                          className="h-7 w-7 rounded-lg text-lg text-text hover:bg-surface-3"
                          aria-label={`Aumentar cantidad de ${product.name}`}
                        >
                          +
                        </button>
                      </div>

                      <div className="text-right min-w-[90px]">
                        <div className="text-lg font-bold text-text">{formatPrice(totalItem)}</div>
                        <button
                          onClick={() => onRemove(product.id)}
                          className="mt-1 text-xs text-muted hover:text-prime transition-colors"
                        >
                          Eliminar
                        </button>
                      </div>
                    </div>
                  </div>
                </div>
              </div>
            );
          })}
        </div>

        <aside
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="rounded-3xl p-5 h-fit"
        >
          <h2 className="text-lg font-semibold text-text">Resumen</h2>

          <div className="mt-5 space-y-3 text-sm">
            <div className="flex items-center justify-between text-muted">
              <span>Subtotal</span>
              <span>{formatPrice(subtotal)}</span>
            </div>
            <div className="flex items-center justify-between text-muted">
              <span>Envío</span>
              <span>{shipping === 0 ? "Gratis" : formatPrice(shipping)}</span>
            </div>
            <div style={{ borderTop: "1px solid #2A2A2A" }} className="pt-3 flex items-center justify-between text-base font-bold text-text">
              <span>Total</span>
              <span>{formatPrice(total)}</span>
            </div>
          </div>

          <button
            onClick={() => navigate({ id: "home" })}
            className="mt-6 w-full rounded-2xl border border-surface-3 bg-transparent py-3 text-sm font-semibold text-text hover:border-prime hover:text-prime transition-all"
          >
            Seguir comparando
          </button>
        </aside>
      </div>
    </div>
  );
}
