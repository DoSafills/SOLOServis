import type { Product, Page } from '../types'
import { formatPrice, getMinPrice, getMinOffer, getAvailableStoreCount } from '../Services/api/frontend-src/api'
import { Badge, Rating, FavoriteButton } from './ui'

interface Props {
  product: Product
  navigate: (page: Page) => void
  isFavorite: boolean
  isComparing: boolean
  onToggleFavorite: (id: string) => void
  onToggleCompare: (id: string) => void
}

export default function ProductCard({
  product,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  const minPrice = getMinPrice(product)
  const minOffer = getMinOffer(product)
  const storeCount = getAvailableStoreCount(product)

  return (
    <div
      style={{ background: '#111111', border: `1px solid ${isComparing ? '#E8001B' : '#2A2A2A'}` }}
      className="rounded-2xl overflow-hidden hover:border-prime transition-all duration-300 group flex flex-col"
    >
      {/* Image */}
      <div
        className="relative h-44 bg-surface overflow-hidden cursor-pointer"
        onClick={() => navigate({ id: 'product-detail', productId: product.id })}
      >
        <img
          src={product.image}
          alt={product.name}
          className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500"
        />
        <div className="absolute inset-0 bg-gradient-to-t from-surface/60 to-transparent" />
        <div className="absolute top-2 left-2 flex gap-1.5">
          <Badge variant={storeCount > 0 ? 'available' : 'unavailable'}>
            {storeCount > 0 ? `${storeCount} tiendas` : 'Agotado'}
          </Badge>
        </div>
        <div className="absolute top-2 right-2">
          <FavoriteButton active={isFavorite} onClick={(e) => { e.stopPropagation(); onToggleFavorite(product.id) }} />
        </div>
      </div>

      {/* Content */}
      <div className="p-4 flex flex-col flex-1 gap-2">
        <div>
          <span className="text-xs font-semibold text-prime uppercase tracking-wide">{product.brand}</span>
          <h3
            className="text-sm font-semibold text-text leading-snug mt-0.5 cursor-pointer hover:text-prime transition-colors line-clamp-2"
            onClick={() => navigate({ id: 'product-detail', productId: product.id })}
          >
            {product.name}
          </h3>
        </div>

        {/* Specs preview */}
        <div className="flex flex-wrap gap-1">
          {Object.entries(product.specs).slice(0, 2).map(([k, v]) => (
            <span key={k} style={{ background: '#1A1A1A', color: '#64748B' }} className="text-xs px-2 py-0.5 rounded-md">
              {v}
            </span>
          ))}
        </div>

        <Rating value={product.rating} count={product.reviewCount} />

        {/* Price */}
        <div className="mt-auto pt-2">
          <div className="text-xs text-muted mb-0.5">Desde</div>
          <div className="price text-xl font-semibold text-prime">{formatPrice(minPrice)}</div>
          {minOffer && (
            <div className="text-xs text-muted mt-0.5">en {minOffer.storeName}</div>
          )}
        </div>

        {/* Actions */}
        <div className="flex gap-2 mt-1">
          <button
            onClick={() => onToggleCompare(product.id)}
            style={
              isComparing
                ? { background: '#E8001B', color: '#0A0A0A' }
                : { background: '#1A1A1A', border: '1px solid #2A2A2A', color: '#94A3B8' }
            }
            className="flex-1 py-1.5 rounded-xl text-xs font-semibold transition-all hover:border-prime hover:text-prime"
          >
            {isComparing ? '✓ Comparando' : 'Comparar'}
          </button>
          <button
            onClick={() => navigate({ id: 'product-detail', productId: product.id })}
            style={{ background: '#E8001B', color: '#0A0A0A' }}
            className="flex-1 py-1.5 rounded-xl text-xs font-semibold hover:opacity-90 transition-opacity"
          >
            Ver producto
          </button>
        </div>
      </div>
    </div>
  )
}
