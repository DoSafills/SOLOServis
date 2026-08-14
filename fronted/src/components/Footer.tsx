import type { Page } from '../types'

interface Props {
  navigate: (page: Page) => void
}

export default function Footer({ navigate }: Props) {
  return (
    <footer style={{ background: '#0A0A0A', borderTop: '1px solid #2A2A2A' }} className="mt-16">
      <div className="max-w-7xl mx-auto px-4 py-12">
        <div className="grid grid-cols-2 md:grid-cols-4 gap-8 mb-10">
          {/* Brand */}
          <div className="col-span-2 md:col-span-1">
            <button onClick={() => navigate({ id: 'home' })} className="flex items-center gap-2 mb-4">
              <div className="w-8 h-8 rounded-lg bg-prime flex items-center justify-center">
                <svg width="16" height="16" viewBox="0 0 16 16" fill="none">
                  <path d="M2 3h12M2 8h8M2 13h10" stroke="#0A0A0A" strokeWidth="2.5" strokeLinecap="round" />
                  <circle cx="13" cy="13" r="2" fill="#0A0A0A" />
                </svg>
              </div>
              <span className="font-bold text-lg tracking-tight text-text">
                compare<span className="text-prime">ya</span>
              </span>
            </button>
            <p className="text-xs text-muted leading-relaxed">
              Compara precios entre cientos de tiendas y proveedores. Encuentra siempre la mejor oferta.
            </p>
            <div className="flex gap-3 mt-4">
              {['Twitter', 'Instagram', 'Facebook'].map((s) => (
                <button key={s} className="w-8 h-8 rounded-lg bg-surface-2 text-muted hover:text-prime hover:bg-prime-muted transition-all flex items-center justify-center text-xs font-bold">
                  {s[0]}
                </button>
              ))}
            </div>
          </div>

          {/* Links */}
          <div>
            <h4 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-4">Plataforma</h4>
            <ul className="space-y-2.5">
              {['Sobre nosotros', 'Cómo funciona', 'Para tiendas', 'Blog'].map((l) => (
                <li key={l}>
                  <button className="text-sm text-muted hover:text-prime transition-colors">{l}</button>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h4 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-4">Soporte</h4>
            <ul className="space-y-2.5">
              {['Centro de ayuda', 'Preguntas frecuentes', 'Contacto', 'Reportar error'].map((l) => (
                <li key={l}>
                  <button className="text-sm text-muted hover:text-prime transition-colors">{l}</button>
                </li>
              ))}
            </ul>
          </div>

          <div>
            <h4 className="text-xs font-semibold text-muted-2 uppercase tracking-widest mb-4">Legal</h4>
            <ul className="space-y-2.5">
              {['Términos de uso', 'Privacidad', 'Cookies', 'Aviso legal'].map((l) => (
                <li key={l}>
                  <button className="text-sm text-muted hover:text-prime transition-colors">{l}</button>
                </li>
              ))}
            </ul>
          </div>
        </div>

        <div style={{ borderTop: '1px solid #2A2A2A' }} className="pt-6 flex flex-col sm:flex-row justify-between items-center gap-3">
          <p className="text-xs text-muted">
            © 2025 Compareya. Todos los derechos reservados.
          </p>
          <p className="text-xs text-muted text-center sm:text-right">
            Los precios pueden variar dependiendo de la tienda o proveedor.
          </p>
        </div>
      </div>
    </footer>
  )
}
