# syntax=docker/dockerfile:1
# Build context: raíz del repo  (docker build -f docker/frontend/Dockerfile .)
# Exclusiones del contexto: docker/frontend/Dockerfile.dockerignore
FROM node:24-alpine AS builder

WORKDIR /app

# Misma versión de pnpm que declara package.json (packageManager).
RUN npm install -g pnpm@11.24.0

COPY fronted/package.json fronted/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile

COPY fronted/ ./

# URLs base que el NAVEGADOR usa para llegar a las APIs.
# Vacío = mismo origen: nginx del frontend reenvía /products y /stores a cada API.
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

COPY docker/frontend/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/dist /usr/share/nginx/html

EXPOSE 8080
