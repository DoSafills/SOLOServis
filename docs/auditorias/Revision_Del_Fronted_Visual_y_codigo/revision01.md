# Auditoría del frontend: Tarea TAL-111

**Fecha:** 30 de octubre de 2026
**Tipo de revisión:** solo lectura. No se modificó ningún archivo, no se hicieron commits ni branches.
**Cómo se revisó:** lectura del código, lint, build, type-check y pruebas manuales en el navegador.

---

## 1. Resumen en pocas palabras

- **Tecnologías:** React 19, TypeScript, Vite 8 y Tailwind CSS 4.
- **Qué se revisó:** 40 archivos de `fronted/src` (14 páginas y 8 componentes reutilizables), más la configuración del proyecto.
- **Hallazgos:** 12 en total (1 Alta, 8 Medias, 3 Bajas) y 1 observación de entorno. Ninguno es Crítico.
- **Lo más importante:**
  1. Los productos de una tienda **no se pueden agregar al carrito** (ID de oferta inventado).
  2. El **type-check falla** con 5 errores (el build sí pasa).
  3. **Tiendas y Favoritos** se ven distintos al resto de la app.

### Resultado de las verificaciones automáticas

| Verificación | Resultado |
|---|---|
| Lint | Correcto, sin errores |
| Build | Correcto (avisos de configuración y bundle de 791 kB) |
| Type-check (`tsc`) | **Falla con 5 errores** |
| Tests | No existen |

---

## 2. Tabla resumen de hallazgos

| ID | Severidad | Tema | Origen |
|---|---|---|---|
| TAL111-01 | Alta | Productos de tienda no se agregan al carrito | Código + prueba manual |
| TAL111-02 | Media | Errores de TypeScript | Código |
| TAL111-03 | Media | Sin URLs ni historial ni página 404 | Código |
| TAL111-04 | Media | Buscador del header solo busca productos | Código |
| TAL111-05 | Media | Botones y buscador de Tiendas y Footer sin función | Código + prueba manual |
| TAL111-06 | Media | Errores de API no se muestran al usuario | Código + prueba manual |
| TAL111-07 | Baja | Cifra "2.400" escrita a mano | Código |
| TAL111-08 | Baja | Idioma del documento y buscador sin etiqueta | Código |
| TAL111-09 | Baja | Archivo duplicado `products-client.ts` | Código |
| TAL111-10 | Media | Restos de categorías en Home | Prueba manual |
| TAL111-11 | Media | Tiendas y Favoritos con diseño antiguo | Prueba manual |
| TAL111-12 | Media | No se entra al detalle de productos desde una tienda | Prueba manual |

---

## 3. Detalle de cada hallazgo

### TAL111-01 | Alta | Productos de tienda no se agregan al carrito

- **Dónde:** `fronted/src/pages/stores/StoresPage.tsx:6,176,260`, `fronted/src/services/api/api.ts:28-45,97-115`, `backend/internal/stores/dto/store.go:23-43,88-106`
- **Qué pasa:** la ficha de tienda se inventa un `offerId` (empieza en 1000) para cada producto. El backend no trae el ID real de la oferta en ese caso, así que al agregar al carrito recibe un ID falso.
- **Lo que se vio en pruebas:** aparece el error **"this offer has not been found"**. Está confirmado.
- **Cómo se arregla:** agregar el `offerId` real al DTO del producto de tienda, guardarlo en el frontend y quitar el contador artificial.
- **Esfuerzo:** pequeño, pero toca backend y frontend.

### TAL111-02 | Media | Errores de TypeScript

- **Dónde:** `fronted/src/App.tsx:810`, `fronted/src/services/api/personalization.ts:36`, `fronted/src/services/api/products-client.ts:5-16,39-64`
- **Qué pasa:** el type-check da 5 errores:
  - falta la prop obligatoria `cartProductIds` en `PriceOfferComparison`;
  - `ApiError` no cumple la regla `erasableSyntaxOnly`;
  - el tipo de imágenes de `ApiProductDetail` no cumple `ApiImage`;
  - `products-client.ts` tiene errores con `specifications` y con ofertas sin `offerId`.
- **Por qué importa:** el build pasa porque Vite no valida tipos, pero la prop faltante puede causar un error real al usar `PriceOfferComparison`.
- **Cómo se arregla:** completar la prop, corregir los tipos y agregar un script `typecheck` en `package.json`.

