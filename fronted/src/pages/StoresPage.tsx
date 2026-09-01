import { useEffect, useState } from 'react'
import type { Page, Product, Store } from '../types'
import { getStores, getProducts } from '../Services/api/frontend-src/api'
import { Badge, Breadcrumb, Rating } from '../components/ui'
import ProductCard from '../components/ProductCard'

interface Props {
  navigate: (page: Page) => void
  favorites: Set<string>
  compareList: Set<string>
  onToggleFavorite: (id: string) => void
  onToggleCompare: (id: string) => void
  storeId?: string
}

function StoreList({ navigate }: { navigate: (page: Page) => void }) {
  const [stores, setStores] = useState<Store[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function loadStores() {
      setLoading(true)
      setError(null)
      try {
        const result = await getStores()
        if (cancelled) return
        setStores(result)
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : 'Error al cargar las tiendas')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadStores()

    return () => {
      cancelled = true
    }
  }, [])

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb items={[
        { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
        { label: 'Tiendas' },
      ]} />

      <h1 className="text-2xl font-bold text-text mb-2">Tiendas y proveedores</h1>
      <p className="text-sm text-muted mb-8">Directorio de tiendas comparadas en nuestra plataforma</p>

      {loading ? (
        <div className="text-center py-20">
          <p className="text-muted">Cargando tiendas…</p>
        </div>
      ) : error ? (
        <div className="text-center py-20">
          <p className="text-warn">{error}</p>
        </div>
      ) : (
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {stores.map((store) => (
            <button
              key={store.id}
              onClick={() => navigate({ id: 'store-detail', storeId: store.id })}
              style={{ background: '#111111', border: '1px solid #2A2A2A' }}
              className="rounded-2xl p-5 text-left hover:border-prime transition-all duration-200 group"
            >
              {/* Logo */}
              <div className="flex items-center gap-3 mb-4">
                <div
                  style={{ background: 'rgba(232,0,27,0.12)', border: '1px solid rgba(232,0,27,0.25)' }}
                  className="w-12 h-12 rounded-xl flex items-center justify-center text-sm font-bold text-prime"
                >
                  {store.logo}
                </div>
                <div>
                  <div className="text-sm font-bold text-text group-hover:text-prime transition-colors">{store.name}</div>
                  <Badge variant={store.reputation === 'Excelente' ? 'best' : 'available'}>
                    {store.reputation}
                  </Badge>
                </div>
              </div>

              <Rating value={store.rating} count={store.reviewCount} />

              <div className="mt-4 space-y-1.5">
                <div className="flex justify-between text-xs">
                  <span className="text-muted">Productos</span>
                  <span className="text-muted-2 font-semibold">{store.productCount.toLocaleString('es-CL')}</span>
                </div>
                <div className="flex justify-between text-xs">
                  <span className="text-muted">Despacho</span>
                  <span className="text-muted-2 font-semibold">{store.dispatchTime}</span>
                </div>
                <div className="flex justify-between text-xs">
                  <span className="text-muted">Sitio</span>
                  <span className="text-prime font-semibold">{store.website}</span>
                </div>
              </div>
            </button>
          ))}
        </div>
      )}
    </div>
  )
}

function StoreDetail({ storeId, navigate, favorites, compareList, onToggleFavorite, onToggleCompare }: Required<Props>) {
  const [store, setStore] = useState<Store | null>(null)
  const [storeProducts, setStoreProducts] = useState<Product[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function loadStoreData() {
      setLoading(true)
      setError(null)
      try {
        const [stores, products]: [Store[], Product[]] = await Promise.all([getStores(), getProducts()])
        if (cancelled) return
        setStore(stores.find((s: Store) => s.id === storeId) ?? null)
        setStoreProducts(products.filter((p: Product) => p.offers.some((o) => o.storeId === storeId)))
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : 'Error al cargar la tienda')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadStoreData()

    return () => {
      cancelled = true
    }
  }, [storeId])

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando tienda…</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn">{error}</p>
      </div>
    )
  }

  if (!store) return null

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb items={[
        { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
        { label: 'Tiendas', onClick: () => navigate({ id: 'stores' }) },
        { label: store.name },
      ]} />

      {/* Store header */}
      <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-6 mb-8">
        <div className="flex items-start gap-4 flex-wrap">
          <div
            style={{ background: 'rgba(232,0,27,0.12)', border: '1px solid rgba(232,0,27,0.3)' }}
            className="w-16 h-16 rounded-2xl flex items-center justify-center text-xl font-bold text-prime shrink-0"
          >
            {store.logo}
          </div>
          <div className="flex-1">
            <div className="flex items-center gap-3 mb-1 flex-wrap">
              <h1 className="text-xl font-bold text-text">{store.name}</h1>
              <Badge variant={store.reputation === 'Excelente' ? 'best' : 'available'}>{store.reputation}</Badge>
            </div>
            <Rating value={store.rating} count={store.reviewCount} />
            <p className="text-sm text-muted mt-2">{store.conditions}</p>
          </div>
          <div className="grid grid-cols-2 gap-3 text-center">
            <div style={{ background: '#1A1A1A' }} className="rounded-xl px-4 py-3">
              <div className="price text-lg font-bold text-prime">{store.productCount.toLocaleString('es-CL')}</div>
              <div className="text-xs text-muted">Productos</div>
            </div>
            <div style={{ background: '#1A1A1A' }} className="rounded-xl px-4 py-3">
              <div className="text-lg font-bold text-text">{store.dispatchTime}</div>
              <div className="text-xs text-muted">Despacho</div>
            </div>
          </div>
        </div>
      </div>

      {/* Products in store */}
      <h2 className="text-lg font-bold text-text mb-4">Productos disponibles en {store.name}</h2>
      <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-4">
        {storeProducts.map((p) => (
          <ProductCard
            key={p.id}
            product={p}
            navigate={navigate}
            isFavorite={favorites.has(p.id)}
            isComparing={compareList.has(p.id)}
            onToggleFavorite={onToggleFavorite}
            onToggleCompare={onToggleCompare}
          />
        ))}
      </div>
    </div>
  )
}

export default function StoresPage(props: Props) {
  if (props.storeId) {
    return (
      <StoreDetail
        {...props}
        storeId={props.storeId}
        favorites={props.favorites}
        compareList={props.compareList}
        onToggleFavorite={props.onToggleFavorite}
        onToggleCompare={props.onToggleCompare}
      />
    )
  }
  return <StoreList navigate={props.navigate} />
}