import type { Product, Page } from "../types";
import { Badge, Rating, FavoriteButton } from "./ui";

interface Props {
  product: Product;
  navigate: (page: Page) => void;
  isFavorite: boolean;
  isComparing: boolean;
  onToggleFavorite: (id: string) => void;
  onToggleCompare: (id: string) => void;
}

export default function ProductCard({
  product,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
}: Props) {

  return (
  <div
    style={{
      background: "#F1F5F9",
      border: `1px solid ${isComparing ? "#0369A1" : "#CBD5E1"}`,
    }}
    className="rounded-2xl overflow-hidden hover:border-prime transition-all duration-300 group flex flex-col"
  >
    {/* Image */}
    <div
      className="relative h-44 bg-surface overflow-hidden cursor-pointer"
      onClick={() => navigate({ id: "product-detail", productId: product.id })}
    >
      {product.image ? (
        <img
          src={product.image}
          alt={product.name}
          className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
        />
      ) : (
        <div className="w-full h-full flex items-center justify-center text-muted text-sm">
          Sin imagen
        </div>
      )}

      <div className="absolute inset-0 bg-gradient-to-t from-surface/60 to-transparent" />

      <div className="absolute top-2 left-2 flex gap-1.5">
        <Badge variant="unavailable">
          Sin ofertas
        </Badge>
      </div>

      <div className="absolute top-2 right-2">
        <FavoriteButton
          active={isFavorite}
          onClick={(e) => {
            e.stopPropagation();
            onToggleFavorite(product.id);
          }}
        />
      </div>
    </div>

      {/* Content */}
      <div className="p-4 flex flex-col flex-1 gap-2">
        <div>
          <span className="text-xs font-semibold text-prime uppercase tracking-wide">
            {product.brand}
          </span>
          <h3
            className="text-sm font-semibold text-text leading-snug mt-0.5 cursor-pointer hover:text-prime transition-colors line-clamp-2"
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
          >
            {product.name}
          </h3>
        </div>

        {/* Specs preview */}
        <div className="flex flex-wrap gap-1">
          {Object.entries(product.specs)
            .slice(0, 2)
            .map(([k, v]) => (
              <span
                key={k}
                style={{ background: "#E2E8F0", color: "#64748B" }}
                className="text-xs px-2 py-0.5 rounded-md"
              >
                {v}
              </span>
            ))}
        </div>

        <Rating value={product.rating} count={product.reviewCount} />

        {/* Price */}
        <div className="text-xs text-muted mb-0.5">Precio</div>
<div className="text-sm font-semibold text-muted">
  No disponible
</div>

        {/* Actions */}
        <div className="flex gap-2 mt-1">
          <button
            onClick={() => onToggleCompare(product.id)}
            style={
              isComparing
                ? { background: "#FFFFFF", color: "#0F172A" }
                : {
                    background: "#F1F5F9",
                    border: "1px solid #CBD5E1",
                    color: "#94A3B8",
                  }
            }
            className="flex-1 py-1.5 rounded-xl text-xs font-semibold transition-all hover:border-prime hover:text-prime"
          >
            {isComparing ? "✓ Comparando" : "Comparar"}
          </button>
          <button
            onClick={() => navigate({ id: "product-detail", productId: product.id })}
            style={{ background: "#0369A1", color: "#FFFFFF" }}
            className="flex-1 py-1.5 rounded-xl text-xs font-semibold hover:opacity-90 transition-opacity"
          >
            Ver producto
          </button>
        </div>
      </div>
    </div>
  );
}
