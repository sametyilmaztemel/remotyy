#!/bin/bash
# Cross-platform install script for remotty
# Usage: curl -fsSL https://remotty.dev/install.sh | bash
set -euo pipefail

REPO="remotty/remotty"
VERSION="${1:-latest}"
BIN_DIR="${BIN_DIR:-/usr/local/bin}"

echo "⎈ Installing remotty..."

# Detect OS and arch
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"
case "$ARCH" in
    x86_64|amd64) ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *) echo "❌ Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Latest release tag
if [ "$VERSION" = "latest" ]; then
    VERSION=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | cut -d'"' -f4)
fi

# Download
URL="https://github.com/$REPO/releases/download/$VERSION/remotty-${OS}-${ARCH}"
echo "⬇️  Downloading $URL..."
curl -fsSL "$URL" -o "$BIN_DIR/remotty"
chmod +x "$BIN_DIR/remotty"

echo "✅ remotty $VERSION installed to $BIN_DIR/remotty"
echo ""
echo "Quick start:"
echo "  remotty signal --dev     # Start signaling server"
echo "  remotty host             # Start host daemon"
echo "  remotty connect          # List and connect to hosts"
