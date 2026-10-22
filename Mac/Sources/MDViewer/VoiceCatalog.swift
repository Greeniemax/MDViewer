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
        piperVoices() + systemVoices()
    }

    static func defaultVoice(from voices: [VoiceOption]) -> VoiceOption? {
        if let amy = voices.first(where: { $0.piperModel == "en_US-amy-medium" }) {
            return amy
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
                piperModel: modelKey
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
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/say")
        process.arguments = ["-v", "?"]
        let pipe = Pipe()
        process.standardOutput = pipe
        process.standardError = FileHandle.nullDevice

        do {
            try process.run()
        } catch {
            return []
        }

        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard let output = String(data: data, encoding: .utf8) else { return [] }

        let regex = try! NSRegularExpression(
            pattern: #"^(.+?)\s{2,}([a-zA-Z]{2}_[a-zA-Z0-9]+)\s+#"#
        )
        let fallbackRegex = try! NSRegularExpression(
            pattern: #"^(\S+)\s+(\S+)"#
        )

        var voices: [VoiceOption] = []
        var seen = Set<String>()

        for line in output.components(separatedBy: "\n") {
            let trimmed = line.trimmingCharacters(in: .whitespacesAndNewlines)
            if trimmed.isEmpty { continue }
            let ns = line as NSString
            let range = NSRange(location: 0, length: ns.length)

            var name = ""
            var locale = ""

            if let match = regex.firstMatch(in: line, range: range) {
                name = ns.substring(with: match.range(at: 1))
                    .trimmingCharacters(in: .whitespaces)
                locale = ns.substring(with: match.range(at: 2))
            } else if let match = fallbackRegex.firstMatch(in: line, range: range) {
                name = ns.substring(with: match.range(at: 1))
                locale = ns.substring(with: match.range(at: 2))
            }

            guard !name.isEmpty, !seen.contains(name) else { continue }
            seen.insert(name)

            let label = locale.isEmpty ? name : "\(name) — \(locale)"
            voices.append(VoiceOption(
                id: "system:\(name)",
                label: label,
                kind: .system,
                systemName: name,
                piperModel: nil
            ))
        }

        return voices.sorted {
            $0.label.localizedCaseInsensitiveCompare($1.label) == .orderedAscending
        }
    }
}
