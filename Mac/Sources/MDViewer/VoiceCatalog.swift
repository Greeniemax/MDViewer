import AVFoundation
import Foundation

enum VoiceCatalog {
    static var cacheURL: URL {
        let dir = FileManager.default
            .urls(for: .cachesDirectory, in: .userDomainMask)
            .first!
            .appendingPathComponent("MDViewer", isDirectory: true)
        return dir.appendingPathComponent("voices.json")
    }

    static func loadCached() -> [VoiceOption]? {
        guard let data = try? Data(contentsOf: cacheURL) else { return nil }
        return try? JSONDecoder().decode([VoiceOption].self, from: data)
    }

    static func saveCache(_ voices: [VoiceOption]) {
        guard !voices.isEmpty else { return }
        let url = cacheURL
        do {
            try FileManager.default.createDirectory(
                at: url.deletingLastPathComponent(),
                withIntermediateDirectories: true
            )
            let data = try JSONEncoder().encode(voices)
            try data.write(to: url, options: .atomic)
        } catch {
            // A cache miss is harmless; the next launch simply scans again.
        }
    }

    static func refresh() -> [VoiceOption] {
        // macOS system voices first; the two bundled models are a fallback at
        // the bottom of the picker.
        systemVoices() + piperVoices()
    }

    static func defaultVoice(from voices: [VoiceOption]) -> VoiceOption? {
        // Prefer an actually natural macOS voice (Enhanced/Premium download).
        if let natural = voices
            .filter({ $0.kind == .system && $0.quality >= 2 })
            .sorted(by: { $0.quality > $1.quality })
            .first {
            return natural
        }
        // No downloaded natural voice: the bundled Piper voice sounds far
        // better than the compact system voices, so use it as the default.
        if let amy = voices.first(where: { $0.piperModel == "en_US-amy-medium" }) {
            return amy
        }
        if let samantha = voices.first(where: {
            $0.kind == .system && $0.systemName?.contains("Samantha") == true
        }) {
            return samantha
        }
        return voices.first
    }

    // MARK: - Piper

    static func piperVoices() -> [VoiceOption] {
        let fm = FileManager.default
        guard PiperLocator.binary() != nil,
              let dir = PiperLocator.voicesDir(),
              let files = try? fm.contentsOfDirectory(at: dir, includingPropertiesForKeys: nil)
        else { return [] }

        var result: [VoiceOption] = []
        for file in files where file.pathExtension == "onnx" {
            let modelKey = file.deletingPathExtension().lastPathComponent
            let jsonURL = file.appendingPathExtension("json")
            var dataset = modelKey
            var locale = ""

            if let data = try? Data(contentsOf: jsonURL),
               let obj = try? JSONSerialization.jsonObject(with: data) as? [String: Any] {
                if let d = obj["dataset"] as? String, !d.isEmpty { dataset = d }
                if let language = obj["language"] as? [String: Any],
                   let code = language["code"] as? String {
                    locale = code
                }
            }

            if let first = dataset.first {
                dataset = first.uppercased() + String(dataset.dropFirst())
            }
            let label = locale.isEmpty
                ? "\(dataset) (Natural)"
                : "\(dataset) (Natural) — \(locale)"

            result.append(VoiceOption(
                id: "piper:\(modelKey)",
                label: label,
                kind: .piper,
                systemName: nil,
                piperModel: modelKey,
                quality: 0
            ))
        }

        func priority(_ v: VoiceOption) -> Int {
            if v.piperModel == "en_US-amy-medium" { return 0 }
            if v.piperModel == "en_US-ryan-medium" { return 1 }
            return 2
        }

        return result.sorted { a, b in
            let pa = priority(a), pb = priority(b)
            if pa != pb { return pa < pb }
            return a.label.localizedCaseInsensitiveCompare(b.label) == .orderedAscending
        }
    }

    // MARK: - macOS system voices

    static func systemVoices() -> [VoiceOption] {
        // AVSpeechSynthesisVoice is the authoritative list of voices installed
        // for this user, and unlike `say -v ?` it exposes the quality tier
        // (compact / enhanced / premium) and a stable identifier that `say -v`
        // also accepts.
        var bestByName: [String: VoiceOption] = [:]

        for voice in AVSpeechSynthesisVoice.speechVoices() {
            guard voice.language.lowercased().hasPrefix("en") else { continue }

            let base = Self.baseName(voice.name)
            guard !base.isEmpty, !Self.noveltyNames.contains(base) else { continue }

            let quality = Int(voice.quality.rawValue)
            let option = VoiceOption(
                id: "system:\(voice.identifier)",
                label: Self.nativeLabel(name: voice.name, language: voice.language, quality: quality),
                kind: .system,
                systemName: voice.identifier,
                piperModel: nil,
                quality: quality
            )

            let key = base.lowercased()
            if let existing = bestByName[key], existing.quality >= quality {
                continue
            }
            bestByName[key] = option
        }

        // Short, curated English list. Higher-quality downloads (Enhanced /
        // Premium) are sorted to the top automatically.
        var result: [(index: Int, voice: VoiceOption)] = []
        for (index, name) in Self.preferredEnglishNames.enumerated() {
            if let voice = bestByName[name.lowercased()] {
                result.append((index, voice))
            }
        }

        if result.isEmpty {
            let fallback = bestByName.values.sorted { a, b in
                if a.quality != b.quality { return a.quality > b.quality }
                return a.label.localizedCaseInsensitiveCompare(b.label) == .orderedAscending
            }
            return Array(fallback.prefix(Self.maxNativeVoices))
        }

        result.sort { a, b in
            if a.voice.quality != b.voice.quality { return a.voice.quality > b.voice.quality }
            return a.index < b.index
        }
        return Array(result.map { $0.voice }.prefix(Self.maxNativeVoices))
    }

    /// "Ava (Premium)" -> "Ava"
    private static func baseName(_ name: String) -> String {
        var base = name
        for suffix in [" (Enhanced)", " (Premium)", " (Compact)"] {
            if base.hasSuffix(suffix) {
                base = String(base.dropLast(suffix.count))
            }
        }
        return base
    }

    private static func nativeLabel(name: String, language: String, quality: Int) -> String {
        let locale = language.replacingOccurrences(of: "-", with: "_")
        var display = name
        let lower = display.lowercased()
        if quality >= 3, !lower.contains("premium") {
            display += " (Premium)"
        } else if quality == 2, !lower.contains("enhanced") {
            display += " (Enhanced)"
        }
        return "\(display) — \(locale)"
    }

    /// Standard, useful English voices in rough order of preference. Only
    /// entries actually installed on this Mac are shown.
    private static let preferredEnglishNames: [String] = [
        "Samantha", "Alex", "Ava", "Allison", "Susan", "Tom", "Kathy",
        "Daniel", "Serena", "Kate", "Oliver", "Arthur", "Martha",
        "Karen", "Lee", "Matilda", "Moira", "Tessa",
        "Rishi", "Aman", "Tara", "Veena"
    ]

    /// Joke/novelty voices that `say` also exposes.
    private static let noveltyNames: Set<String> = [
        "Albert", "Bad News", "Bahh", "Bells", "Boing", "Bubbles", "Cellos",
        "Fred", "Good News", "Grandma", "Grandpa", "Jester", "Junior", "Organ",
        "Ralph", "Rocko", "Sandy", "Shelley", "Superstar", "Trinoids",
        "Whisper", "Wobble", "Zarvox", "Eddy", "Flo", "Reed"
    ]

    private static let maxNativeVoices = 10
}
