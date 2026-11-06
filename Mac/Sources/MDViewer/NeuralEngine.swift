import Foundation
import Darwin

/// One persistent `neural-tts --serve` helper process.
///
/// The helper keeps the ONNX model and espeak loaded between lines, so reading
/// a paragraph pays the model load cost once instead of once per line. Requests
/// are newline-delimited JSON and serialized with a lock; the helper renders
/// one line at a time.
final class NeuralServer {
    let family: String
    let process: Process

    private let stdinPipe: Pipe
    private let stdoutPipe: Pipe
    private let lock = NSLock()

    init?(family: String) {
        guard let bin = NeuralLocator.binary(),
              let model = NeuralLocator.model(family: family),
              let config = NeuralLocator.config(family: family),
              let ort = NeuralLocator.ortLibrary(),
              let espeak = NeuralLocator.espeakLibrary(),
              let data = NeuralLocator.espeakData() else {
            return nil
        }

        var args = [
            "--serve",
            "--family", family,
            "--model", model.path,
            "--config", config.path,
            "--ort", ort.path,
            "--espeak-dll", espeak.path,
            "--espeak-data", data.path
        ]
        if family == "kokoro" {
            let voices = NeuralLocator.neuralDir()?
                .appendingPathComponent("voices-v1.0.bin")
            guard let voices, FileManager.default.fileExists(atPath: voices.path) else {
                return nil
            }
            args += ["--voices", voices.path]
        }

        let stdinPipe = Pipe()
        let stdoutPipe = Pipe()
        let process = Process()
        process.executableURL = bin
        process.currentDirectoryURL = bin.deletingLastPathComponent()
        process.arguments = args
        process.standardInput = stdinPipe
        process.standardOutput = stdoutPipe
        process.standardError = FileHandle.nullDevice

        do {
            try process.run()
        } catch {
            return nil
        }

        self.family = family
        self.process = process
        self.stdinPipe = stdinPipe
        self.stdoutPipe = stdoutPipe
    }

    var isRunning: Bool { process.isRunning }

    /// Renders one line to `out`. Returns false when the helper reported an
    /// error; throws when the process died or the protocol broke.
    @discardableResult
    func render(text: String, voice: String, speed: Double, lang: String, out: String) throws -> Bool {
        lock.lock()
        defer { lock.unlock() }

        let request: [String: Any] = [
            "text": text,
            "voice": voice,
            "speed": speed,
            "lang": lang,
            "out": out
        ]
        let payload = try JSONSerialization.data(withJSONObject: request)
        var line = payload
        line.append(0x0A)

        do {
            try stdinPipe.fileHandleForWriting.write(contentsOf: line)
        } catch {
            throw SpeechError.renderFailed
        }

        let response = try readLine()
        let object = try JSONSerialization.jsonObject(with: response) as? [String: Any]
        guard let ok = object?["ok"] as? Bool else {
            throw SpeechError.renderFailed
        }
        return ok
    }

    private func readLine() throws -> Data {
        var buffer = Data()
        let handle = stdoutPipe.fileHandleForReading
        while true {
            let chunk = handle.availableData
            if chunk.isEmpty {
                throw SpeechError.renderFailed
            }
            buffer.append(chunk)
            if let newline = buffer.firstIndex(of: 0x0A) {
                return buffer.prefix(upTo: newline)
            }
            if buffer.count > 4 * 1024 * 1024 {
                throw SpeechError.renderFailed
            }
        }
    }

    func terminate() {
        guard process.isRunning else { return }
        let pid = process.processIdentifier
        process.terminate()
        kill(pid, SIGKILL)
    }
}

/// Builds the neural entries shown in the voice picker by asking the helper to
/// list the voices baked into each model's config.
enum NeuralVoiceCatalog {
    static func voices() -> [VoiceOption] {
        var result: [VoiceOption] = []
        result += list(family: "paradee")
        // Kokoro needs 120 MB of assets and renders far below real time on
        // most Macs; only advertise it when the user opted in.
        if ProcessInfo.processInfo.environment["MDVIEWER_NEURAL_KOKORO"] == "1" {
            result += list(family: "kokoro")
        }
        return result
    }

    private static func list(family: String) -> [VoiceOption] {
        guard NeuralLocator.isReady(family: family),
              let bin = NeuralLocator.binary(),
              let config = NeuralLocator.config(family: family) else {
            return []
        }

        var args = ["--list-voices", "--family", family, "--config", config.path]
        if family == "kokoro" {
            guard let dir = NeuralLocator.neuralDir() else { return [] }
            let voices = dir.appendingPathComponent("voices-v1.0.bin")
            guard FileManager.default.fileExists(atPath: voices.path) else { return [] }
            args += ["--voices", voices.path]
        }

        let process = Process()
        process.executableURL = bin
        process.arguments = args
        process.standardError = FileHandle.nullDevice
        let pipe = Pipe()
        process.standardOutput = pipe

        do {
            try process.run()
        } catch {
            return []
        }
        let data = pipe.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        guard process.terminationStatus == 0,
              let text = String(data: data, encoding: .utf8) else {
            return []
        }

        let kind: VoiceOption.Kind = family == "kokoro" ? .kokoro : .paradee
        var voices: [VoiceOption] = []
        for raw in text.split(separator: "\n") {
            let parts = raw.split(separator: "\t", omittingEmptySubsequences: false)
            guard parts.count >= 3, parts[0] == Substring(family) else { continue }
            let key = String(parts[1])
            let label = String(parts[2])
            voices.append(VoiceOption(
                id: "\(family):\(key)",
                label: label,
                kind: kind,
                systemName: nil,
                piperModel: nil,
                neuralVoice: key,
                quality: 0
            ))
        }
        return voices
    }
}


