# syntax=docker/dockerfile:1.7

FROM golang:1.26.2-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/outless ./cmd/outless

FROM alpine:3.22 AS runtime-base
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata docker-cli curl && adduser -D -u 10001 appuser && mkdir -p /app/tmp /app/logs /var/lib/outless && chown appuser:appuser /app/tmp /app/logs /var/lib/outless

COPY --from=builder /out/ /usr/local/bin/
COPY docker-entrypoint.sh /usr/local/bin/docker-entrypoint.sh
RUN chmod +x /usr/local/bin/docker-entrypoint.sh

USER appuser

FROM runtime-base AS outless
EXPOSE 41220
ENTRYPOINT ["/usr/local/bin/docker-entrypoint.sh"]
