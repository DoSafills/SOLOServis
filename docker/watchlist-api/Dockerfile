FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o watchlist-api ./cmd/watchlist-api
FROM alpine:3.22
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/watchlist-api ./watchlist-api
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8092
CMD ["./watchlist-api"]