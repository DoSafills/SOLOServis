import type { AnyCartItem, CartItem, Page, ServiceCartItem } from "../../types";
import { Breadcrumb } from "../../components/common/ui";
import {
  formatPrice,
  getBillingPeriodName,
  getBillingPeriodText,
} from "../../services/utils/productUtils";

interface Props {
  cart: AnyCartItem[];
  navigate: (page: Page) => void;
  onUpdateQuantity: (cartItemId: number, quantity: number) => void;
  onRemove: (cartItemId: number) => void;
}

export default function CartPage({ cart, navigate, onUpdateQuantity, onRemove }: Props) {
  const itemCount = cart.reduce((sum, item) => sum + item.quantity, 0);
  const storeSummaries = Array.from(
    cart
      .filter((item): item is CartItem => item.type === "product")
      .reduce((stores, item) => {
        const storeId = item.offer.storeId;
        const existing = stores.get(storeId);
        if (existing) {
          existing.items.push(item);
        } else {
          stores.set(storeId, { name: item.offer.storeName, items: [item] });
        }
        return stores;
      }, new Map<string, { name: string; items: CartItem[] }>()),
  ).map(([id, store]) => ({
    id,
    ...store,
    total: store.items.reduce((sum, item) => sum + item.offer.price * item.quantity, 0),
  }));
  const serviceSummaries = Array.from(
    cart
      .filter((item): item is ServiceCartItem => item.type === "service")
      .reduce((providers, item) => {
        const provider = item.service.provider || "Servicios";
        const billingPeriod = item.service.billingPeriod ?? "monthly";
        const key = `${provider}:${billingPeriod}`;
        const existing = providers.get(key);
        if (existing) existing.items.push(item);
        else providers.set(key, { name: provider, billingPeriod, items: [item] });
        return providers;
      }, new Map<string, { name: string; billingPeriod: string; items: ServiceCartItem[] }>()),
  ).map(([, provider]) => ({
    ...provider,
    total: provider.items.reduce((sum, item) => sum + item.service.monthlyPrice * item.quantity, 0),
  }));

  const getDiscountPercent = (
    listPrice: number | null | undefined,
    price: number,
  ): number | null => {
    if (listPrice === null || listPrice === undefined || listPrice <= price || listPrice <= 0) {
      return null;
    }
    return Math.round(((listPrice - price) / listPrice) * 100);
  };

  const getSafeOfferUrl = (url: string | undefined): string | null => {
    if (!url) return null;

    try {
      const parsed = new URL(url);
      return parsed.protocol === "http:" || parsed.protocol === "https:" ? url : null;
    } catch {
      return null;
    }
  };

  if (cart.length === 0) {
    return (
      <div className="mx-auto my-6 max-w-4xl rounded-[32px] bg-gradient-to-br from-surface to-bg px-4 py-16 text-center shadow-xl shadow-black/30">
        <Breadcrumb
          items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Cesta" }]}
        />

        <div className="rounded-3xl border border-bd bg-surface p-10 shadow-lg shadow-black/25">
          <div className="mx-auto mb-5 flex h-16 w-16 items-center justify-center rounded-2xl bg-prime-muted text-prime">
            <svg
              width="28"
              height="28"
              viewBox="0 0 24 24"
              fill="none"
              stroke="currentColor"
              strokeWidth="2"
            >
              <circle cx="9" cy="19" r="1" />
              <circle cx="18" cy="19" r="1" />
              <path d="M2 3h3l2.5 10.5a1 1 0 0 0 1 .8H18a1 1 0 0 0 1-.8L21 7H6" />
            </svg>
          </div>
          <h1 className="text-2xl font-bold text-text">Tu cesta está vacía</h1>
          <p className="mx-auto mt-3 max-w-md text-sm text-muted">
            Añade productos o servicios para tener tus opciones guardadas.
          </p>
          <button
            onClick={() => navigate({ id: "home" })}
            className="mt-6 rounded-2xl bg-prime px-5 py-3 text-sm font-bold text-[#181714] shadow-md shadow-black/30 transition hover:bg-prime-dark"
          >
            Explorar productos
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto my-6 max-w-7xl rounded-[32px] border border-bd bg-gradient-to-br from-surface to-bg px-4 py-8 shadow-xl shadow-black/30 md:px-6">
      <Breadcrumb
        items={[{ label: "Inicio", onClick: () => navigate({ id: "home" }) }, { label: "Cesta" }]}
      />

      <div className="mb-8 flex items-end justify-between gap-3 flex-wrap">
        <div>
          <p className="text-sm font-extrabold uppercase tracking-[0.2em] text-prime">Cesta</p>
          <h1 className="mt-2 text-3xl font-extrabold text-text">Tus productos y servicios</h1>
        </div>
        <span className="rounded-full border border-prime/40 bg-prime-muted px-4 py-2 text-sm font-bold text-prime">
          {itemCount} artículos
        </span>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-6">
        <div className="space-y-4">
          {cart.map((item) => {
            if (item.type === "service") {
              const { service, quantity, cartItemId } = item;
              const totalItem = service.monthlyPrice * quantity;
              return (
                <div
                  key={`service-${cartItemId}`}
                  className="rounded-3xl border border-bd bg-gradient-to-br from-surface to-surface-2 p-4 shadow-lg shadow-black/20 md:p-5"
                >
                  <div className="flex flex-col gap-4 md:flex-row">
                    <button
                      onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
                      className="h-28 w-full shrink-0 overflow-hidden rounded-2xl md:w-32"
                    >
                      <img
                        src={service.image}
                        alt={service.name}
                        className="h-full w-full object-cover"
                      />
                    </button>
                    <div className="flex flex-1 flex-col justify-between gap-4 md:flex-row md:items-center">
                      <div>
                        <p className="text-sm font-extrabold uppercase tracking-[0.16em] text-prime">
                          {service.provider}
                        </p>
                        <button
                          onClick={() => navigate({ id: "service-detail", serviceId: service.id })}
                          className="text-left"
                        >
                          <h2 className="mt-1 text-xl font-extrabold text-text">{service.name}</h2>
                        </button>
                        <p className="mt-2 text-sm font-medium text-muted">{service.category}</p>
                        <div className="mt-3 rounded-xl border border-bd bg-surface-2 px-3 py-2">
                          <div className="text-xl font-extrabold text-prime">
                            {formatPrice(service.monthlyPrice)}{" "}
                            {getBillingPeriodText(service.billingPeriod)}
                          </div>
                          {service.installationCost !== null && (
                            <p className="mt-1 text-xs font-medium text-muted">
                              Instalación:{" "}
                              {service.installationCost === 0
                                ? "Gratis"
                                : formatPrice(service.installationCost)}
                            </p>
                          )}
                        </div>
                      </div>
                      <div className="flex items-center justify-between gap-4 md:justify-end">
                        <div className="flex items-center gap-2 rounded-xl border border-bd bg-surface-2 px-2 py-1.5">
                          <button
                            onClick={() => onUpdateQuantity(cartItemId, quantity - 1)}
                            className="h-8 w-8 rounded-lg text-xl font-bold text-prime hover:bg-prime-muted"
                            aria-label={`Disminuir cantidad de ${service.name}`}
                          >
                            −
                          </button>
                          <span className="min-w-7 text-center text-base font-extrabold text-text">
                            {quantity}
                          </span>
                          <button
                            onClick={() => onUpdateQuantity(cartItemId, quantity + 1)}
                            className="h-8 w-8 rounded-lg text-xl font-bold text-prime hover:bg-prime-muted"
                            aria-label={`Aumentar cantidad de ${service.name}`}
                          >
                            +
                          </button>
                        </div>
                        <div className="min-w-[90px] text-right">
                          <div className="text-xl font-extrabold text-text">
                            {formatPrice(totalItem)} {getBillingPeriodText(service.billingPeriod)}
                          </div>
                          <button
                            onClick={() => onRemove(cartItemId)}
                            className="mt-1 text-sm font-semibold text-slate-400 transition-colors hover:text-rose-300"
                          >
                            Eliminar
                          </button>
                        </div>
                      </div>
                    </div>
                  </div>
                </div>
              );
            }

            const { product, offer, quantity, cartItemId } = item;
            const price = offer.price;
            const totalItem = price * quantity;
            const discountPercent = getDiscountPercent(offer.listPrice, price);
            const offerUrl = getSafeOfferUrl(offer.url);

            return (
              <div
                key={`product-${cartItemId}`}
                className="rounded-3xl border border-bd bg-gradient-to-br from-surface to-surface-2 p-4 shadow-lg shadow-black/20 md:p-5"
              >
                <div className="flex flex-col md:flex-row gap-4">
                  <button
                    onClick={() => navigate({ id: "product-detail", productId: product.id })}
                    className="w-full md:w-32 h-28 rounded-2xl overflow-hidden shrink-0"
                  >
                    <img
                      src={product.image}
                      alt={product.name}
                      className="w-full h-full object-cover"
                    />
                  </button>

                  <div className="flex-1 flex flex-col md:flex-row gap-4 md:items-center justify-between">
                    <div>
                      <button
                        onClick={() => navigate({ id: "product-detail", productId: product.id })}
                        className="text-left"
                      >
                        <p className="text-sm font-extrabold uppercase tracking-[0.16em] text-prime">
                          {product.brand}
                        </p>
                        <h2 className="mt-1 text-xl font-extrabold text-text">{product.name}</h2>
                      </button>

                      <div className="mt-2 flex flex-wrap items-center gap-2 text-sm font-medium text-muted">
                        <span>{product.category}</span>
                        <span className="text-prime">•</span>
                        <span>{product.model}</span>
                      </div>

                      <p className="mt-2 text-base font-bold text-prime">
                        Oferta de {offer.storeName}
                      </p>
                      <div className="mt-3 rounded-xl border border-bd bg-surface-2 px-3 py-2">
                        {discountPercent !== null && (
                          <div className="flex items-center gap-2">
                            <span className="text-sm font-medium text-muted line-through">
                              {formatPrice(offer.listPrice!)}
                            </span>
                            <span className="rounded-full border border-prime/40 bg-prime-muted px-2 py-0.5 text-xs font-extrabold text-prime">
                              {discountPercent}% OFF
                            </span>
                          </div>
                        )}
                        <div className="text-xl font-extrabold text-prime">
                          {formatPrice(price)}
                        </div>
                      </div>
                      {offerUrl ? (
                        <a
                          href={offerUrl}
                          target="_blank"
                          rel="noopener noreferrer"
                          className="mt-2 inline-flex text-sm font-bold text-prime hover:text-text hover:underline"
                        >
                          Ver producto en {offer.storeName}
                        </a>
                      ) : (
                        <p className="mt-2 text-sm text-muted">Enlace de tienda no disponible</p>
                      )}
                    </div>

                    <div className="flex items-center justify-between gap-4 md:justify-end">
                      <div className="flex items-center gap-2 rounded-xl border border-bd bg-surface-2 px-2 py-1.5">
                        <button
                          onClick={() => onUpdateQuantity(cartItemId, quantity - 1)}
                          disabled={quantity <= 1}
                          className="h-8 w-8 rounded-lg text-xl font-bold text-prime hover:bg-prime-muted disabled:cursor-not-allowed disabled:opacity-40"
                          aria-label={`Disminuir cantidad de ${product.name}`}
                        >
                          −
                        </button>
                        <span className="min-w-7 text-center text-base font-extrabold text-text">
                          {quantity}
                        </span>
                        <button
                          onClick={() => onUpdateQuantity(cartItemId, quantity + 1)}
                          className="h-8 w-8 rounded-lg text-xl font-bold text-prime hover:bg-prime-muted"
                          aria-label={`Aumentar cantidad de ${product.name}`}
                        >
                          +
                        </button>
                      </div>

                      <div className="text-right min-w-[90px]">
                        <div className="text-xl font-extrabold text-text">
                          {formatPrice(totalItem)}
                        </div>
                        <button
                          onClick={() => onRemove(cartItemId)}
                          className="mt-1 text-sm font-semibold text-slate-400 transition-colors hover:text-rose-300"
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

        <aside className="h-fit rounded-3xl border border-bd bg-surface p-5 shadow-lg shadow-black/20">
          <h2 className="text-2xl font-extrabold text-text">Resumen por tienda</h2>

          <div className="mt-4 space-y-4">
            {storeSummaries.map((store) => (
              <section
                key={store.id}
                className="overflow-hidden rounded-2xl border border-bd bg-gradient-to-br from-surface to-surface-2 shadow-sm shadow-black/20"
              >
                <h3 className="border-b border-bd bg-prime-muted px-4 py-3 text-xl font-extrabold text-prime">
                  {store.name}
                </h3>
                <ul className="space-y-3 p-4">
                  {store.items.map(({ product, offer, quantity }) => {
                    const discountPercent = getDiscountPercent(offer.listPrice, offer.price);
                    return (
                      <li
                        key={product.id}
                        className="border-b border-bd pb-3 last:border-0 last:pb-0"
                      >
                        <p className="text-base font-bold leading-snug text-text">{product.name}</p>
                        <div className="mt-1.5 flex flex-wrap items-center gap-x-2 gap-y-1">
                          <span className="text-sm font-semibold text-muted">
                            Cantidad: {quantity}
                          </span>
                          {discountPercent !== null && (
                            <>
                              <span className="text-sm font-medium text-muted line-through">
                                {formatPrice(offer.listPrice!)}
                              </span>
                              <span className="rounded-full border border-prime/40 bg-prime-muted px-2 py-0.5 text-xs font-extrabold text-prime">
                                {discountPercent}% OFF
                              </span>
                            </>
                          )}
                          <span className="text-base font-extrabold text-prime">
                            {formatPrice(offer.price)}
                          </span>
                        </div>
                      </li>
                    );
                  })}
                </ul>
                <div className="flex items-center justify-between border-t border-bd bg-surface-2 px-4 py-3">
                  <span className="text-base font-bold text-text">Total en {store.name}</span>
                  <span className="text-xl font-black text-prime">{formatPrice(store.total)}</span>
                </div>
              </section>
            ))}
            {serviceSummaries.map((provider) => (
              <section
                key={`${provider.name}:${provider.billingPeriod}`}
                className="overflow-hidden rounded-2xl border border-bd bg-gradient-to-br from-surface to-surface-2 shadow-sm shadow-black/20"
              >
                <h3 className="border-b border-bd bg-prime-muted px-4 py-3 text-xl font-extrabold text-prime">
                  {provider.name} · {getBillingPeriodName(provider.billingPeriod)}
                </h3>
                <ul className="space-y-3 p-4">
                  {provider.items.map(({ service, quantity, serviceOfferId }) => (
                    <li
                      key={serviceOfferId}
                      className="border-b border-bd pb-3 last:border-0 last:pb-0"
                    >
                      <p className="text-base font-bold leading-snug text-text">{service.name}</p>
                      <div className="mt-1.5 flex items-center justify-between gap-2">
                        <span className="text-sm font-semibold text-muted">
                          Cantidad: {quantity}
                        </span>
                        <span className="text-base font-extrabold text-prime">
                          {formatPrice(service.monthlyPrice * quantity)}{" "}
                          {getBillingPeriodText(service.billingPeriod)}
                        </span>
                      </div>
                    </li>
                  ))}
                </ul>
                <div className="flex items-center justify-between border-t border-bd bg-surface-2 px-4 py-3">
                  <span className="text-base font-bold text-text">
                    Total {getBillingPeriodName(provider.billingPeriod)}
                  </span>
                  <span className="text-xl font-black text-prime">
                    {formatPrice(provider.total)} {getBillingPeriodText(provider.billingPeriod)}
                  </span>
                </div>
              </section>
            ))}
          </div>

          <button
            onClick={() => navigate({ id: "home" })}
            className="mt-6 w-full rounded-2xl border border-prime/40 bg-prime-muted py-3 text-base font-bold text-prime transition-all hover:border-prime hover:bg-prime/20 hover:text-text"
          >
            Seguir comparando
          </button>
        </aside>
      </div>
    </div>
  );
}
