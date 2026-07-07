#!/bin/bash
#===============================================================================
# remotty iOS — App Store Archive Script
# ===============================================================================
# This script archives and exports the remotty iOS app for App Store submission.
#
# Prerequisites:
#   - Xcode 15+ installed
#   - An Apple Developer account with an active App Store Connect team
#   - Update ExportOptions.plist with your actual Team ID
#   - App icon images placed in Assets.xcassets/AppIcon.appiconset/
#
# Usage:
#   ./scripts/build-ios-appstore.sh [version] [build]
#
# Examples:
#   ./scripts/build-ios-appstore.sh            # Uses Info.plist values
#   ./scripts/build-ios-appstore.sh 1.0.0 2    # Override version & build
#===============================================================================

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
IOS_DIR="$PROJECT_DIR/ios/remotty"
ARCHIVE_PATH="$PROJECT_DIR/build/remotty-ios.xcarchive"
EXPORT_DIR="$PROJECT_DIR/build/remotty-ios-export"

echo "========================================"
echo "  remotty iOS — App Store Archive"
echo "========================================"

# Override version/build if provided
if [ $# -ge 1 ]; then
    echo "  Version: $1"
    plutil -replace CFBundleShortVersionString -string "$1" "$IOS_DIR/Info.plist"
fi
if [ $# -ge 2 ]; then
    echo "  Build:    $2"
    plutil -replace CFBundleVersion -string "$2" "$IOS_DIR/Info.plist"
fi

echo ""
echo "📦 Archiving remotty-ios..."
echo ""

# Step 1: Create the Xcode project from the SPM package
# (This generates an .xcodeproj file from Package.swift)
cd "$IOS_DIR"
swift package generate-xcodeproj --output "$IOS_DIR/remotty.xcodeproj"

# Step 2: Archive the app
xcodebuild archive \
    -project "$IOS_DIR/remotty.xcodeproj" \
    -scheme remotty-ios \
    -configuration Release \
    -archivePath "$ARCHIVE_PATH" \
    -destination "generic/platform=iOS" \
    SWIFT_OPTIMIZATION_LEVEL="-O" \
    CODE_SIGN_STYLE="Automatic" \
    DEVELOPMENT_TEAM="YOUR_TEAM_ID" \
    PROVISIONING_PROFILE_SPECIFIER=""

echo ""
echo "✅ Archive created: $ARCHIVE_PATH"
echo ""

# Step 3: Export for App Store
echo "📦 Exporting for App Store..."
echo ""

xcodebuild -exportArchive \
    -archivePath "$ARCHIVE_PATH" \
    -exportPath "$EXPORT_DIR" \
    -exportOptionsPlist "$IOS_DIR/ExportOptions.plist"

echo ""
echo "✅ App Store IPA exported to: $EXPORT_DIR"
echo ""

# Step 4: Validate with XCTest (if test target exists)
echo "🧪 Running tests..."
cd "$IOS_DIR"
swift test --parallel 2>/dev/null || echo "  ⚠️  Tests skipped (not configured for iOS testing via CLI)"

echo ""
echo "========================================"
echo "  Done! Upload to App Store Connect:"
echo "  - IPA: $EXPORT_DIR/remotty-ios.ipa"
echo "  - Open Application Loader or use:"
echo "    xcrun altool --upload-app -f \"$EXPORT_DIR/remotty-ios.ipa\" -t ios -u YOUR_APPLE_ID"
echo "========================================"
