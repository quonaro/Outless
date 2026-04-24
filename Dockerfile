# syntax=docker/dockerfile:1.7

FROM golang:1.26.2-alpine AS builder
WORKDIR /src

RUN apk add --no-cache ca-certificates tzdata

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/api ./cmd/api
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/hub ./cmd/hub
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/checker ./cmd/checker
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/probe-manager ./cmd/probe-manager
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/token ./cmd/token

FROM alpine:3.22 AS runtime-base
WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata docker-cli && adduser -D -u 10001 appuser && mkdir -p /app/tmp && chown appuser:appuser /app/tmp

COPY --from=builder /out/ /usr/local/bin/

USER appuser

FROM runtime-base AS api
EXPOSE 41220
ENTRYPOINT ["/usr/local/bin/api"]

FROM runtime-base AS hub
ENTRYPOINT ["/usr/local/bin/hub"]

FROM runtime-base AS checker
ENTRYPOINT ["/usr/local/bin/checker"]

FROM runtime-base AS probe-manager
ENTRYPOINT ["/usr/local/bin/probe-manager"]

FROM runtime-base AS token
ENTRYPOINT ["/usr/local/bin/token"]
