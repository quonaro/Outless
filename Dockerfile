# syntax=docker/dockerfile:1.7

FROM golang:1.26.2-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/outless ./cmd/outless

# Download Xray binary for embedded mode
FROM alpine:3.22 AS xray-downloader
RUN apk add --no-cache curl && \
    curl -L -o /xray "https://github.com/XTLS/Xray-core/releases/download/v1.8.24/Xray-linux-64.zip" && \
    unzip -p /xray xray > /usr/local/bin/xray && \
    chmod +x /usr/local/bin/xray && \
    rm /xray

FROM alpine:3.22 AS runtime-base
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata docker-cli && adduser -D -u 10001 appuser && mkdir -p /app/tmp /var/lib/outless && chown appuser:appuser /app/tmp /var/lib/outless

COPY --from=builder /out/ /usr/local/bin/
COPY --from=xray-downloader /usr/local/bin/xray /usr/local/bin/xray

USER appuser

FROM runtime-base AS outless
EXPOSE 41220 443
ENTRYPOINT ["/usr/local/bin/outless"]
