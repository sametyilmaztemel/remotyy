#!/bin/bash
# Build remotty macOS .dmg from source
# Usage: bash scripts/build-dmg.sh [--install]
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$ROOT_DIR/build"
DMG_NAME="remotty.dmg"

echo "📦 Building remotty macOS app..."
echo "Root: $ROOT_DIR"

# 1. Build Go daemon binary
echo "🔨 Building Go daemon..."
cd "$ROOT_DIR"
mkdir -p "$BUILD_DIR"
go build -o "$BUILD_DIR/remottyd" -ldflags="-s -w" ./cmd/remotty

# 2. Build SwiftUI menu bar app
echo "🖥  Building SwiftUI app..."
cd "$ROOT_DIR/remotty-macOS"
swift build -c release --product remotty 2>&1 | tail -3
SWIFT_BINARY=".build/release/remotty"

# 3. Create .app bundle
echo "📁 Creating .app bundle..."
APP_BUNDLE="$BUILD_DIR/remotty.app"
rm -rf "$APP_BUNDLE"
mkdir -p "$APP_BUNDLE/Contents/MacOS"
mkdir -p "$APP_BUNDLE/Contents/Resources"

cp "$SWIFT_BINARY" "$APP_BUNDLE/Contents/MacOS/remotty"
cp "Info.plist" "$APP_BUNDLE/Contents/"
cp "$BUILD_DIR/remottyd" "$APP_BUNDLE/Contents/Resources/remottyd"

# 4. Create DMG
echo "💿 Creating DMG..."
DMG_PATH="$BUILD_DIR/$DMG_NAME"
rm -f "$DMG_PATH"

hdiutil create -volname "remotty" \
  -srcfolder "$APP_BUNDLE" \
  -ov -format UDZO \
  "$DMG_PATH" 2>&1 | tail -1

# 5. Copy to Downloads
if [ "${1:-}" = "--install" ] || [ "${1:-}" = "-i" ]; then
    cp "$DMG_PATH" ~/Downloads/
    echo "✅ Copied to ~/Downloads/$DMG_NAME"
    
    # Install and open
    rm -rf /Applications/remotty.app
    hdiutil attach ~/Downloads/$DMG_NAME 2>&1 | tail -1
    cp -R "/Volumes/remotty/remotty.app" /Applications/
    xattr -dr com.apple.quarantine /Applications/remotty.app
    hdiutil detach "/Volumes/remotty" 2>/dev/null || true
    open /Applications/remotty.app
    echo "✅ Installed and launched"
else
    echo "✅ DMG: $DMG_PATH"
    echo "   Size: $(du -h "$DMG_PATH" | cut -f1)"
    echo ""
    echo "To install: open $DMG_PATH"
    echo "Or run: bash $0 --install"
fi
