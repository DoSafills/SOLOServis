# syntax=docker/dockerfile:1
# Build context: backend/  (docker build -f docker/store-api/Dockerfile backend)
FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Binario estático y reproducible (sin CGO, sin rutas locales embebidas).
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o store-api ./cmd/store-api

FROM alpine:3.22

# UID/GID numéricos: Kubernetes puede verificar runAsNonRoot.
RUN addgroup -S -g 10001 appgroup && adduser -S -u 10001 -G appgroup appuser

WORKDIR /app

COPY --from=builder --chown=10001:10001 /app/store-api ./store-api

USER 10001:10001

EXPOSE 8082

CMD ["./store-api"]
