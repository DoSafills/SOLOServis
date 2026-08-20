import { useState } from 'react'
import type { Page } from '../types'
import { products, getMinPrice } from '../data/mockData'
import ProductCard from '../components/ProductCard'
import { Breadcrumb, EmptyState, Pagination } from '../components/ui'

interface Props {
  query: string
  navigate: (page: Page) => void
  favorites: Set<string>
  compareList: Set<string>
  onToggleFavorite: (id: string) => void
  onToggleCompare: (id: string) => void
}

type SortOption = 'relevance' | 'price-asc' | 'price-desc' | 'rating'

export default function SearchResultsPage({ query, navigate, favorites, compareList, onToggleFavorite, onToggleCompare }: Props) {
  const [sort, setSort] = useState<SortOption>('relevance')
  const [priceMin, setPriceMin] = useState('')
  const [priceMax, setPriceMax] = useState('')
  const [selectedBrands, setSelectedBrands] = useState<Set<string>>(new Set())
  const [availableOnly, setAvailableOnly] = useState(false)
  const [filtersOpen, setFiltersOpen] = useState(false)
  const [page, setPage] = useState(1)
  const PER_PAGE = 6

  const brands = [...new Set(products.map((p) => p.brand))]

  let filtered = products.filter((p) => {
    if (query && !p.name.toLowerCase().includes(query.toLowerCase()) && !p.brand.toLowerCase().includes(query.toLowerCase()) && !p.category.toLowerCase().includes(query.toLowerCase())) return false
    const min = getMinPrice(p)
    if (priceMin && min < Number(priceMin.replace(/\D/g, ''))) return false
    if (priceMax && min > Number(priceMax.replace(/\D/g, ''))) return false
    if (selectedBrands.size > 0 && !selectedBrands.has(p.brand)) return false
    if (availableOnly && p.offers.every((o) => !o.available)) return false
    return true
  })

  filtered = [...filtered].sort((a, b) => {
    if (sort === 'price-asc') return getMinPrice(a) - getMinPrice(b)
    if (sort === 'price-desc') return getMinPrice(b) - getMinPrice(a)
    if (sort === 'rating') return b.rating - a.rating
    return 0
  })

  const paginated = filtered.slice((page - 1) * PER_PAGE, page * PER_PAGE)

  const toggleBrand = (brand: string) => {
    setSelectedBrands((prev) => {
      const next = new Set(prev)
      if (next.has(brand)) {
        next.delete(brand)
      } else {
        next.add(brand)
      }
      return next
    })
    setPage(1)
  }

  const renderFilters = () => (
    <div className="space-y-6">
      {/* Category */}
      <div>
        <h4 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3">Disponibilidad</h4>
        <label className="flex items-center gap-2 cursor-pointer">
          <input
            type="checkbox"
            checked={availableOnly}
            onChange={(e) => setAvailableOnly(e.target.checked)}
            className="accent-prime"
          />
          <span className="text-sm text-muted-2">Solo disponibles</span>
        </label>
      </div>

      {/* Price */}
      <div>
        <h4 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3">Precio (CLP)</h4>
        <div className="flex gap-2">
          <input
            value={priceMin}
            onChange={(e) => setPriceMin(e.target.value)}
            placeholder="Mín"
            style={{ background: '#1A1A1A', border: '1px solid #2A2A2A' }}
            className="w-full px-3 py-2 rounded-xl text-xs text-text placeholder-muted focus:outline-none focus:border-prime transition-colors"
          />
          <input
            value={priceMax}
            onChange={(e) => setPriceMax(e.target.value)}
            placeholder="Máx"
            style={{ background: '#1A1A1A', border: '1px solid #2A2A2A' }}
            className="w-full px-3 py-2 rounded-xl text-xs text-text placeholder-muted focus:outline-none focus:border-prime transition-colors"
          />
        </div>
      </div>

      {/* Brands */}
      <div>
        <h4 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-3">Marca</h4>
        <div className="space-y-2">
          {brands.map((brand) => (
            <label key={brand} className="flex items-center gap-2 cursor-pointer">
              <input
                type="checkbox"
                checked={selectedBrands.has(brand)}
                onChange={() => toggleBrand(brand)}
                className="accent-prime"
              />
              <span className="text-sm text-muted-2">{brand}</span>
            </label>
          ))}
        </div>
      </div>

      {/* Clear */}
      {(selectedBrands.size > 0 || priceMin || priceMax || availableOnly) && (
        <button
          onClick={() => { setSelectedBrands(new Set()); setPriceMin(''); setPriceMax(''); setAvailableOnly(false) }}
          className="text-xs text-prime hover:text-prime-dark transition-colors"
        >
          Limpiar filtros
        </button>
      )}
    </div>
  )

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb items={[
        { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
        { label: 'Productos' },
        ...(query ? [{ label: `"${query}"` }] : []),
      ]} />

      <div className="flex items-start justify-between mb-6 flex-wrap gap-3">
        <div>
          <h1 className="text-2xl font-bold text-text">
            {query ? `Resultados para "${query}"` : 'Todos los productos'}
          </h1>
          <p className="text-sm text-muted mt-1">{filtered.length} productos encontrados</p>
        </div>

        <div className="flex items-center gap-2 flex-wrap">
          {/* Mobile filter button */}
          <button
            onClick={() => setFiltersOpen(true)}
            style={{ background: '#111111', border: '1px solid #2A2A2A' }}
            className="lg:hidden flex items-center gap-1.5 px-3 py-2 rounded-xl text-sm text-muted-2 hover:border-prime hover:text-prime transition-all"
          >
            <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><line x1="4" y1="6" x2="11" y2="6" /><line x1="8" y1="6" x2="8" y2="2" /><line x1="8" y1="10" x2="8" y2="6" /><line x1="4" y1="18" x2="11" y2="18" /><line x1="8" y1="22" x2="8" y2="18" /><line x1="8" y1="14" x2="8" y2="18" /><line x1="13" y1="12" x2="20" y2="12" /><line x1="16" y1="8" x2="16" y2="12" /><line x1="16" y1="16" x2="16" y2="12" /></svg>
            Filtros
          </button>

          {/* Sort */}
          <select
            value={sort}
            onChange={(e) => setSort(e.target.value as SortOption)}
            style={{ background: '#111111', border: '1px solid #2A2A2A', color: '#94A3B8' }}
            className="px-3 py-2 rounded-xl text-sm focus:outline-none focus:border-prime transition-colors"
          >
            <option value="relevance">Relevancia</option>
            <option value="price-asc">Precio: menor a mayor</option>
            <option value="price-desc">Precio: mayor a menor</option>
            <option value="rating">Mejor valoración</option>
          </select>

          {/* Compare button */}
          {compareList.size >= 2 && (
            <button
              onClick={() => navigate({ id: 'product-comparison', productIds: [...compareList] })}
              style={{ background: '#E8001B', color: '#0A0A0A' }}
              className="px-4 py-2 rounded-xl text-sm font-semibold hover:opacity-90 transition-opacity"
            >
              Comparar {compareList.size} productos
            </button>
          )}
        </div>
      </div>

      <div className="flex gap-6">
        {/* Sidebar filters (desktop) */}
        <aside
          style={{ background: '#111111', border: '1px solid #2A2A2A' }}
          className="hidden lg:block w-56 shrink-0 rounded-2xl p-5 self-start sticky top-24"
        >
          <h3 className="text-sm font-semibold text-text mb-5">Filtros</h3>
          {renderFilters()}
        </aside>

        {/* Mobile filter modal */}
        {filtersOpen && (
          <div className="fixed inset-0 z-50 lg:hidden">
            <div className="absolute inset-0 bg-black/60 backdrop-blur-sm" onClick={() => setFiltersOpen(false)} />
            <div
              style={{ background: '#111111', borderLeft: '1px solid #2A2A2A' }}
              className="absolute right-0 top-0 bottom-0 w-72 p-6 overflow-y-auto"
            >
              <div className="flex items-center justify-between mb-6">
                <h3 className="text-sm font-semibold text-text">Filtros</h3>
                <button onClick={() => setFiltersOpen(false)} className="text-muted hover:text-text transition-colors">
                  <svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M18 6 6 18" /><path d="m6 6 12 12" /></svg>
                </button>
              </div>
              {renderFilters()}
            </div>
          </div>
        )}

        {/* Results */}
        <div className="flex-1 min-w-0">
          {paginated.length === 0 ? (
            <EmptyState
              title="No encontramos resultados"
              description={`No hay productos que coincidan con "${query}". Intenta con otros términos o elimina algunos filtros.`}
              action={{ label: 'Modificar búsqueda', onClick: () => navigate({ id: 'home' }) }}
            />
          ) : (
            <>
              <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-4">
                {paginated.map((p) => (
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
              <Pagination page={page} total={filtered.length} perPage={PER_PAGE} onChange={setPage} />
            </>
          )}
        </div>
      </div>
    </div>
  )
}
