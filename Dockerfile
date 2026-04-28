# syntax=docker/dockerfile:1.7

FROM golang:1.26.2-alpine AS builder
WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /out/outless ./cmd/outless

FROM scratch AS outless
WORKDIR /app
EXPOSE 41220
COPY --from=builder /out/outless /outless
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
ENTRYPOINT ["/outless"]
