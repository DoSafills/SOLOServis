FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o favorites-api ./cmd/favorites-api
FROM alpine:3.22
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/favorites-api ./favorites-api
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8089
CMD ["./favorites-api"]