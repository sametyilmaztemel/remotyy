# Build

Creating a single Go binary from scratch.

```
go build -o remotty-host ./cmd/remotty-host
go build -o remotty-signal ./cmd/remotty-signal
go build -o remotty ./cmd/remotty
```

Cross-compile for ARM:

```
GOOS=linux GOARCH=arm64 go build -o remotty-host-linux-arm64 ./cmd/remotty-host
GOOS=linux GOARCH=arm64 go build -o remotty-signal-linux-arm64 ./cmd/remotty-signal
```

## Dependencies

- Go 1.22+
- Node.js 18+ (for web client)
- libvips (optional, for image processing)

## Web Client

```bash
cd web
npm install
npm run build    # production build
npm run dev      # dev server with HMR
```

## Testing

```bash
make test         # all tests
make test-short   # quick tests only
make lint         # golangci-lint
```
