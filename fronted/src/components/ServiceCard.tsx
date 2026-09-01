import type { Service, Page } from '../types'
import { formatPrice } from '../Services/api/frontend-src/api'
import { Badge, Rating, FavoriteButton } from './ui'

interface Props {
  service: Service
  navigate: (page: Page) => void
  isFavorite: boolean
  isComparing: boolean
  onToggleFavorite: (id: string) => void
  onToggleCompare: (id: string) => void
}

export default function ServiceCard({
  service,
  navigate,
  isFavorite,
  isComparing,
  onToggleFavorite,
  onToggleCompare,
}: Props) {
  return (
    <div
      style={{ background: '#111111', border: `1px solid ${isComparing ? '#E8001B' : '#2A2A2A'}` }}
      className="rounded-2xl overflow-hidden hover:border-prime transition-all duration-300 group flex flex-col"
    >
      {/* Header band */}
      <div className="relative h-36 bg-surface overflow-hidden cursor-pointer" onClick={() => navigate({ id: 'service-detail', serviceId: service.id })}>
        <img src={service.image} alt={service.name} className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
        <div className="absolute inset-0 bg-gradient-to-t from-surface to-transparent opacity-80" />
        <div className="absolute top-2 left-2">
          <Badge variant="available">{service.category}</Badge>
        </div>
        <div className="absolute top-2 right-2">
          <FavoriteButton active={isFavorite} onClick={(e) => { e.stopPropagation(); onToggleFavorite(service.id) }} />
        </div>
        <div className="absolute bottom-2 left-3">
          <span className="text-xs font-bold text-text bg-black/40 px-2 py-0.5 rounded-md backdrop-blur-sm">
            {service.provider}
          </span>
        </div>
      </div>

      {/* Content */}
      <div className="p-4 flex flex-col flex-1 gap-2">
        <div>
          <span className="text-xs font-semibold text-prime uppercase tracking-wide">{service.subcategory}</span>
          <h3
            className="text-sm font-semibold text-text leading-snug mt-0.5 cursor-pointer hover:text-prime transition-colors"
            onClick={() => navigate({ id: 'service-detail', serviceId: service.id })}
          >
            {service.name}
          </h3>
        </div>

        {/* Key spec */}
        <div className="flex flex-wrap gap-1">
          {Object.entries(service.specs).slice(0, 2).map(([k, v]) => (
            <span key={k} style={{ background: '#1A1A1A', color: '#64748B' }} className="text-xs px-2 py-0.5 rounded-md">
              {v}
            </span>
          ))}
        </div>

        <Rating value={service.rating} count={service.reviewCount} />

        {/* Price */}
        <div className="mt-auto pt-2">
          <div className="price text-xl font-semibold text-prime">
            {formatPrice(service.monthlyPrice)}<span className="text-xs font-normal text-muted"> /mes</span>
          </div>
          <div className="flex gap-3 mt-1 text-xs text-muted">
            <span>Instalación: {service.installationCost === 0 ? 'Gratis' : service.installationCost ? formatPrice(service.installationCost) : 'Sin costo'}</span>
            <span>Contrato: {service.contractMonths ? `${service.contractMonths} meses` : 'Sin permanencia'}</span>
          </div>
        </div>

        {/* Actions */}
        <div className="flex gap-2 mt-1">
          <button
            onClick={() => onToggleCompare(service.id)}
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
            onClick={() => navigate({ id: 'service-detail', serviceId: service.id })}
            style={{ background: '#E8001B', color: '#0A0A0A' }}
            className="flex-1 py-1.5 rounded-xl text-xs font-semibold hover:opacity-90 transition-opacity"
          >
            Ver servicio
          </button>
        </div>
      </div>
    </div>
  )
}
