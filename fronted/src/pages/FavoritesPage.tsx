import { useEffect, useState } from 'react'
import type { Page } from '../types'
import type { Product, Service } from '../types'
import { getProducts, getServices, formatPrice, getMinPrice } from '../Services/api/frontend-src/api'
import { Badge, Breadcrumb, FavoriteButton, Rating } from '../components/ui'

interface Props {
  navigate: (page: Page) => void
  favorites: Set<string>
  onToggleFavorite: (id: string) => void
}

export default function FavoritesPage({ navigate, favorites, onToggleFavorite }: Props) {
  const [products, setProducts] = useState<Product[]>([])
  const [services, setServices] = useState<Service[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function loadData() {
      setLoading(true)
      setError(null)
      try {
        const [productsRes, servicesRes] = await Promise.all([
          getProducts(),
          getServices(),
        ])
        if (cancelled) return
        setProducts(productsRes)
        setServices(servicesRes)
      } catch (err) {
        if (!cancelled) {
          setError(err instanceof Error ? err.message : 'Error al cargar los datos')
        }
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadData()
    return () => {
      cancelled = true
    }
  }, [])

  const favProducts = products.filter((p) => favorites.has(p.id))
  const favServices = services.filter((s) => favorites.has(s.id))

  if (favorites.size === 0) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-8">
        <Breadcrumb items={[
          { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
          { label: 'Favoritos' },
        ]} />
        <div className="flex flex-col items-center justify-center py-24 gap-4 text-center">
          <div className="w-16 h-16 rounded-2xl bg-surface-2 flex items-center justify-center mb-2">
            <svg width="28" height="28" viewBox="0 0 24 24" fill="none" stroke="#64748B" strokeWidth="1.5">
              <path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" />
            </svg>
          </div>
          <h3 className="text-lg font-semibold text-text">No tienes favoritos aún</h3>
          <p className="text-sm text-muted max-w-xs">Guarda productos y servicios para seguir sus precios y recibir alertas de bajadas.</p>
          <button onClick={() => navigate({ id: 'home' })} style={{ background: '#E8001B', color: '#0A0A0A' }} className="mt-2 px-5 py-2 rounded-xl text-sm font-semibold">
            Explorar productos
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb items={[
        { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
        { label: 'Favoritos' },
      ]} />

      <h1 className="text-2xl font-bold text-text mb-1">Mis favoritos</h1>
      <p className="text-sm text-muted mb-8">{favorites.size} items guardados</p>

      {loading && (
        <div className="space-y-3">
          {[...Array(3)].map((_, i) => (
            <div key={i} style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl h-24 animate-pulse" />
          ))}
        </div>
      )}

      {!loading && error && (
        <p className="text-sm text-red-400">No pudimos cargar tus favoritos: {error}</p>
      )}

      {!loading && !error && (
        <>
          {/* Products */}
          {favProducts.length > 0 && (
            <section className="mb-10">
              <h2 className="text-base font-semibold text-muted-2 uppercase tracking-widest text-xs mb-4">Productos</h2>
              <div className="space-y-3">
                {favProducts.map((p) => {
                  const minPrice = getMinPrice(p)
                  // Simulate a previous price for demo
                  const prevPrice = Math.round(minPrice * 1.08 / 1000) * 1000
                  const diff = prevPrice - minPrice
                  return (
                    <div
                      key={p.id}
                      style={{ background: '#111111', border: '1px solid #2A2A2A' }}
                      className="rounded-2xl p-4 flex items-center gap-4 hover:border-prime transition-all group"
                    >
                      <div
                        className="w-20 h-16 rounded-xl overflow-hidden shrink-0 cursor-pointer"
                        onClick={() => navigate({ id: 'product-detail', productId: p.id })}
                      >
                        <img src={p.image} alt={p.name} className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-xs text-prime font-semibold">{p.brand}</div>
                        <div
                          className="text-sm font-semibold text-text truncate cursor-pointer hover:text-prime transition-colors"
                          onClick={() => navigate({ id: 'product-detail', productId: p.id })}
                        >
                          {p.name}
                        </div>
                        <Rating value={p.rating} />
                      </div>
                      <div className="text-right shrink-0">
                        <div className="price text-lg font-bold text-prime">{formatPrice(minPrice)}</div>
                        {diff > 0 && (
                          <div className="flex items-center gap-1 text-xs text-success font-semibold justify-end">
                            <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="3"><path d="m18 15-6-6-6 6" /></svg>
                            Bajó {formatPrice(diff)}
                          </div>
                        )}
                        <div className="text-xs text-muted line-through">{formatPrice(prevPrice)}</div>
                      </div>
                      <div className="flex flex-col items-center gap-2 shrink-0">
                        <FavoriteButton active={true} onClick={() => onToggleFavorite(p.id)} />
                        <Badge variant={p.offers.some((o) => o.available) ? 'available' : 'unavailable'}>
                          {p.offers.some((o) => o.available) ? 'Disponible' : 'Agotado'}
                        </Badge>
                      </div>
                    </div>
                  )
                })}
              </div>
            </section>
          )}

          {/* Services */}
          {favServices.length > 0 && (
            <section>
              <h2 className="text-base font-semibold text-muted-2 uppercase tracking-widest text-xs mb-4">Servicios</h2>
              <div className="space-y-3">
                {favServices.map((s) => {
                  const prevPrice = Math.round(s.monthlyPrice * 1.06 / 100) * 100
                  const diff = prevPrice - s.monthlyPrice
                  return (
                    <div
                      key={s.id}
                      style={{ background: '#111111', border: '1px solid #2A2A2A' }}
                      className="rounded-2xl p-4 flex items-center gap-4 hover:border-prime transition-all group"
                    >
                      <div className="w-20 h-16 rounded-xl overflow-hidden shrink-0 cursor-pointer" onClick={() => navigate({ id: 'service-detail', serviceId: s.id })}>
                        <img src={s.image} alt={s.name} className="w-full h-full object-cover group-hover:scale-105 transition-transform duration-500" />
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="text-xs text-prime font-semibold">{s.provider}</div>
                        <div className="text-sm font-semibold text-text truncate cursor-pointer hover:text-prime transition-colors" onClick={() => navigate({ id: 'service-detail', serviceId: s.id })}>
                          {s.name}
                        </div>
                        <Badge variant="available">{s.category}</Badge>
                      </div>
                      <div className="text-right shrink-0">
                        <div className="price text-lg font-bold text-prime">{formatPrice(s.monthlyPrice)}<span className="text-xs text-muted font-normal">/mes</span></div>
                        {diff > 0 && (
                          <div className="flex items-center gap-1 text-xs text-success font-semibold justify-end">
                            ↓ Bajó {formatPrice(diff)}
                          </div>
                        )}
                      </div>
                      <FavoriteButton active={true} onClick={() => onToggleFavorite(s.id)} />
                    </div>
                  )
                })}
              </div>
            </section>
          )}
        </>
      )}
    </div>
  )
}