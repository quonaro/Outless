# syntax=docker/dockerfile:1.7

FROM golang:1.26.2-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata git

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/hub ./cmd/hub
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/checker ./cmd/checker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/unified ./cmd/unified

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

FROM runtime-base AS api
EXPOSE 41220
ENTRYPOINT ["/usr/local/bin/api"]

FROM runtime-base AS hub
ENTRYPOINT ["/usr/local/bin/hub"]

FROM runtime-base AS checker
ENTRYPOINT ["/usr/local/bin/checker"]

FROM runtime-base AS unified
EXPOSE 41220 443
ENTRYPOINT ["/usr/local/bin/unified"]
