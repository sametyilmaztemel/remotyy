// swift-tools-version: 5.9
import PackageDescription

let package = Package(
    name: "remotty-ios",
    defaultLocalization: "en",
    platforms: [
        .iOS(.v17),
    ],
    products: [
        .executable(
            name: "remotty-ios",
            targets: ["remotty-ios"]
        ),
    ],
    dependencies: [
        // WebRTC framework for iOS
        .package(url: "https://github.com/stasel/WebRTC", branch: "latest"),
    ],
    targets: [
        .executableTarget(
            name: "remotty-ios",
            dependencies: [
                .product(name: "WebRTC", package: "WebRTC"),
            ],
            path: ".",
            exclude: [
                "Package.swift",
                "Info.plist",
                "ExportOptions.plist",
                "Assets.xcassets",
                "Tests",
                ".build",
            ],
            sources: [
                "remottyApp.swift",
                "ContentView.swift",
                "ConnectionView.swift",
                "TerminalView.swift",
                "WebRTCService.swift",
                "TouchHandler.swift",
                "ScreenView.swift",
            ],
            resources: [
                .process("Assets.xcassets"),
            ],
            swiftSettings: [
                .define("IOS"),
                .unsafeFlags(["-O"]),
            ],
            linkerSettings: [
                .linkedFramework("SwiftUI"),
                .linkedFramework("UIKit"),
                .linkedFramework("AVFoundation"),
            ]
        ),
        .testTarget(
            name: "remotty-ios-tests",
            dependencies: ["remotty-ios"],
            path: "Tests",
            sources: [
                "WebRTCServiceTests.swift",
                "TouchHandlerTests.swift",
                "TerminalViewTests.swift",
            ]
        ),
    ]
)
