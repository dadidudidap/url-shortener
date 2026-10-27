FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY . .

WORKDIR /app/worker
RUN CGO_ENABLED=0 GOOS=linux go build -o url-worker .

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/worker/url-worker .

CMD ["./url-worker"]