### TAL111-03 | Media | Sin URLs, sin historial y sin página 404

- **Dónde:** `fronted/src/App.tsx:171,351-352,767-903`
- **Qué pasa:** la navegación solo cambia un estado interno. Si recargas (F5) en una página interna vuelves a Inicio, el botón Atrás no funciona y no se pueden compartir enlaces. Tampoco hay 404: cualquier ruta desconocida muestra Inicio.
- **Cómo se arregla:** agregar un router con URLs, declarar las rutas existentes y crear una página 404.
- **Esfuerzo:** mediano. Es un cambio grande, conviene hacerlo en una tarea aparte.

### TAL111-04 | Media | El buscador del header solo busca productos

- **Dónde:** `fronted/src/components/common/Header.tsx:15-18,87,260`
- **Qué pasa:** el campo dice "Buscar producto o servicio", pero siempre manda a la búsqueda de productos. Los servicios nunca se buscan desde ahí.
- **Cómo se arregla:** buscar en ambos catálogos, dejar elegir el tipo, o cambiar el texto del campo.

### TAL111-05 | Media | Controles sin función en Tiendas y Footer

- **Dónde:** `fronted/src/pages/stores/StoresPage.tsx:66-72,77-89`, `fronted/src/components/common/Footer.tsx:38-40,55-90`
- **Qué pasa:**
  - El buscador de Tiendas no tiene estado ni `onChange`, así que **no filtra nada** (no depende de la cantidad de tiendas). Confirmado en pruebas.
  - El botón "Agregar tienda" no hace nada.
  - Si la lista llega vacía, se ve una cuadrícula en blanco sin mensaje.
  - Los botones de redes sociales y las opciones de Plataforma, Soporte y Legal del footer tampoco hacen nada.
- **Cómo se arregla:** implementar lo que ya se pueda, o mostrarlo como texto no clicable mientras no haya destino. Filtrar tiendas y agregar un mensaje de lista vacía.

### TAL111-06 | Media | Errores de API no se muestran al usuario

- **Dónde:** `fronted/src/pages/home/HomePage.tsx:54-66,297-328`, `fronted/src/pages/products/ProductDetailPage.tsx:103-145`, `fronted/src/pages/services/ServiceDetailPage.tsx:65-98`, `fronted/src/pages/services/ServiceComparisonPage.tsx:42-80`
- **Qué pasa:**
  - Inicio solo hace `console.error` si el API falla: sin carga, sin error y sin mensaje.
  - El detalle de producto muestra "Producto no encontrado" aunque el problema sea que el API está caído.
  - Servicios (detalle y comparación) hacen algo parecido.
- **Lo que se vio en pruebas:** con el API bloqueado por CORS, Inicio quedó vacío sin ningún aviso.
- **Cómo se arregla:** separar "no encontrado" de "error del servidor", mostrar el error al usuario y agregar un botón de reintentar.

### TAL111-07 | Baja | Cifra fija "2.400"

- **Dónde:** `fronted/src/pages/home/HomePage.tsx:143,228`
- **Qué pasa:** se muestra "2.400 tiendas/proveedores" como dato real y "en tiempo real", pero está escrito a mano.
- **Cómo se arregla:** obtener el dato del API, o quitar la cifra y la frase de "tiempo real".

### TAL111-08 | Baja | Idioma y accesibilidad

- **Dónde:** `fronted/.figma/make/site.json:4`, `fronted/src/pages/home/HomePage.tsx:174-177`
- **Qué pasa:** el documento está en `language: "en"` pero la interfaz es en español. Además, el buscador de Inicio solo tiene `placeholder`, sin `label` ni `aria-label`.
- **Cómo se arregla:** poner `language` en `"es"` y agregar un `aria-label` al buscador.

### TAL111-09 | Baja | Archivo duplicado

- **Dónde:** `fronted/src/services/api/products-client.ts:1-85` (duplica a `products.ts` y `api.ts`)
- **Qué pasa:** nadie importa este archivo y repite lógica que ya existe en otros, pero igual genera errores de TypeScript.
- **Cómo se arregla:** confirmar si se usa. Si no, borrarlo con aprobación. Si sí, dejarlo como único adaptador.

### TAL111-10 | Media | Restos de categorías en Home

