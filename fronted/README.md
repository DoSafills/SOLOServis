# SOLOServis Frontend

Aplicación web de SOLOServis construida con React, TypeScript, Vite y Tailwind
CSS. Consume las APIs REST del backend y presenta el catálogo de productos,
servicios y tiendas.

## Requisitos

- Node.js compatible con la versión de Vite del proyecto.
- pnpm (`fronted/package.json` declara la versión del package manager).
- Backend disponible para las funcionalidades que consultan datos.

## Desarrollo local

Desde esta carpeta (`fronted/`):

```powershell
pnpm install
pnpm dev
```

Vite muestra en la terminal la URL local del frontend (por defecto,
`http://localhost:8443`). Las URLs de APIs son relativas al mismo origen y el
proxy de desarrollo las enruta a los servicios locales. Las URLs se pueden
sobrescribir con variables `VITE_PRODUCTS_API_URL`, `VITE_STORES_API_URL`,
`VITE_SERVICES_API_URL`, `VITE_SEARCH_API_URL`, `VITE_AUTH_API_URL`,
`VITE_FAVORITES_API_URL`, `VITE_CART_API_URL` y `VITE_COMPARISON_API_URL`.

El frontend consume las APIs HTTP configuradas y no se conecta directamente a
la base de datos.

## Tiendas e ilustraciones de categorías

- Los logos de tiendas proceden de `store.logo_url` a través de Store API. Si
  falta el logo o la URL es un placeholder conocido, la página intenta usar el
  favicon del sitio; las iniciales son el último fallback.
- Las ilustraciones SVG de categorías de productos y servicios se cargan desde
  Product API (`GET /categories/artworks/{key}`). Si no se puede cargar una,
  se muestra un emoji minimalista temporal.

## Comprobaciones

```powershell
pnpm lint
pnpm build
```
