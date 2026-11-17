# SOLOServis Frontend

Aplicación web de SOLOServis para explorar y comparar productos, servicios y
tiendas. Está construida con React 19, TypeScript, Vite 8, Tailwind CSS 4 y
Recharts. Consume APIs REST; no se conecta directamente a PostgreSQL.

## Requisitos

- Node.js compatible con la versión de Vite declarada en `package.json`.
- pnpm `11.24.0` (versión declarada en `package.json`).
- Backend disponible para las funciones que cargan o guardan datos.

## Instalación y desarrollo

Desde esta carpeta (`fronted/`):

```powershell
pnpm install
pnpm dev
```

El servidor escucha en `http://localhost:8443` por defecto. Se puede cambiar el
puerto con `PORT`; Vite no selecciona otro puerto automáticamente si el
configurado está ocupado.

En desarrollo, las peticiones del mismo origen se envían a las APIs locales
mediante el proxy de Vite. Si no se configuran URLs `VITE_*`, los destinos del
proxy son:

| Prefijo                                           | Puerto local |
| ------------------------------------------------- | -----------: |
| `/products`, `/product-categories`, `/categories` |         8081 |
| `/stores`                                         |         8082 |
| `/services`                                       |         8083 |
| `/search`                                         |         8086 |
| `/comparisons`                                    |         8087 |
| `/auth`                                           |         8088 |
| `/favorites`                                      |         8089 |
| `/cart`                                           |         8090 |
| `/reviews`                                        |         8091 |
| `/watchlist`                                      |         8092 |

Para compilar y servir la versión de producción localmente:

```powershell
pnpm build
pnpm preview
```

## Funcionalidades

### Inicio y navegación

- Inicio con productos destacados, productos populares en un carrusel,
  categorías de productos y servicios, y tarjetas de servicios.
- Accesos principales a productos, servicios y tiendas; buscador de productos
  en el encabezado y navegación adaptable a pantallas pequeñas.
- Navegación con historial del navegador: las páginas se representan en la URL
  con parámetros y responden a Atrás/Adelante.
- Preferencia de tema claro u oscuro.

### Productos

- Exploración por categorías y subcategorías.
- Búsqueda con filtros de precio, marca, tienda, atributos y disponibilidad;
  ordenamiento y paginación.
- Página de detalle con imágenes, descripción, especificaciones, calificación,
  reseñas, historial de precios y ofertas por tienda.
- Acciones para marcar favoritos, agregar al carrito o seleccionar para
  comparar.
- Comparación de hasta tres productos de categorías iguales o compatibles.
  Presenta las especificaciones en paralelo y permite abrir la comparación de
  ofertas de un producto.

### Comparación de ofertas

- Compara ofertas del mismo producto entre tiendas y puede mostrar el costo de
  envío junto con el precio.
- Solo considera para destacar un mejor precio las ofertas disponibles, con
  precio finito y positivo, y moneda compatible (CLP).
- Requiere al menos dos ofertas válidas para habilitar la comparación. Si falta
  un costo de envío necesario para calcular el total, no destaca una mejor
  oferta por total.
- No convierte monedas ni unidades de presentación. Las unidades no
  comparables se indican en la comparación de productos.

### Servicios

- Exploración por categorías y búsqueda de servicios.
- Filtros de precio, proveedor, especificaciones y disponibilidad, además de
  ordenamiento y paginación.
- Página de detalle con información del proveedor, precio y periodo de cobro,
  especificaciones, beneficios, cobertura e historial de precios.
- Favoritos, selección para comparar y agregado al carrito.
- Comparación de hasta tres servicios de categorías iguales o compatibles.

### Tiendas

- Listado de tiendas con búsqueda por nombre o proveedor.
- Detalle de tienda con información, ubicaciones y productos publicados.
- Desde los productos de una tienda se puede abrir el detalle, marcar favoritos,
  comparar productos o agregarlos al carrito.
- Los logos usan la información de la tienda; si no hay un logo utilizable, la
  interfaz intenta mostrar el favicon y luego las iniciales.

### Cuenta, favoritos e historial

