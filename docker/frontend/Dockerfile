# syntax=docker/dockerfile:1
# Build context: raíz del repo  (docker build -f docker/frontend/Dockerfile .)
# Exclusiones del contexto: docker/frontend/Dockerfile.dockerignore
FROM node:24-alpine AS builder

WORKDIR /app

# Misma versión de pnpm que declara package.json (packageManager).
RUN npm install -g pnpm@11.24.0

COPY fronted/package.json fronted/pnpm-lock.yaml fronted/pnpm-workspace.yaml ./
RUN pnpm install --frozen-lockfile

COPY fronted/ ./

# URLs base que el NAVEGADOR usa para llegar a las APIs.
# Vacío = mismo origen: nginx del frontend reenvía cada ruta (/products, /auth, ...) a su API.
# Nunca usar DNS interno de Kubernetes ni localhost aquí.
ARG VITE_PRODUCT_API_URL=""
ARG VITE_STORE_API_URL=""
ARG VITE_SERVICE_API_URL=""
ARG VITE_FAVORITES_API_URL=""
ARG VITE_CART_API_URL=""
ARG VITE_COMPARISON_API_URL=""
ARG VITE_AUTH_API_URL=""
ARG VITE_SEARCH_API_URL=""
ENV VITE_PRODUCT_API_URL=$VITE_PRODUCT_API_URL \
    VITE_STORE_API_URL=$VITE_STORE_API_URL \
    VITE_SERVICE_API_URL=$VITE_SERVICE_API_URL \
    VITE_FAVORITES_API_URL=$VITE_FAVORITES_API_URL \
    VITE_CART_API_URL=$VITE_CART_API_URL \
    VITE_COMPARISON_API_URL=$VITE_COMPARISON_API_URL \
    VITE_AUTH_API_URL=$VITE_AUTH_API_URL \
    VITE_SEARCH_API_URL=$VITE_SEARCH_API_URL

RUN pnpm build

# Imagen final: nginx sin root (UID 101), escucha en 8080.
FROM nginxinc/nginx-unprivileged:1.29-alpine

# El entrypoint de nginx genera conf.d/default.conf desde la plantilla con
# envsubst al arrancar; el filtro evita tocar variables de nginx ($host, $uri).
# Destinos por defecto: microservicios (mismos nombres que k8s y compose).
# En el modo consolidado (UCT) el Deployment los apunta todos a backend:8080.
ENV NGINX_ENVSUBST_FILTER="_UPSTREAM$"     PRODUCT_UPSTREAM=product-api:8081     STORE_UPSTREAM=store-api:8082     SERVICE_UPSTREAM=service-api:8083     AUTH_UPSTREAM=auth-api:8088     FAVORITES_UPSTREAM=favorites-api:8089     CART_UPSTREAM=cart-api:8090     SEARCH_UPSTREAM=search-api:8086     COMPARISON_UPSTREAM=comparison-api:8087     REVIEW_UPSTREAM=review-api:8091     WATCHLIST_UPSTREAM=watchlist-api:8092

COPY docker/frontend/default.conf.template /etc/nginx/templates/default.conf.template
COPY docker/frontend/proxy.conf /etc/nginx/soloservis-proxy.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 8080
