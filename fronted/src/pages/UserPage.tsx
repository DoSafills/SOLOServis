import { useState } from 'react'
import type { Page } from '../types'
import { products, formatPrice, getMinPrice } from '../data/mockData'
import { Breadcrumb, Badge } from '../components/ui'

interface Props {
  navigate: (page: Page) => void
  favorites: Set<string>
}

type Tab = 'profile' | 'favorites' | 'history' | 'watched' | 'settings'

export default function UserPage({ navigate, favorites }: Props) {
  const [activeTab, setActiveTab] = useState<Tab>('profile')

  const tabs: { id: Tab; label: string; icon: React.ReactNode }[] = [
    { id: 'profile', label: 'Perfil', icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20 21v-2a4 4 0 0 0-4-4H8a4 4 0 0 0-4 4v2" /><circle cx="12" cy="7" r="4" /></svg> },
    { id: 'favorites', label: 'Favoritos', icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M20.84 4.61a5.5 5.5 0 0 0-7.78 0L12 5.67l-1.06-1.06a5.5 5.5 0 0 0-7.78 7.78l1.06 1.06L12 21.23l7.78-7.78 1.06-1.06a5.5 5.5 0 0 0 0-7.78z" /></svg> },
    { id: 'history', label: 'Historial', icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="10" /><polyline points="12 6 12 12 16 14" /></svg> },
    { id: 'watched', label: 'Observados', icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><path d="M1 12s4-8 11-8 11 8 11 8-4 8-11 8-11-8-11-8z" /><circle cx="12" cy="12" r="3" /></svg> },
    { id: 'settings', label: 'Configuración', icon: <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2"><circle cx="12" cy="12" r="3" /><path d="M19.4 15a1.65 1.65 0 0 0 .33 1.82l.06.06a2 2 0 0 1-2.83 2.83l-.06-.06a1.65 1.65 0 0 0-1.82-.33 1.65 1.65 0 0 0-1 1.51V21a2 2 0 0 1-4 0v-.09A1.65 1.65 0 0 0 9 19.4a1.65 1.65 0 0 0-1.82.33l-.06.06a2 2 0 0 1-2.83-2.83l.06-.06A1.65 1.65 0 0 0 4.68 15a1.65 1.65 0 0 0-1.51-1H3a2 2 0 0 1 0-4h.09A1.65 1.65 0 0 0 4.6 9a1.65 1.65 0 0 0-.33-1.82l-.06-.06a2 2 0 0 1 2.83-2.83l.06.06A1.65 1.65 0 0 0 9 4.68a1.65 1.65 0 0 0 1-1.51V3a2 2 0 0 1 4 0v.09a1.65 1.65 0 0 0 1 1.51 1.65 1.65 0 0 0 1.82-.33l.06-.06a2 2 0 0 1 2.83 2.83l-.06.06A1.65 1.65 0 0 0 19.4 9a1.65 1.65 0 0 0 1.51 1H21a2 2 0 0 1 0 4h-.09a1.65 1.65 0 0 0-1.51 1z" /></svg> },
  ]

  const searchHistory = ['RTX 4060', 'iPhone 15', 'Notebook gaming', 'Internet hogar 500 Mbps', 'Samsung Galaxy S25']
  const watched = products.slice(0, 3)

  return (
    <div className="max-w-7xl mx-auto px-4 py-8">
      <Breadcrumb items={[
        { label: 'Inicio', onClick: () => navigate({ id: 'home' }) },
        { label: 'Mi cuenta' },
      ]} />

      <div className="flex gap-6 flex-col lg:flex-row">
        {/* Sidebar */}
        <aside className="lg:w-56 shrink-0">
          {/* Avatar */}
          <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-5 mb-4 flex flex-col items-center gap-3">
            <div
              style={{ background: 'rgba(232,0,27,0.15)', border: '2px solid rgba(232,0,27,0.4)' }}
              className="w-16 h-16 rounded-full flex items-center justify-center text-xl font-bold text-prime"
            >
              JG
            </div>
            <div className="text-center">
              <div className="text-sm font-bold text-text">Juan González</div>
              <div className="text-xs text-muted">juan@email.cl</div>
            </div>
            <Badge variant="best">Pro</Badge>
          </div>

          {/* Nav */}
          <nav style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-2">
            {tabs.map((tab) => (
              <button
                key={tab.id}
                onClick={() => setActiveTab(tab.id)}
                style={activeTab === tab.id
                  ? { background: 'rgba(232,0,27,0.12)', color: '#E8001B' }
                  : { color: '#64748B' }
                }
                className="w-full flex items-center gap-2.5 px-3 py-2.5 rounded-xl text-sm font-medium transition-all hover:text-prime hover:bg-prime-muted"
              >
                {tab.icon}
                {tab.label}
                {tab.id === 'favorites' && favorites.size > 0 && (
                  <span className="ml-auto text-xs bg-prime text-bg font-bold rounded-full w-5 h-5 flex items-center justify-center">
                    {favorites.size}
                  </span>
                )}
              </button>
            ))}
          </nav>
        </aside>

        {/* Content */}
        <div className="flex-1 min-w-0">
          {activeTab === 'profile' && (
            <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-6">
              <h2 className="text-lg font-bold text-text mb-6">Información de perfil</h2>
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                {[
                  { label: 'Nombre', value: 'Juan', placeholder: 'Tu nombre' },
                  { label: 'Apellido', value: 'González', placeholder: 'Tu apellido' },
                  { label: 'Email', value: 'juan@email.cl', placeholder: 'tu@email.cl' },
                  { label: 'Teléfono', value: '+56 9 1234 5678', placeholder: '+56 9 xxxx xxxx' },
                ].map((field) => (
                  <div key={field.label}>
                    <label className="text-xs font-semibold text-muted-2 uppercase tracking-wide mb-2 block">{field.label}</label>
                    <input
                      defaultValue={field.value}
                      placeholder={field.placeholder}
                      style={{ background: '#1A1A1A', border: '1px solid #2A2A2A' }}
                      className="w-full px-4 py-2.5 rounded-xl text-sm text-text placeholder-muted focus:outline-none focus:border-prime transition-colors"
                    />
                  </div>
                ))}
              </div>
              <button style={{ background: '#E8001B', color: '#0A0A0A' }} className="mt-6 px-5 py-2.5 rounded-xl text-sm font-semibold hover:opacity-90 transition-opacity">
                Guardar cambios
              </button>
            </div>
          )}

          {activeTab === 'favorites' && (
            <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-6">
              <h2 className="text-lg font-bold text-text mb-4">Mis favoritos</h2>
              {favorites.size === 0 ? (
                <p className="text-sm text-muted">No tienes favoritos aún.</p>
              ) : (
                <p className="text-sm text-muted">{favorites.size} items guardados.</p>
              )}
              <button onClick={() => navigate({ id: 'favorites' })} style={{ background: '#1A1A1A', border: '1px solid #2A2A2A', color: '#94A3B8' }} className="mt-4 px-4 py-2 rounded-xl text-sm hover:border-prime hover:text-prime transition-all">
                Ver todos los favoritos →
              </button>
            </div>
          )}

          {activeTab === 'history' && (
            <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-6">
              <h2 className="text-lg font-bold text-text mb-4">Historial de búsquedas</h2>
              <div className="space-y-2">
                {searchHistory.map((q, i) => (
                  <div key={i} style={{ background: '#1A1A1A' }} className="flex items-center justify-between px-4 py-3 rounded-xl">
                    <div className="flex items-center gap-3">
                      <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="#64748B" strokeWidth="2"><circle cx="11" cy="11" r="8" /><path d="m21 21-4.35-4.35" /></svg>
                      <span className="text-sm text-text">{q}</span>
                    </div>
                    <button
                      onClick={() => navigate({ id: 'search-products', query: q })}
                      className="text-xs text-prime hover:text-prime-dark transition-colors"
                    >
                      Buscar →
                    </button>
                  </div>
                ))}
              </div>
            </div>
          )}

          {activeTab === 'watched' && (
            <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-6">
              <h2 className="text-lg font-bold text-text mb-4">Productos observados</h2>
              <div className="space-y-3">
                {watched.map((p) => (
                  <div
                    key={p.id}
                    style={{ background: '#1A1A1A' }}
                    className="flex items-center gap-3 p-3 rounded-xl cursor-pointer hover:bg-surface-3 transition-colors"
                    onClick={() => navigate({ id: 'product-detail', productId: p.id })}
                  >
                    <img src={p.image} alt={p.name} className="w-14 h-10 rounded-lg object-cover" />
                    <div className="flex-1 min-w-0">
                      <div className="text-xs text-prime">{p.brand}</div>
                      <div className="text-sm font-semibold text-text truncate">{p.name}</div>
                    </div>
                    <div className="price text-sm font-bold text-prime shrink-0">{formatPrice(getMinPrice(p))}</div>
                  </div>
                ))}
              </div>
            </div>
          )}

          {activeTab === 'settings' && (
            <div style={{ background: '#111111', border: '1px solid #2A2A2A' }} className="rounded-2xl p-6">
              <h2 className="text-lg font-bold text-text mb-6">Configuración</h2>
              <div className="space-y-4">
                {[
                  { label: 'Notificaciones de bajada de precio', desc: 'Recibe alertas cuando un favorito baje de precio' },
                  { label: 'Alertas de disponibilidad', desc: 'Aviso cuando un producto agotado vuelva a estar disponible' },
                  { label: 'Resumen semanal', desc: 'Email con las mejores ofertas de la semana' },
                ].map((setting, i) => (
                  <div key={i} style={{ background: '#1A1A1A', border: '1px solid #2A2A2A' }} className="flex items-center justify-between p-4 rounded-xl">
                    <div>
                      <div className="text-sm font-semibold text-text">{setting.label}</div>
                      <div className="text-xs text-muted mt-0.5">{setting.desc}</div>
                    </div>
                    <div
                      style={{ background: i === 0 ? '#E8001B' : '#2A2A2A' }}
                      className="w-10 h-5 rounded-full relative cursor-pointer transition-colors"
                    >
                      <div
                        style={{ background: 'white', left: i === 0 ? '20px' : '2px' }}
                        className="absolute top-0.5 w-4 h-4 rounded-full transition-all"
                      />
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
