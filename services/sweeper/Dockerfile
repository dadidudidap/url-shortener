FROM golang:1.27-alpine AS builder

WORKDIR /app

COPY . .

RUN CGO_ENABLED=0 GOOS=linux go build -o url-sweeper ./services/sweeper

FROM alpine:latest

WORKDIR /app

COPY --from=builder /app/url-sweeper .

CMD ["./url-sweeper"]
