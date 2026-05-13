# remotty Makefile
.PHONY: all build build-all build-cli build-web \
        build-linux-arm64 build-linux-amd64 build-darwin-arm64 build-darwin-amd64 \
        build-macos-app build-dmg xcode-project build-tauri \
        release clean test lint dev

# ─── Config ──────────────────────────────────────────────────
BIN_DIR   := bin
VERSION   := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT    := $(shell git rev-parse --short HEAD 2>/dev/null || echo "unknown")
DATE      := $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
GO_LDFLAGS := -ldflags "-s -w -X github.com/remotty/remotty/internal/config.Version=$(VERSION)"

# ─── Go Build (all-in-one binary) ─────────────────────────────
all: build

build: build-cli

build-cli:
	@echo "🔨 Building remotty $(VERSION)..."
	@mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build $(GO_LDFLAGS) -o $(BIN_DIR)/remotty ./cmd/remotty
	@echo "✅ Built: $(BIN_DIR)/remotty"

# ─── Cross-compile ────────────────────────────────────────────
build-linux-arm64:
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build $(GO_LDFLAGS) -o $(BIN_DIR)/remotty-linux-arm64 ./cmd/remotty

build-linux-amd64:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build $(GO_LDFLAGS) -o $(BIN_DIR)/remotty-linux-amd64 ./cmd/remotty

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 CGO_ENABLED=0 go build $(GO_LDFLAGS) -o $(BIN_DIR)/remotty-darwin-arm64 ./cmd/remotty

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 CGO_ENABLED=0 go build $(GO_LDFLAGS) -o $(BIN_DIR)/remotty-darwin-amd64 ./cmd/remotty

build-all-platforms: build-linux-arm64 build-linux-amd64 build-darwin-arm64 build-darwin-amd64

# ─── Web Client ───────────────────────────────────────────────
build-web:
	@echo "🌐 Building web client..."
	cd web && npm ci && npm run build
	@echo "✅ Web client built: web/dist/"

# ─── macOS Native App ─────────────────────────────────────────
xcode-project:
	@echo "🛠  Generating Xcode project..."
	python3 scripts/gen-xcode-project.py

build-macos-app: xcode-project
	@echo "🖥  Building macOS .app..."
	cd remotty-macOS && xcodebuild -project remotty.xcodeproj \
		-scheme remotty -configuration Release build \
		-derivedDataPath build -quiet 2>/dev/null || \
		xcodebuild -project remotty.xcodeproj \
		-scheme remotty -configuration Release build \
		-derivedDataPath build 2>&1 | tail -5

build-dmg: build-macos-app
	@echo "📦 Packaging macOS .dmg..."
	bash scripts/build-dmg.sh

# ─── Tauri Desktop ────────────────────────────────────────────
build-tauri:
	@echo "🦀 Building Tauri desktop app..."
	cd src-tauri && cargo build --release 2>&1 | tail -5

# ─── Release Packaging ────────────────────────────────────────
release: build build-web build-all-platforms
	@echo "📦 Packaging release artifacts..."
	@mkdir -p dist
	cp $(BIN_DIR)/remotty dist/
	cd web && tar czf ../dist/remotty-web.tar.gz dist/
	cd dist && for f in remotty-*; do \
		[ "$$f" = "remotty-web.tar.gz" ] && continue; \
		gzip -c "$$f" > "$$f.tar.gz"; \
	done
	@echo "✅ Release artifacts in dist/"
	@ls -la dist/

# ─── Development ──────────────────────────────────────────────
dev:
	@echo "Starting remotty signal server on :9000..."
	go run ./cmd/remotty signal --dev

dev-host:
	@echo "Starting remotty host..."
	go run ./cmd/remotty host --signal ws://localhost:9000

dev-web:
	cd web && npm run dev

# ─── Quality ──────────────────────────────────────────────────
test:
	@echo "Running tests..."
	go test ./... -v -count=1 -timeout=60s

test-short:
	go test ./... -short -race -count=1 -timeout=30s

lint:
	golangci-lint run ./...

vet:
	go vet ./...

# ─── Utility ──────────────────────────────────────────────────
clean:
	rm -rf $(BIN_DIR) dist/
	go clean ./...
	@echo "🧹 Cleaned"

.PHONY: help
help:
	@echo "remotty — remote terminal & screen access"
	@echo ""
	@echo "Build:"
	@echo "  make build              Build CLI binary"
	@echo "  make build-all-platforms Cross-compile for all platforms"
	@echo "  make build-web          Build web client"
	@echo "  make build-macos-app    Build macOS native app (requires Xcode)"
	@echo "  make build-dmg          Build macOS .dmg (requires Xcode)"
	@echo "  make build-tauri        Build Tauri desktop app (requires Rust)"
	@echo ""
	@echo "Development:"
	@echo "  make dev       Start signaling server"
	@echo "  make dev-host  Start host daemon"
	@echo "  make dev-web   Start web dev server"
	@echo ""
	@echo "Quality:"
	@echo "  make test      Run all tests"
	@echo "  make lint      Run linter"
	@echo ""
	@echo "Release:"
	@echo "  make release   Build all release artifacts"
	@echo "  make clean     Clean build artifacts"
