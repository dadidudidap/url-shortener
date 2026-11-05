FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o url-worker ./services/worker

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/url-worker .

CMD ["./url-worker"]
