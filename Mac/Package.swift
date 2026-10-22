// swift-tools-version:5.9
import PackageDescription

let package = Package(
    name: "MDViewer",
    platforms: [
        .macOS(.v13)
    ],
    products: [
        .executable(name: "MDViewer", targets: ["MDViewer"])
    ],
    targets: [
        .executableTarget(
            name: "MDViewer",
            path: "Sources/MDViewer"
        )
    ],
    swiftLanguageVersions: [.v5]
)
