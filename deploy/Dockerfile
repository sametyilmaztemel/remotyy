# Stage 1: Build Go binary
FROM golang:1.22-alpine AS builder
RUN apk add --no-cache git
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /remotty ./cmd/remotty

# Stage 2: Build web client
FROM node:20-alpine AS web-builder
WORKDIR /app
COPY web/package*.json ./
RUN npm ci
COPY web/ .
RUN npm run build

# Stage 3: Minimal runtime
FROM alpine:3.19
RUN apk add --no-cache ca-certificates
WORKDIR /app

COPY --from=builder /remotty /usr/local/bin/
COPY --from=web-builder /app/dist /app/web/dist

EXPOSE 9000 3000

ENTRYPOINT ["remotty"]
CMD ["signal", "--dev"]
