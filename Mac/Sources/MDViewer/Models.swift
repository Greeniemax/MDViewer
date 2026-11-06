import Foundation
import Darwin

struct DocumentTab: Identifiable, Equatable {
    let id: UUID
    var path: URL?
    var title: String
    var text: String
    var savedText: String

    var isDirty: Bool { text != savedText }

    static func untitled() -> DocumentTab {
        DocumentTab(id: UUID(), path: nil, title: "Untitled", text: "", savedText: "")
    }
}

enum PaneMode: String {
    case editor
    case viewer
}

struct VoiceOption: Identifiable, Hashable, Codable {
    enum Kind: String, Codable {
        case piper
        case system
        case paradee
        case kokoro
    }

    let id: String
    let label: String
    let kind: Kind
    let systemName: String?
    let piperModel: String?
    /// Neural (Paradee/Kokoro) voice key, e.g. `af_heart`.
    let neuralVoice: String?
    /// 0 = bundled Piper, 1 = compact system voice, 2 = enhanced, 3 = premium.
    let quality: Int

    init(
        id: String,
        label: String,
        kind: Kind,
        systemName: String?,
        piperModel: String?,
        neuralVoice: String? = nil,
        quality: Int
    ) {
        self.id = id
        self.label = label
        self.kind = kind
        self.systemName = systemName
        self.piperModel = piperModel
        self.neuralVoice = neuralVoice
        self.quality = quality
    }
}

struct SpeechJob {
    let index: Int
    let text: String
}

struct EditorSelection: Equatable {
    let range: NSRange
    let token: Int
}

