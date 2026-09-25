import type { CartItem, Page } from "../../types";
import { Breadcrumb } from "../../components/ui";

interface Props {
  navigate: (page: Page) => void;
  cart: CartItem[];
  onRemoveFromCart: (id: string) => void;
  onUpdateCartItem: (id: string, quantity: number) => void;
}

export default function CartPage({ navigate, cart, onRemoveFromCart, onUpdateCartItem }: Props) {
  const total = cart.reduce((sum, item) => sum + item.price * item.quantity, 0);

  if (cart.length === 0) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-8">
        <Breadcrumb
          items={[
            { label: "Inicio", onClick: () => navigate({ id: "home" }) },
            { label: "Cesta" },
          ]}
        />

        <div className="flex flex-col items-center justify-center py-24 gap-4 text-center">
          <div className="w-16 h-16 rounded-2xl bg-surface-2 flex items-center justify-center mb-2">
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="#64748B"
              strokeWidth="1.5"
            >
              <circle cx="9" cy="19" r="1.5" />
              <circle cx="18" cy="19" r="1.5" />
              <path d="M3 4h2l2.4 10.2a1 1 0 0 0 1 .8h9.9a1 1 0 0 0 1-.8L20 7H7" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-text">Tu cesta está vacía</h3>
          <p className="text-sm text-muted max-w-xs">
            Añade productos para compararlos, guardarlos y revisarlos más tarde.
          </p>
          <button
            onClick={() => navigate({ id: "home" })}
            style={{ background: "#E8001B", color: "#0A0A0A" }}
            className="mt-2 px-5 py-2 rounded-xl text-sm font-semibold"
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

      <div className="mb-6 flex items-end justify-between gap-3 flex-wrap">
        <div>
          <p className="text-xs uppercase tracking-[0.2em] text-prime font-semibold">Tu compra</p>
          <h1 className="text-2xl font-bold text-text mt-2">Mi cesta</h1>
        </div>
        <button
          onClick={() => navigate({ id: "home" })}
          className="text-sm text-muted-2 hover:text-prime transition-colors"
        >
          Seguir comprando
        </button>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[1.6fr_0.8fr] gap-6">
        <section className="space-y-4">
          {cart.map((item) => (
            <div
              key={item.id}
              style={{ background: "#111111", border: "1px solid #2A2A2A" }}
              className="rounded-2xl p-4 flex items-center gap-4 shadow-[0_12px_30px_rgba(15,23,42,0.08)] transition-all duration-200 hover:border-prime/40 hover:-translate-y-0.5"
            >
              <img src={item.image} alt={item.name} className="w-20 h-20 object-cover rounded-xl" />

              <div className="flex-1 min-w-0">
                <div className="text-[11px] uppercase tracking-[0.18em] text-prime font-semibold">
                  {item.brand}
                </div>
                <div className="text-sm font-semibold text-text mt-1 line-clamp-2">{item.name}</div>
                <div className="text-base font-bold text-prime mt-2">
                  ${item.price.toLocaleString("es-CL")}
                </div>
              </div>

              <div className="flex items-center gap-2">
                <button
                  onClick={() => onUpdateCartItem(item.id, item.quantity - 1)}
                  className="w-8 h-8 rounded-lg bg-surface-2 text-text font-bold hover:bg-surface-3 transition-colors"
                >
                  −
                </button>
                <span className="w-8 text-center text-sm font-semibold text-text">{item.quantity}</span>
                <button
                  onClick={() => onUpdateCartItem(item.id, item.quantity + 1)}
                  className="w-8 h-8 rounded-lg bg-surface-2 text-text font-bold hover:bg-surface-3 transition-colors"
                >
                  +
                </button>
              </div>

              <button
                onClick={() => onRemoveFromCart(item.id)}
                className="text-xs text-muted hover:text-danger transition-colors"
              >
                Eliminar
              </button>
            </div>
          ))}
        </section>

        <aside
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="rounded-2xl p-5 h-fit shadow-[0_16px_40px_rgba(15,23,42,0.12)]"
        >
          <h2 className="text-base font-semibold text-text mb-4">Resumen</h2>
          <div className="space-y-3 text-sm text-muted">
            <div className="flex justify-between">
              <span>Productos</span>
              <span>{cart.reduce((sum, item) => sum + item.quantity, 0)}</span>
            </div>
            <div className="flex justify-between">
              <span>Subtotal</span>
              <span>${total.toLocaleString("es-CL")}</span>
            </div>
          </div>

          <div className="mt-5 rounded-xl bg-surface-2 p-3 border border-border/80">
            <div className="flex items-center justify-between text-sm text-muted">
              <span>Total</span>
              <span className="text-xs uppercase tracking-[0.18em] text-prime font-semibold">CLP</span>
            </div>
            <div className="mt-2 text-2xl font-black text-prime">${total.toLocaleString("es-CL")}</div>
          </div>
        </aside>
      </div>
    </div>
  );
}
