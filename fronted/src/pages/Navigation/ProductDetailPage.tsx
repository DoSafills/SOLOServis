import { useEffect, useState } from "react";
import type { Page, PricePoint, StoreOffer } from "../../types";
import { getProductById, type ApiProductDetail } from "../../Services/api/products";
import { getPriceHistory } from "../../Services/api/priceHistory";
import { formatPrice } from "../../Services/api/frontend-src/api";
import { mapOffer } from "../../Services/api/catalog";
import { Badge, Breadcrumb, FavoriteButton, Rating } from "../../components/ui";
import PriceHistory from "../../components/PriceHistory";

interface Props {
  productId: string;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

export default function ProductDetailPage({
  productId,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const [product, setProduct] = useState<ApiProductDetail | null>(null);
  const [notFound, setNotFound] = useState(false);
  const [history, setHistory] = useState<PricePoint[]>([]);
  const [offerHistory, setOfferHistory] = useState<PricePoint[]>([]);

  useEffect(() => {
    setProduct(null);
    setNotFound(false);
    setHistory([]);
    setOfferHistory([]);
    getProductById(productId)
      .then(setProduct)
      .catch(() => setNotFound(true));
  }, [productId]);

  const offers: StoreOffer[] = (product?.offers ?? []).map(mapOffer);
  const sortedOffers = [...offers].sort((a, b) => {
    if (a.available && !b.available) return -1;
    if (!a.available && b.available) return 1;
    return a.price - b.price;
  });
  const cheapestAvailable = sortedOffers.find((o) => o.available);

  useEffect(() => {
    if (!cheapestAvailable?.id) return;
    getPriceHistory(cheapestAvailable.id)
      .then((points) => {
        const mapped = points.map((p) => ({ date: p.recordedAt.slice(0, 10), price: Number(p.price) }));
        setHistory(mapped.filter((_, i) => !points[i].isPromotional).reverse());
        setOfferHistory(mapped.filter((_, i) => points[i].isPromotional).reverse());
      })
      .catch(console.error);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [cheapestAvailable?.id]);

  if (notFound) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Producto no encontrado.</p>
      </div>
    );
  }

  if (!product) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando...</p>
      </div>
    );
  }

  const offerPrice =
    cheapestAvailable?.listPrice && cheapestAvailable.listPrice > cheapestAvailable.price
      ? cheapestAvailable.price
      : undefined;

  const specs = product.model ? { Modelo: product.model } : {};

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb
        items={[
          { label: "Inicio", onClick: () => navigate({ id: "home" }) },
          {
            label: "Productos",
            onClick: () => navigate({ id: "search-products", query: "" }),
          },
          {
            label: product.category,
            onClick: () =>
              navigate({ id: "search-products", query: "", category: product.category }),
          },
          { label: product.name },
        ]}
      />

      {/* Top section */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-10">
        {/* Images */}
        <div
          style={{ background: "#111111", border: "1px solid #2A2A2A" }}
          className="rounded-2xl overflow-hidden h-72 flex items-center justify-center text-muted text-sm"
        >
          {product.images.length > 0 ? (
            <img
              src={product.images[0].url}
              alt={product.images[0].altText || product.name}
              className="w-full h-full object-cover"
            />
          ) : (
            "Sin imagen"
          )}
        </div>

        {/* Info */}
        <div className="flex flex-col gap-4">
          <div>
            <div className="flex items-center justify-between mb-1">
              <span className="text-xs font-bold text-prime uppercase tracking-widest">
                {product.brand} · {product.category}
              </span>
              <FavoriteButton active={isFavorite} onClick={() => onToggleFavorite(product.id)} />
            </div>
            <h1 className="text-2xl font-bold text-text leading-snug">{product.name}</h1>
            <div className="mt-2">
              <Rating value={product.rating} count={product.reviewCount} />
            </div>
          </div>

          <p className="text-sm text-muted leading-relaxed">{product.description}</p>

          {/* Specs preview */}
          <div
            style={{ background: "#111111", border: "1px solid #2A2A2A" }}
            className="rounded-2xl p-4"
          >
            <h3 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3">
              Características principales
            </h3>
            <div className="grid grid-cols-2 gap-2">
              {Object.entries(specs).map(([k, v]) => (
                <div key={k} style={{ background: "#1A1A1A" }} className="rounded-xl px-3 py-2">
                  <div className="text-xs text-muted">{k}</div>
                  <div className="text-sm font-semibold text-text">{v}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Price block: current + offer */}
          {cheapestAvailable && (
            <div className="grid grid-cols-1 gap-2">
              {/* Regular price */}
              <div
                style={{ background: "#1A1A1A", border: "1px solid #2A2A2A" }}
                className="rounded-2xl p-4"
              >
                <div className="text-xs text-muted-2 font-semibold uppercase tracking-wide mb-1">
                  Precio actual
                </div>
                <div className="price text-2xl font-bold text-text">
                  {formatPrice(cheapestAvailable.price)}
                </div>
                <div className="text-xs text-muted mt-1">
                  Mejor precio en {cheapestAvailable.storeName} · Envío:{" "}
                  {cheapestAvailable.shipping === 0 ? (
                    <span className="text-success">Gratis</span>
                  ) : cheapestAvailable.shipping ? (
                    formatPrice(cheapestAvailable.shipping)
                  ) : (
                    "—"
                  )}
                </div>
              </div>

              {/* Offer price */}
              {offerPrice && cheapestAvailable.listPrice ? (
                <div
                  style={{
                    background: "rgba(232,0,27,0.08)",
                    border: "1px solid rgba(232,0,27,0.3)",
                  }}
                  className="rounded-2xl p-4 relative overflow-hidden"
                >
                  <div
                    style={{ background: "#E8001B", color: "white" }}
                    className="absolute top-0 right-0 text-xs font-bold px-3 py-1 rounded-bl-xl"
                  >
                    OFERTA
                  </div>
                  <div className="text-xs text-prime font-semibold uppercase tracking-wide mb-1">
                    Precio oferta
                  </div>
                  <div className="price text-2xl font-bold text-prime">
                    {formatPrice(offerPrice)}
                  </div>
                  <div className="flex items-center gap-3 mt-1 flex-wrap">
                    <span className="text-xs text-muted line-through">
                      {formatPrice(cheapestAvailable.listPrice)}
                    </span>
                    <span
                      style={{
                        background: "rgba(232,0,27,0.15)",
                        color: "#E8001B",
                      }}
                      className="text-xs font-bold px-2 py-0.5 rounded-md"
                    >
                      -{Math.round((1 - offerPrice / cheapestAvailable.listPrice) * 100)}% OFF
                    </span>
                    <span className="text-xs text-muted">
                      Ahorro: {formatPrice(cheapestAvailable.listPrice - offerPrice)}
                    </span>
                  </div>
                </div>
              ) : (
                <div
                  style={{ background: "#1A1A1A", border: "1px solid #2A2A2A" }}
                  className="rounded-2xl p-4 flex items-center gap-3"
                >
                  <div
                    style={{
                      background: "#111111",
                      border: "1px solid #2A2A2A",
                    }}
                    className="w-8 h-8 rounded-lg flex items-center justify-center shrink-0"
                  >
                    <svg
                      width="14"
                      height="14"
                      viewBox="0 0 24 24"
                      fill="none"
                      stroke="#64748B"
                      strokeWidth="2"
                    >
                      <path d="M20 12V22H4V12" />
                      <path d="M22 7H2v5h20V7z" />
                      <path d="M12 22V7" />
                      <path d="M12 7H7.5a2.5 2.5 0 0 1 0-5C11 2 12 7 12 7z" />
                      <path d="M12 7h4.5a2.5 2.5 0 0 0 0-5C13 2 12 7 12 7z" />
                    </svg>
                  </div>
                  <div>
                    <div className="text-xs font-semibold text-muted-2">Sin oferta activa</div>
                    <div className="text-xs text-muted">
                      Agrega a favoritos para recibir alerta de ofertas
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          <div className="flex gap-3">
            <button
              onClick={() => onToggleCompare(product.id)}
              style={
                isComparing
                  ? { background: "#E8001B", color: "#0A0A0A" }
                  : {
                      background: "#1A1A1A",
                      border: "1px solid #2A2A2A",
                      color: "#94A3B8",
                    }
              }
              className="flex-1 py-3 rounded-2xl text-sm font-semibold transition-all hover:border-prime hover:text-prime"
            >
              {isComparing ? "✓ Agregado al comparador" : "Agregar al comparador"}
            </button>
          </div>
        </div>
      </div>

      {/* Price comparison table */}
      <section
        style={{ background: "#111111", border: "1px solid #2A2A2A" }}
        className="rounded-2xl p-6 mb-6"
      >
        <h2 className="text-lg font-bold text-text mb-5">Comparar precios</h2>
        <div className="overflow-x-auto">
          <table className="w-full">
            <thead>
              <tr style={{ borderBottom: "1px solid #2A2A2A" }}>
                <th className="text-left text-xs font-semibold text-muted-2 uppercase tracking-widest pb-3">
                  Tienda
                </th>
                <th className="text-right text-xs font-semibold text-muted-2 uppercase tracking-widest pb-3">
                  Precio
                </th>
                <th className="text-center text-xs font-semibold text-muted-2 uppercase tracking-widest pb-3">
                  Disponibilidad
                </th>
                <th className="text-center text-xs font-semibold text-muted-2 uppercase tracking-widest pb-3">
                  Envío
                </th>
                <th className="text-right text-xs font-semibold text-muted-2 uppercase tracking-widest pb-3">
                  Acción
                </th>
              </tr>
            </thead>
            <tbody>
              {sortedOffers.map((offer, i) => (
                <tr
                  key={offer.id ?? offer.storeId}
                  style={{
                    borderBottom: i < sortedOffers.length - 1 ? "1px solid #1A1A1A" : "none",
                  }}
                >
                  <td className="py-4">
                    <div className="flex items-center gap-3">
                      <div
                        style={{
                          background: "#1A1A1A",
                          border: "1px solid #2A2A2A",
                        }}
                        className="w-9 h-9 rounded-xl flex items-center justify-center text-xs font-bold text-muted-2"
                      >
                        {offer.storeName.slice(0, 2).toUpperCase()}
                      </div>
                      <div>
                        <div className="text-sm font-semibold text-text">{offer.storeName}</div>
                        {offer === cheapestAvailable && <Badge variant="best">Mejor precio</Badge>}
                      </div>
                    </div>
                  </td>
                  <td className="py-4 text-right">
                    <span
                      className={`price text-base font-bold ${
                        offer === cheapestAvailable ? "text-prime" : "text-text"
                      }`}
                    >
                      {formatPrice(offer.price)}
                    </span>
                  </td>
                  <td className="py-4 text-center">
                    <Badge variant={offer.available ? "available" : "unavailable"}>
                      {offer.available ? "Disponible" : "Agotado"}
                    </Badge>
                  </td>
                  <td className="py-4 text-center text-sm text-muted">
                    {offer.shipping === null ? (
                      "—"
                    ) : offer.shipping === 0 ? (
                      <span className="text-success font-medium">Gratis</span>
                    ) : (
                      formatPrice(offer.shipping)
                    )}
                  </td>
                  <td className="py-4 text-right">
                    <a
                      href={offer.available ? offer.url : undefined}
                      target="_blank"
                      rel="noreferrer"
                      style={
                        offer.available
                          ? { background: "#E8001B", color: "#0A0A0A" }
                          : {
                              background: "#1A1A1A",
                              color: "#64748B",
                              cursor: "not-allowed",
                              pointerEvents: "none",
                            }
                      }
                      className="inline-block px-4 py-1.5 rounded-xl text-xs font-semibold transition-opacity hover:opacity-90"
                    >
                      Ver oferta
                    </a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      {/* Price history */}
      <PriceHistory history={history} offerHistory={offerHistory} currentOfferPrice={offerPrice} />

      {/* Full specs */}
      <section
        style={{ background: "#111111", border: "1px solid #2A2A2A" }}
        className="rounded-2xl p-6 mt-6"
      >
        <h2 className="text-lg font-bold text-text mb-5">Especificaciones técnicas</h2>
        <div className="grid grid-cols-1 sm:grid-cols-2 gap-px" style={{ background: "#1A1A1A" }}>
          {Object.entries(specs).map(([k, v]) => (
            <div
              key={k}
              style={{ background: "#111111" }}
              className="px-4 py-3 flex justify-between items-center gap-4"
            >
              <span className="text-sm text-muted">{k}</span>
              <span className="text-sm font-semibold text-text text-right">{v}</span>
            </div>
          ))}
        </div>
      </section>
    </div>
  );
}
