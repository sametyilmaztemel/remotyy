# Build stage
FROM golang:1.24-alpine AS builder

RUN apk add --no-cache git ca-certificates tzdata

WORKDIR /build

# Cache deps
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY . .

# Build binary with version injection
ARG VERSION=dev
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags="-s -w -X github.com/sametyilmaztemel/remotty/internal/config.Version=${VERSION}" \
    -o remotty ./cmd/remotty

# Web client build
FROM node:22-alpine AS web-builder

WORKDIR /build/web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Runtime stage
FROM alpine:3.21

RUN apk add --no-cache ca-certificates tzdata bash

# Create non-root user
RUN addgroup -S remotty && adduser -S -G remotty remotty

# Binary
COPY --from=builder /build/remotty /usr/local/bin/remotty

# Web UI (optional)
COPY --from=web-builder /build/web/dist /opt/remotty/web

# Config directory
RUN mkdir -p /etc/remotty /var/lib/remotty /var/log/remotty && \
    chown -R remotty:remotty /etc/remotty /var/lib/remotty /var/log/remotty

USER remotty

EXPOSE 9000

ENTRYPOINT ["remotty"]
CMD ["signal"]
