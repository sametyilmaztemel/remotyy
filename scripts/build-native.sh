#!/bin/bash
# Build script for remotty native apps
set -e

echo "╔════════════════════════════════════════════╗"
echo "║        remotty — Native App Builder        ║"
echo "╚════════════════════════════════════════════╝"

build_tauri() {
    echo ""
    echo "📦 Building Tauri desktop app..."
    cd src-tauri
    
    if ! command -v cargo &> /dev/null; then
        echo "❌ Rust/Cargo not found. Install from https://rustup.rs"
        exit 1
    fi
    
    cargo build --release 2>&1 | tail -5
    echo "✅ Tauri build complete: target/release/remotty"
    cd ..
}

build_ios() {
    echo ""
    echo "📱 Building iOS app..."
    
    if ! command -v xcodebuild &> /dev/null; then
        echo "⚠️  Xcode not found. Skipping iOS build."
        return
    fi
    
    cd ios
    xcodebuild -project remotty.xcodeproj \
        -scheme remotty \
        -configuration Release \
        -sdk iphoneos \
        -derivedDataPath build \
        CODE_SIGN_IDENTITY="" \
        CODE_SIGNING_REQUIRED=NO 2>&1 | tail -5
    echo "✅ iOS build complete"
    cd ..
}

build_macos() {
    echo ""
    echo "🖥  Building macOS native app..."
    
    cd remotty-macOS
    swift build -c release 2>&1 | tail -5
    echo "✅ macOS build complete: .build/release/remotty-macOS"
    cd ..
}

case "${1:-all}" in
    tauri)
        build_tauri
        ;;
    ios)
        build_ios
        ;;
    macos)
        build_macos
        ;;
    all)
        build_tauri
        build_macos
        echo ""
        echo "✨ All builds complete!"
        ;;
    *)
        echo "Usage: $0 {tauri|ios|macos|all}"
        exit 1
        ;;
esac
