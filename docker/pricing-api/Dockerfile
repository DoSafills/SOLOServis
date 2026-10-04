FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o pricing-api ./cmd/pricing-api
FROM alpine:3.22
RUN addgroup -S appgroup && adduser -S appuser -G appgroup
WORKDIR /app
COPY --from=builder /app/pricing-api ./pricing-api
RUN chown -R appuser:appgroup /app
USER appuser
EXPOSE 8085
CMD ["./pricing-api"]