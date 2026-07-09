// swift-tools-version: 5.9

import PackageDescription

let package = Package(
    name: "remotty",
    platforms: [
        .macOS(.v14),
    ],
    targets: [
        .executableTarget(
            name: "remotty",
            path: ".",
            exclude: ["Info.plist", "build", "remotty.xcodeproj"],
            sources: [
                "main.swift",
                "AppDelegate.swift",
                "HostManager.swift",
            ],
            swiftSettings: [
                .unsafeFlags(["-O"])
            ]
        ),
    ]
)
