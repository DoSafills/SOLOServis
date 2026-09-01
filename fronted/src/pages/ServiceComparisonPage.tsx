import { useEffect, useState } from 'react'
import type { Page, Service } from '../types'
import { getServiceById, formatPrice } from '../Services/api/frontend-src/api'
import { Breadcrumb, Badge } from '../components/ui'

interface Props {
  serviceIds: string[]
  navigate: (page: Page) => void
}

export default function ServiceComparisonPage({ serviceIds, navigate }: Props) {
  const [selected, setSelected] = useState<Service[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    let cancelled = false

    async function loadServices() {
      if (serviceIds.length === 0) {
        setSelected([])
        setLoading(false)
        return
      }

      setLoading(true)
      setError(null)
      try {
        const results = await Promise.all(serviceIds.map((id) => getServiceById(id)))
        if (cancelled) return
        const valid = results.filter((s: Service | null): s is Service => s !== null)
        setSelected(valid)
      } catch (err) {
        if (cancelled) return
        setError(err instanceof Error ? err.message : 'Error al cargar los servicios')
      } finally {
        if (!cancelled) setLoading(false)
      }
    }

    loadServices()

    return () => {
      cancelled = true
    }
  }, [serviceIds])

  if (loading) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted">Cargando servicios…</p>
      </div>
    )
  }

  if (error) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-warn mb-4">{error}</p>
        <button onClick={() => navigate({ id: 'search-services', query: '' })} style={{ background: '#E8001B', color: '#0A0A0A' }} className="px-5 py-2 rounded-xl text-sm font-semibold">
          Buscar servicios
        </button>
      </div>
    )
  }

  if (selected.length < 2) {
    return (
      <div className="max-w-7xl mx-auto px-4 py-20 text-center">
        <p className="text-muted mb-4">Selecciona al menos 2 servicios para comparar.</p>
        <button onClick={() => navigate({ id: 'search-services', query: '' })} style={{ background: '#E8001B', color: '#0A0A0A' }} className="px-5 py-2 rounded-xl text-sm font-semibold">
          Buscar servicios
        </button>
      </div>
    )
  }

  const lowestPrice = Math.min(...selected.map((s) => s.monthlyPrice))
  const highestRating = Math.max(...selected.map((s) => s.rating))

  const compareRows: { key: string; label: string; getValue: (s: Service) => string; highlight?: (values: string[]) => string | null }[] = [
    {
      key: 'monthlyPrice',
      label: 'Precio mensual',
      getValue: (s) => formatPrice(s.monthlyPrice) + '/mes',
    },
    {
      key: 'installation',
      label: 'Instalación',
      getValue: (s) => s.installationCost === 0 ? 'Gratis' : s.installationCost ? formatPrice(s.installationCost) : 'Sin costo',
    },
    {
      key: 'contract',
      label: 'Permanencia',
      getValue: (s) => s.contractMonths ? `${s.contractMonths} meses` : 'Sin permanencia',
    },
    {
      key: 'rating',
      label: 'Valoración',
      getValue: (s) => `★ ${s.rating.toFixed(1)} (${s.reviewCount.toLocaleString('es-CL')})`,
    },
    {
      key: 'coverage',
      label: 'Cobertura',
      getValue: (s) => s.coverage,
    },
  ]

  const specKeys = [...new Set(selected.flatMap((s) => Object.keys(s.specs)))]

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb items={[
        { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
        { label: 'Servicios', onClick: () => navigate({ id: 'search-services', query: '' }) },
        { label: 'Comparación' },
      ]} />

      <h1 className="text-2xl font-bold text-text mb-2">Comparación de servicios</h1>
      <p className="text-sm text-muted mb-8">Comparando {selected.length} servicios</p>

      <div className="overflow-x-auto">
        <table className="w-full min-w-[520px]">
          <thead>
            <tr>
              <th style={{ background: '#111111', borderBottom: '1px solid #2A2A2A', borderRight: '1px solid #2A2A2A' }} className="text-left text-xs font-semibold text-muted-2 uppercase tracking-widest p-4 w-40">
                Característica
              </th>
              {selected.map((s) => (
                <th key={s.id} style={{ background: '#111111', borderBottom: '1px solid #2A2A2A', borderRight: '1px solid #1A1A1A' }} className="p-4 text-center">
                  <div className="flex flex-col items-center gap-2">
                    <div className="w-16 h-12 rounded-xl overflow-hidden">
                      <img src={s.image} alt={s.provider} className="w-full h-full object-cover" />
                    </div>
                    <div>
                      <div className="text-sm font-bold text-prime">{s.provider}</div>
                      <div className="text-xs text-muted-2">{s.name}</div>
                    </div>
                    <button
                      onClick={() => navigate({ id: 'service-detail', serviceId: s.id })}
                      style={{ background: '#1A1A1A', border: '1px solid #2A2A2A', color: '#94A3B8' }}
                      className="text-xs px-3 py-1 rounded-lg hover:border-prime hover:text-prime transition-all"
                    >
                      Ver detalle
                    </button>
                  </div>
                </th>
              ))}
            </tr>
          </thead>

          <tbody>
            {compareRows.map((row, ri) => (
              <tr key={row.key} style={{ background: ri % 2 === 0 ? '#0A0A0A' : 'transparent' }}>
                <td style={{ borderBottom: '1px solid #1A1A1A', borderRight: '1px solid #2A2A2A' }} className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide">
                  {row.label}
                </td>
                {selected.map((s) => {
                  const val = row.getValue(s)
                  const isBest = (row.key === 'monthlyPrice' && s.monthlyPrice === lowestPrice)
                    || (row.key === 'rating' && s.rating === highestRating)
                  return (
                    <td key={s.id} style={{ borderBottom: '1px solid #1A1A1A', borderRight: '1px solid #1A1A1A' }} className="p-4 text-center">
                      <span className={`text-sm font-semibold ${isBest ? 'text-prime' : 'text-text'}`}>{val}</span>
                      {isBest && row.key === 'monthlyPrice' && (
                        <div className="mt-1"><Badge variant="best">Mejor precio</Badge></div>
                      )}
                    </td>
                  )
                })}
              </tr>
            ))}

            {/* Spec rows */}
            {specKeys.map((key, ri) => (
              <tr key={key} style={{ background: (ri + compareRows.length) % 2 === 0 ? '#0A0A0A' : 'transparent' }}>
                <td style={{ borderBottom: '1px solid #1A1A1A', borderRight: '1px solid #2A2A2A' }} className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide">
                  {key}
                </td>
                {selected.map((s) => (
                  <td key={s.id} style={{ borderBottom: '1px solid #1A1A1A', borderRight: '1px solid #1A1A1A' }} className="p-4 text-center">
                    <span className="text-sm text-text">{s.specs[key] ?? <span className="text-muted">—</span>}</span>
                  </td>
                ))}
              </tr>
            ))}

            {/* Benefits */}
            <tr>
              <td style={{ borderRight: '1px solid #2A2A2A' }} className="p-4 text-xs font-semibold text-muted-2 uppercase tracking-wide align-top">
                Beneficios
              </td>
              {selected.map((s) => (
                <td key={s.id} style={{ borderRight: '1px solid #1A1A1A' }} className="p-4">
                  <ul className="space-y-1">
                    {s.benefits.map((b) => (
                      <li key={b} className="flex items-center gap-1.5 text-xs text-muted-2">
                        <svg width="10" height="10" viewBox="0 0 24 24" fill="none" stroke="#E8001B" strokeWidth="3"><path d="M20 6 9 17l-5-5" /></svg>
                        {b}
                      </li>
                    ))}
                  </ul>
                </td>
              ))}
            </tr>
          </tbody>
        </table>
      </div>
    </div>
  )
}