- Registro, inicio y cierre de sesión.
- Perfil, favoritos e historial de búsquedas de productos y servicios.
- Los favoritos y el historial requieren una sesión autenticada y se cargan
  desde el backend.
- Si una sesión expira, la aplicación la restaura o actualiza cuando es posible;
  los errores de conexión o persistencia se informan en la interfaz.

### Carrito

- Admite productos y servicios; permite cambiar cantidades y quitar elementos.
- En modo invitado, el carrito se guarda localmente. Al iniciar sesión, los
  elementos locales se sincronizan con el carrito de la cuenta.
- Con sesión, las operaciones se guardan mediante la API de carrito.
- El resumen separa los productos por tienda y presenta totales cuando los
  precios y monedas son válidos.
- El frontend no procesa pagos ni completa una compra.

## Funcionamiento y persistencia

- `App.tsx` mantiene el estado compartido de navegación, sesión, favoritos,
  comparaciones y carrito, y conecta las páginas con las acciones de la API.
- La navegación usa `history.pushState` y `popstate`; los parámetros de página
  permiten compartir o restaurar vistas compatibles.
- Se utiliza `localStorage` para preferencias (tema y tamaño de página),
  selecciones de comparación, sesión y carrito de invitado.
- Los filtros de búsqueda se guardan en `sessionStorage` por contexto de
  búsqueda y se validan al restaurarse.
- Los datos recibidos se convierten a los tipos usados por la interfaz. Los
  precios nulos o inválidos se mantienen como no disponibles en vez de usarse
  para calcular el mejor precio.
- Las respuestas de error de las APIs se muestran en la interfaz; cuando una
  respuesta sigue el formato problem+json se utiliza su detalle.

## APIs y configuración

Las URLs base se pueden establecer como variables de entorno `VITE_*`. Si no
se establecen, se usan rutas relativas al mismo origen, normalmente servidas
por el proxy de desarrollo o por el enrutamiento del entorno desplegado.

| Variable                  | Alias compatible       | API                    |
| ------------------------- | ---------------------- | ---------------------- |
| `VITE_PRODUCTS_API_URL`   | `VITE_PRODUCT_API_URL` | Productos y categorías |
| `VITE_STORES_API_URL`     | `VITE_STORE_API_URL`   | Tiendas                |
| `VITE_SERVICES_API_URL`   | `VITE_SERVICE_API_URL` | Servicios              |
| `VITE_SEARCH_API_URL`     | —                      | Búsqueda e historial   |
| `VITE_AUTH_API_URL`       | —                      | Autenticación          |
| `VITE_FAVORITES_API_URL`  | —                      | Favoritos              |
| `VITE_CART_API_URL`       | —                      | Carrito                |
| `VITE_COMPARISON_API_URL` | —                      | Comparaciones          |

El módulo `src/services/api/` concentra las solicitudes HTTP y la adaptación de
respuestas al modelo del frontend. En producción, configura el proxy del
servidor web para dirigir cada prefijo a la API correspondiente.

## Estructura del código

```text
src/
  App.tsx                  Estado compartido, navegación y composición de vistas
  pages/                   Inicio, búsqueda, detalle y comparación
                           de productos/servicios, tiendas y cuenta
  components/              Encabezado, tarjetas, controles y componentes comunes
  services/api/             Clientes HTTP, mapeadores y persistencia remota
  services/pageRouting.ts   Serialización y restauración de páginas en la URL
  services/utils/           Validaciones y utilidades compartidas
  hooks/                    Hooks de persistencia del estado de página
  constants/                Catálogo, categorías y textos comunes
  types.ts                  Tipos compartidos de dominio y navegación
```

## Comprobaciones

```powershell
pnpm format          # Aplicar oxfmt
pnpm format:check    # Verificar formato
pnpm lint            # ESLint
pnpm build           # Compilar para producción
pnpm audit           # Revisar dependencias
pnpm check           # Formato, lint, build y audit
pnpm exec tsc -b     # Verificación de TypeScript
```

No hay una suite de pruebas automatizadas del frontend configurada actualmente.

## Alcance no implementado

Aunque existen APIs relacionadas, este frontend todavía no incluye pantallas
para seguimiento de precios (watchlist) ni notificaciones.