/// Shared lookup so the speech engine and the voice catalogue agree on where
/// the bundled Piper engine and its voices live.
enum PiperLocator {
    static func piperDir() -> URL? {
        let fm = FileManager.default

        if let env = ProcessInfo.processInfo.environment["MDVIEWER_PIPER_DIR"], !env.isEmpty {
            let url = URL(fileURLWithPath: env)
            if fm.fileExists(atPath: url.path) { return url }
        }

        if let res = Bundle.main.resourceURL {
            let url = res.appendingPathComponent("piper", isDirectory: true)
            if fm.fileExists(atPath: url.path) { return url }
        }

        // Development fallback: <repo>/Mac/Sources/MDViewer/ThisFile.swift
        let sourceFile = URL(fileURLWithPath: #filePath)
        let macRoot = sourceFile
            .deletingLastPathComponent() // MDViewer
            .deletingLastPathComponent() // Sources
            .deletingLastPathComponent() // Mac
        let repoResources = macRoot
            .deletingLastPathComponent() // repo root
            .appendingPathComponent("resources/piper", isDirectory: true)

        let arch = ProcessInfo.processInfo.machineArchitecture
        let engineDir = repoResources
            .appendingPathComponent("engine", isDirectory: true)
            .appendingPathComponent(arch, isDirectory: true)
            .appendingPathComponent("piper", isDirectory: true)
        if fm.fileExists(atPath: engineDir.path) {
            // Build a transient layout: engine folder is the piper dir but
            // voices live one level up in the repo's resources folder.
            // The catalogue handles this via voicesDirOverride.
            return engineDir
        }

        if fm.fileExists(atPath: repoResources.path) { return repoResources }
        return nil
    }

    static func binary() -> URL? {
        guard let dir = piperDir() else { return nil }
        let candidates = [
            dir.appendingPathComponent("piper"),
            dir.appendingPathComponent("piper/piper")
        ]
        for c in candidates where FileManager.default.isExecutableFile(atPath: c.path) {
            return c
        }
        return nil
    }

    static func voicesDir() -> URL? {
        guard let dir = piperDir() else { return nil }
        let direct = dir.appendingPathComponent("voices", isDirectory: true)
        if FileManager.default.fileExists(atPath: direct.path) { return direct }

        // Development layout: <repo>/resources/piper/engine/<arch>/piper
        // voices are at <repo>/resources/piper/voices.
        let up = dir
            .deletingLastPathComponent() // <arch>
            .deletingLastPathComponent() // engine
            .deletingLastPathComponent() // piper
            .appendingPathComponent("voices", isDirectory: true)
        if FileManager.default.fileExists(atPath: up.path) { return up }
        return nil
    }
}

extension ProcessInfo {
    var machineArchitecture: String {
        var sysinfo = utsname()
        uname(&sysinfo)
        let machine = withUnsafePointer(to: &sysinfo.machine) {
            $0.withMemoryRebound(to: CChar.self, capacity: 256) {
                String(cString: $0)
            }
        }
        if machine == "arm64" { return "darwin-arm64" }
        return "darwin-amd64"
    }
}

/// Where the neural TTS helper (Paradee / Kokoro) and its assets live.
enum NeuralLocator {
    static func neuralDir() -> URL? {
        let fm = FileManager.default

        if let env = ProcessInfo.processInfo.environment["MDVIEWER_NEURAL_DIR"],
           !env.isEmpty,
           fm.fileExists(atPath: env) {
            return URL(fileURLWithPath: env)
        }

        if let res = Bundle.main.resourceURL {
            let url = res.appendingPathComponent("neural", isDirectory: true)
            if fm.fileExists(atPath: url.path) { return url }
        }

        // Development fallback: <repo>/Mac/Sources/MDViewer/ThisFile.swift
        let sourceFile = URL(fileURLWithPath: #filePath)
        let macRoot = sourceFile
            .deletingLastPathComponent() // MDViewer
            .deletingLastPathComponent() // Sources
            .deletingLastPathComponent() // Mac
        let repoNeural = macRoot
            .deletingLastPathComponent() // repo root
            .appendingPathComponent("resources/neural", isDirectory: true)
        if fm.fileExists(atPath: repoNeural.path) { return repoNeural }
        return nil
    }

    static func binary() -> URL? {
        guard let dir = neuralDir() else { return nil }
        let bin = dir.appendingPathComponent("neural-tts")
        guard FileManager.default.isExecutableFile(atPath: bin.path) else { return nil }
        return bin
    }

    static func ortLibrary() -> URL? {
        guard let dir = neuralDir() else { return nil }
        for name in ["libonnxruntime.dylib", "libonnxruntime.1.20.0.dylib"] {
            let url = dir.appendingPathComponent(name)
            if FileManager.default.fileExists(atPath: url.path) { return url }
        }
        return nil
    }

    static func espeakLibrary() -> URL? {
        guard let dir = neuralDir() else { return nil }
        for name in ["libespeak-ng.1.dylib", "libespeak-ng.dylib"] {
            let url = dir.appendingPathComponent(name)
            if FileManager.default.fileExists(atPath: url.path) { return url }
        }
        return nil
    }

    static func espeakData() -> URL? {
        guard let dir = neuralDir() else { return nil }
        let data = dir.appendingPathComponent("espeak-ng-data", isDirectory: true)
        if FileManager.default.fileExists(atPath: data.path) { return dir }
        // The packaged app links the Piper bundle's copy instead of duplicating
        // the 18 MB data set.
        for base in [dir, dir.deletingLastPathComponent().appendingPathComponent("piper", isDirectory: true)] {
            if FileManager.default.fileExists(
                atPath: base.appendingPathComponent("espeak-ng-data").path
            ) {
                return base
            }
        }
        return nil
    }

    static func model(family: String) -> URL? {
        guard let dir = neuralDir() else { return nil }
        let name = family == "kokoro" ? "kokoro-v1.0.int8.onnx" : "paradee_int8.onnx"
        let url = dir.appendingPathComponent(name)
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }

    static func config(family: String) -> URL? {
        guard let dir = neuralDir() else { return nil }
        let name = family == "kokoro" ? "kokoro-config.json" : "paradee-config.json"
        let url = dir.appendingPathComponent(name)
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }

    static func isReady(family: String) -> Bool {
        binary() != nil
            && ortLibrary() != nil
            && espeakLibrary() != nil
            && espeakData() != nil
            && model(family: family) != nil
            && config(family: family) != nil
    }
}