- **Dónde:** `fronted/src/pages/home/HomePage.tsx` (sección de productos, línea por ubicar)
- **Qué pasa:** en el apartado de productos del Home todavía aparecen elementos de categorías, aunque las categorías ya no existen.
- **Cómo se arregla:** quitar esos elementos y revisar que no queden enlaces ni textos que apunten a categorías.
- **Estado:** verificado visualmente, falta ubicar archivo y línea.

### TAL111-11 | Media | Tiendas y Favoritos con diseño antiguo

- **Dónde:** `fronted/src/pages/stores/StoresPage.tsx`, `fronted/src/pages/user/FavoritesPage.tsx` (líneas por ubicar)
- **Qué pasa:** ambas páginas conservan el diseño viejo y no combinan con Productos y Servicios.
- **Cómo se arregla:** igualarlas a Productos y Servicios (tarjetas, encabezados, colores y espaciados) reutilizando los componentes que ya existen. Es la solución más rápida.
- **Estado:** verificado visualmente, falta ubicar archivo y línea.

### TAL111-12 | Media | No se entra al detalle de productos desde una tienda

- **Dónde:** `fronted/src/pages/stores/StoresPage.tsx` (ficha de tienda, línea por ubicar)
- **Qué pasa:** desde la ficha de una tienda no se puede abrir el detalle de sus productos.
- **Posible causa:** la misma de TAL111-01 (IDs inventados), pero aún no está verificado en el código.
- **Cómo se arregla:** confirmar si falla por ID inválido o por falta de handler, y corregirlo junto con TAL111-01.

---

## 4. Observación de entorno (Baja)

- La URL del API está fija en `localhost` (puertos 8081 y 8083) y el backend solo permite el origen `http://localhost:8443` en CORS.
- Si abres el frontend desde `http://192.168.1.9:8443`, todas las llamadas fallan por CORS.
- **Solución rápida:** abrir siempre `http://localhost:8443` en tu PC.
- **Para probar desde otro dispositivo:** hay que permitir ese origen en el backend y cambiar la URL base del API en el frontend (mejor con variables de entorno).

---

## 5. Lo que no se pudo verificar

- Contraste de colores, comportamiento responsive real y lector de pantalla (solo se leyeron clases y estructura).
- Autenticación real: las protecciones de cuenta se revisaron solo a nivel de interfaz.
- No hay `TODO`, `FIXME`, `HACK`, "coming soon", lorem ipsum ni `console.log` en el código.

---

## 6. Qué atacar primero

| Prioridad | Qué hacer | Esfuerzo |
|---|---|---|
| 1 | Arreglar el `offerId` real (TAL111-01) y verificar si también resuelve TAL111-12 | Pequeño |
| 2 | Igualar el diseño de Tiendas y Favoritos (TAL111-11) y limpiar categorías del Home (TAL111-10) | Pequeño a mediano |
| 3 | Resolver los errores de TypeScript y agregar el script `typecheck` (TAL111-02) | Mediano |
| 4 | Mostrar estados de error, carga y vacío (TAL111-05 y 06) y arreglar el buscador (TAL111-04) | Pequeño a mediano |
| 5 | Agregar router con URLs y página 404 (TAL111-03), en una tarea aparte | Mediano a grande |
| 6 | Limpieza: idioma, etiqueta del buscador, cifra fija y archivo duplicado (TAL111-07, 08 y 09) | Pequeño |

---

## 7. Archivos revisados

- **Páginas (14)**, en `fronted/src/pages/`: HomePage, PriceOfferComparison, ProductCategoriesPage, ProductComparisonPage, ProductDetailPage, SearchResultsPage, ServiceCategoriesPage, ServiceComparisonPage, ServiceDetailPage, ServicesPage, StoresPage, CartPage, FavoritesPage, UserPage.
- **Componentes (8)**, en `fronted/src/components/`: Footer, Header, ui, ComparisonDock, PriceHistory, ProductCard, ServiceCard, ServiceComparisonDock.
- **Otros (18)**, en `fronted/src/`: App.tsx, main.tsx, App.css, index.css, types.ts, vite-env.d.ts, constants (productCatalog, uiText), productComparisonUtils, ServiceComparisonUtils, services/api (api, config, personalization, products-client, products, services, stores), services/utils/productUtils.
- **Configuración:** package.json, vite.config.ts, tsconfig.app.json, eslint.config.js, index.html, .figma/make/site.json.