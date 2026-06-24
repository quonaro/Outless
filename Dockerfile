# syntax=docker/dockerfile:1

# Stage 1: Build frontend
FROM node:22-alpine AS frontend
WORKDIR /frontend
RUN corepack enable && corepack prepare pnpm@9.15.0 --activate
COPY frontend/package.json frontend/pnpm-lock.yaml ./
RUN pnpm install --frozen-lockfile
COPY frontend/ .
RUN pnpm generate

# Stage 2: Build Go binary
FROM golang:1.26-alpine AS builder
ENV GOPROXY=https://proxy.golang.org,direct
RUN apk add --no-cache ca-certificates git
WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /frontend/.output/public /build/web/dist
RUN CGO_ENABLED=0 GOOS=linux GOARCH=${TARGETARCH} GOARM=${TARGETVARIANT#v} \
    go build -trimpath -tags "with_reality_server with_utls" \
    -ldflags="-s -w" -o outless ./cmd/outless

# Stage 3: Minimal scratch image
FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /build/outless /outless
EXPOSE 41220
ENTRYPOINT ["/outless"]
CMD ["server", "run"]
