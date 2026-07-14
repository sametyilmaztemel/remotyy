// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "remotty",
    platforms: [
        .macOS(.v14),
        .iOS(.v17),
    ],
    products: [
        .executable(name: "remotty-macOS", targets: ["remotty-macOS"]),
        .library(name: "remotty-ios", targets: ["remotty-ios"]),
    ],
    dependencies: [
        // WebRTC SDK for iOS/macOS
        .package(url: "https://github.com/webrtc-sdk/Specs.git", branch: "main"),
    ],
    targets: [
        .executableTarget(
            name: "remotty-macOS",
            dependencies: [],
            path: "remotty-macOS",
            sources: [
                "remottyMenuBarApp.swift",
                "MenuBarView.swift",
                "SettingsView.swift",
            ]
        ),
        .target(
            name: "remotty-ios",
            dependencies: [],
            path: "ios/remotty",
            sources: [
                "remottyApp.swift",
                "ContentView.swift",
                "ConnectionView.swift",
                "TerminalView.swift",
            ]
        ),
    ]
)
