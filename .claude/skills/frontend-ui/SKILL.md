---
name: frontend-ui
description: Use when changing anything under fronted/ (React, TypeScript, Tailwind, Vite). Covers the pnpm workflow, port 8443, VITE_* API configuration, problem+json handling and the pre-delivery check.
---

# Frontend — `fronted/`

La carpeta se llama **`fronted`**, sin la "n". Cualquier ruta
`frontend/...` está mal y no coincide con nada. Reglas generales en el
`AGENTS.md` de la raíz (§9) y en `fronted/AGENTS.md`.

## Stack real

| Pieza | Versión |
| --- | --- |
| React | 19 |
| TypeScript | 5.9 |
| Vite | 8 |
| Tailwind CSS | 4 (`@tailwindcss/vite`) |
| Gráficos | `recharts` |
| Formato | `oxfmt` |
| Lint | ESLint 10 |
| Gestor | `pnpm` (nunca `npm`) |

## Mapa

| Ruta | Qué hay |
| --- | --- |
| `src/main.tsx` | Entrada; monta `App.tsx` en `#root` |
| `src/App.tsx` | Componente principal |
| `src/pages/` | Pantallas por ruta |
| `src/components/` | Componentes reutilizables |
| `src/services/api/config.ts` | URLs desde `VITE_*_API_URL` |
| `src/hooks/` | Hooks de datos |
| `src/index.css` | Tailwind v4 y tema global |

## Reglas

1. **URLs de API:** siempre por `src/services/api/config.ts`. En Kubernetes las
   `VITE_*` quedan vacías y todo pasa por el mismo origen (nginx enruta).
2. **Errores del backend:** llegan como `problem+json`. Decide con `code`,
   muestra `detail`. No muestres el JSON crudo.
3. **Estilos:** clases utilitarias de Tailwind v4 en JSX. Lo global va en
   `src/index.css`.
4. **No inventes datos.** Si una API no existe, la pantalla no se hace; se
   reporta.
5. **No hay tests de frontend.** No los inventes ni añadas un runner sin
   acordarlo: la verificación es `pnpm check`.

## Comandos

```bash
cd fronted
pnpm dev                 # servidor en el puerto 8443
pnpm check               # formato + lint + build + audit
pnpm format              # oxfmt, para arreglar formato
pnpm lint                # ESLint
pnpm build               # sólo el build
```

`pnpm check` falla hoy en `format:check` por archivos con CRLF (defecto
conocido, ver `AGENTS.md` §13). Si lo ves, dilo: no es tu cambio.

## Antes de abrir el PR

- [ ] `pnpm build` pasa.
- [ ] `pnpm lint` pasa.
- [ ] `pnpm format:check` — si falla por CRLF, se reporta tal cual.
- [ ] La pantalla usa las URLs de `config.ts`, no `localhost` a mano.
- [ ] Los estados de error y de carga están cubiertos.

## Anti-patrones

- `npm` en vez de `pnpm`.
- Escribir `frontend/` en rutas o scripts.
- Hardcodear `http://localhost:808x` en un componente.
- Añadir TanStack Query o Prisma: no están en el proyecto.
