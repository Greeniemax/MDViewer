import AVFoundation
import Foundation
import Darwin

enum SpeechError: Error {
    case stopped
    case engineMissing
    case renderFailed
    case playbackFailed
}

final class PrefetchEntry {
    let semaphore = DispatchSemaphore(value: 0)
    var path: String?
    var error: Error?
    var process: Process?
    var synthesizer: AVSpeechSynthesizer?
}

/// File-then-play speech pipeline.
///
/// Every engine renders a complete audio file first (Piper -> WAV, macOS
/// AVSpeechSynthesizer -> WAV), then `afplay` plays it. That makes it possible
/// to render line N+1 while line N is still playing, so paragraphs flow
/// without synthesis gaps. Stop kills the player, any in-flight Piper process
/// and any active speech synthesizer immediately, then discards queued files.
final class SpeechEngine {
    private let lock = NSLock()
    private var session = 0
    private var stopRequested = false
    private var prefetched: [String: PrefetchEntry] = [:]
    private var player: Process?
    private var inlineSynth: Process?
    private var inlineSynthesizer: AVSpeechSynthesizer?

    private let work = DispatchQueue(label: "com.mdviewer.speech.work")
    private let render = DispatchQueue(
        label: "com.mdviewer.speech.render",
        qos: .userInitiated,
        attributes: .concurrent
    )

    // MARK: - Public API

    func speak(
        jobs: [SpeechJob],
        voice: VoiceOption,
        rate: Int,
        onLine: @escaping (Int) -> Void,
        onFinish: @escaping () -> Void
    ) {
        lock.lock()
        session += 1
        let mySession = session
        stopRequested = false
        lock.unlock()

        work.async { [weak self] in
            guard let self else { return }

            guard !jobs.isEmpty else {
                self.finish(onFinish)
                return
            }

            // Prime the first line immediately.
            self.prefetch(
                text: jobs[0].text,
                voice: voice,
                rate: rate,
                line: jobs[0].index
            )

            var index = 0
            while index < jobs.count {
                if self.isStopped(mySession) { break }

                let job = jobs[index]
                DispatchQueue.main.async { onLine(job.index) }

                // Ensure this line is rendering, then start the next one so it
                // is ready the moment this line's playback ends.
                self.prefetch(
                    text: job.text,
                    voice: voice,
                    rate: rate,
                    line: job.index
                )
                if index + 1 < jobs.count {
                    let next = jobs[index + 1]
                    self.prefetch(
                        text: next.text,
                        voice: voice,
                        rate: rate,
                        line: next.index
                    )
                }

                do {
                    try self.speakOne(
                        text: job.text,
                        voice: voice,
                        rate: rate,
                        line: job.index,
                        session: mySession
                    )
                } catch {
                    if self.isStopped(mySession) { break }
                    // Rendering failed for this line; continue with the next.
                    NSLog("MDViewer TTS: line \(job.index + 1) failed: \(error)")
                }

                if self.isStopped(mySession) { break }
                index += 1
                if index < jobs.count {
                    Thread.sleep(forTimeInterval: 0.06)
                }
            }

            self.cleanup()
            self.finish(onFinish)
        }
    }

    /// Immediately stops playback and any in-flight synthesis. Safe to call
    /// from the main thread; it never waits for child processes to exit.
    func stop() {
        lock.lock()
        session += 1
        stopRequested = true

        let activePlayer = player
        player = nil
        let activeSynth = inlineSynth
        inlineSynth = nil
        let activeSynthesizer = inlineSynthesizer
        inlineSynthesizer = nil
        let entries = Array(prefetched.values)
        prefetched.removeAll()
        lock.unlock()

        if let activePlayer { terminate(activePlayer) }
        if let activeSynth { terminate(activeSynth) }
        if let activeSynthesizer { activeSynthesizer.stopSpeaking(at: .immediate) }

        for entry in entries {
            lock.lock()
            let process = entry.process
            let path = entry.path
            let synthesizer = entry.synthesizer
            lock.unlock()

            if let process { terminate(process) }
            synthesizer?.stopSpeaking(at: .immediate)
            if let path {
                try? FileManager.default.removeItem(atPath: path)
            }
            // Wake a worker blocked waiting for this render.
            entry.semaphore.signal()
        }
    }

    // MARK: - Prefetch

    private func prefetch(text: String, voice: VoiceOption, rate: Int, line: Int) {
        let key = Self.key(text: text, voice: voice, rate: rate, line: line)

        lock.lock()
        if stopRequested || prefetched[key] != nil {
            lock.unlock()
            return
        }
        let entry = PrefetchEntry()
        prefetched[key] = entry
        let mySession = session
        lock.unlock()

        render.async { [weak self] in
            guard let self else { return }

            self.lock.lock()
            let cancelled = self.stopRequested || self.session != mySession
            self.lock.unlock()
            if cancelled {
                self.lock.lock()
                if self.prefetched[key] === entry {
                    self.prefetched.removeValue(forKey: key)
                }
                self.lock.unlock()
                entry.error = SpeechError.stopped
                entry.semaphore.signal()
                return
            }

            let result = self.renderToFile(
                text: text,
                voice: voice,
                rate: rate,
                entry: entry,
                inline: false
            )

            self.lock.lock()
            let wanted = (self.prefetched[key] === entry)
                && (self.session == mySession)
                && !self.stopRequested

            if wanted {
                entry.path = result.path
                entry.error = result.error
            } else {
                if self.prefetched[key] === entry {
                    self.prefetched.removeValue(forKey: key)
                }
                entry.error = SpeechError.stopped
            }
            self.lock.unlock()

            if !wanted, let path = result.path {
                try? FileManager.default.removeItem(atPath: path)
            }
            entry.semaphore.signal()
        }
    }

    // MARK: - Playback

    private func speakOne(
        text: String,
        voice: VoiceOption,
        rate: Int,
        line: Int,
        session mySession: Int
    ) throws {
        let key = Self.key(text: text, voice: voice, rate: rate, line: line)

        lock.lock()
        let entry = prefetched[key]
        let stoppedBefore = stopRequested || session != mySession
        lock.unlock()

        if stoppedBefore { throw SpeechError.stopped }

        let path: String
        let renderError: Error?

        if let entry {
            entry.semaphore.wait()

            lock.lock()
            path = entry.path ?? ""
            renderError = entry.error
            if prefetched[key] === entry {
                prefetched.removeValue(forKey: key)
            }
            let stoppedNow = stopRequested || session != mySession
            lock.unlock()

            if stoppedNow {
                if !path.isEmpty {
                    try? FileManager.default.removeItem(atPath: path)
                }
                throw SpeechError.stopped
            }
        } else {
            let transient = PrefetchEntry()
            let result = renderToFile(
                text: text,
                voice: voice,
                rate: rate,
                entry: transient,
                inline: true
            )
            path = result.path ?? ""
            renderError = result.error
        }

        if let renderError { throw renderError }
        guard !path.isEmpty else { throw SpeechError.renderFailed }

        lock.lock()
        let stoppedNow = stopRequested || session != mySession
        lock.unlock()
        if stoppedNow {
            try? FileManager.default.removeItem(atPath: path)
            throw SpeechError.stopped
        }

        defer { try? FileManager.default.removeItem(atPath: path) }
        try play(path: path, session: mySession)
    }

    private func play(path: String, session mySession: Int) throws {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/usr/bin/afplay")
        process.arguments = [path]
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice

        do {
            try process.run()
        } catch {
            throw SpeechError.playbackFailed
        }

        lock.lock()
        let cancelled = stopRequested || session != mySession
        if !cancelled { player = process }
        lock.unlock()

        if cancelled {
            terminate(process)
            throw SpeechError.stopped
        }

        process.waitUntilExit()

        lock.lock()
        if player === process { player = nil }
        lock.unlock()

        if process.terminationStatus != 0 {
            throw SpeechError.playbackFailed
        }
    }

    // MARK: - Rendering

    private func renderToFile(
        text: String,
        voice: VoiceOption,
        rate: Int,
        entry: PrefetchEntry,
        inline: Bool
    ) -> (path: String?, error: Error?) {
        if voice.kind == .piper, let model = voice.piperModel {
            let result = renderPiper(
                text: text,
                modelKey: model,
                rate: rate,
                entry: entry,
                inline: inline
            )
            if result.path != nil { return result }
            if stoppedNow() { return result }

            // Piper unavailable or failed: fall back to the default macOS
            // voice rather than dropping the line.
            return renderSystem(
                text: text,
                systemName: nil,
                rate: rate,
                entry: entry,
                inline: inline
            )
        }

        return renderSystem(
            text: text,
            systemName: voice.systemName,
            rate: rate,
            entry: entry,
            inline: inline
        )
    }

    private func renderPiper(
        text: String,
        modelKey: String,
        rate: Int,
        entry: PrefetchEntry,
        inline: Bool
    ) -> (path: String?, error: Error?) {
        let fm = FileManager.default
        guard
            let binary = PiperLocator.binary(),
            let voices = PiperLocator.voicesDir()
        else { return (nil, SpeechError.engineMissing) }

        let model = voices.appendingPathComponent(modelKey + ".onnx")
        guard fm.isExecutableFile(atPath: binary.path),
              fm.fileExists(atPath: model.path) else {
            return (nil, SpeechError.engineMissing)
        }

        let output = Self.tempFileURL(ext: "wav")
        let process = Process()
        process.executableURL = binary
        process.currentDirectoryURL = binary.deletingLastPathComponent()
        process.arguments = [
            "--model", model.path,
            "--output_file", output.path,
            "--length_scale", String(format: "%g", Self.lengthScale(rate))
        ]

        let stdinPipe = Pipe()
        process.standardInput = stdinPipe
        process.standardOutput = FileHandle.nullDevice
        process.standardError = FileHandle.nullDevice

        do {
            try process.run()
        } catch {
            return (nil, error)
        }

        register(process, entry: entry, inline: inline)

        if let data = (text + "\n").data(using: .utf8) {
            try? stdinPipe.fileHandleForWriting.write(contentsOf: data)
        }
        try? stdinPipe.fileHandleForWriting.close()

        process.waitUntilExit()
        unregister(process, entry: entry, inline: inline)

        if process.terminationStatus != 0 || !fm.fileExists(atPath: output.path) {
            try? fm.removeItem(at: output)
            return (nil, stoppedNow() ? SpeechError.stopped : SpeechError.renderFailed)
        }
        return (output.path, nil)
    }

    private func renderSystem(
        text: String,
        systemName: String?,
        rate: Int,
        entry: PrefetchEntry,
        inline: Bool
    ) -> (path: String?, error: Error?) {
        let fm = FileManager.default
        let output = Self.tempFileURL(ext: "wav")
        guard let voice = Self.systemVoice(identifier: systemName) else {
            return (nil, SpeechError.engineMissing)
        }

        let stateLock = NSLock()
        var file: AVAudioFile?
        var finished = false
        var writeFailed = false

        let synthesizer = AVSpeechSynthesizer()
        registerSynth(synthesizer, entry: entry, inline: inline)

        let utterance = AVSpeechUtterance(string: text)
        utterance.voice = voice
        utterance.rate = Self.avRate(rate)
        utterance.preUtteranceDelay = 0
        utterance.postUtteranceDelay = 0

        // `write` streams PCM buffers; assemble them into a WAV file so the
        // rest of the pipeline can prefetch and `afplay` it like any render.
        synthesizer.write(utterance) { buffer in
            guard let pcm = buffer as? AVAudioPCMBuffer else { return }
            if pcm.frameLength == 0 {
                stateLock.lock()
                finished = true
                stateLock.unlock()
                return
            }

            stateLock.lock()
            if file == nil {
                file = try? AVAudioFile(forWriting: output, settings: pcm.format.settings)
            }
            do {
                try file?.write(from: pcm)
            } catch {
                writeFailed = true
                finished = true
            }
            stateLock.unlock()
        }

        // The callbacks are delivered on this thread's run loop, so keep it
        // turning while the synthesizer works. Stop() breaks us out via
        // stopRequested and `stopSpeaking`.
        let deadline = Date().addingTimeInterval(120)
        while !stoppedNow() && Date() < deadline {
            stateLock.lock()
            let done = finished
            stateLock.unlock()
            if done { break }
            RunLoop.current.run(mode: .default, before: Date().addingTimeInterval(0.02))
        }

        unregisterSynth(synthesizer, entry: entry, inline: inline)
        stateLock.lock()
        file = nil
        let ok = finished && !writeFailed
        stateLock.unlock()

        if stoppedNow() {
            try? fm.removeItem(at: output)
            return (nil, SpeechError.stopped)
        }
        if !ok || !fm.fileExists(atPath: output.path) {
            try? fm.removeItem(at: output)
            return (nil, SpeechError.renderFailed)
        }
        return (output.path, nil)
    }

    private static func systemVoice(identifier: String?) -> AVSpeechSynthesisVoice? {
        if let identifier, !identifier.isEmpty,
           let voice = AVSpeechSynthesisVoice(identifier: identifier) {
            return voice
        }
        return AVSpeechSynthesisVoice(language: "en-US")
    }

    /// 100-250 WPM mapped onto AVSpeechUtterance's 0...1 rate scale.
    private static func avRate(_ wpm: Int) -> Float {
        let clamped = Double(max(80, min(300, wpm)))
        let value = 0.5 + (clamped - 150.0) * 0.003
        return Float(max(0.3, min(0.9, value)))
    }

    // MARK: - Child processes

    private func register(_ process: Process, entry: PrefetchEntry, inline: Bool) {
        lock.lock()
        entry.process = process
        if inline { inlineSynth = process }
        let cancelled = stopRequested
        lock.unlock()
        if cancelled { terminate(process) }
    }

    private func unregister(_ process: Process, entry: PrefetchEntry, inline: Bool) {
        lock.lock()
        if entry.process === process { entry.process = nil }
        if inline, inlineSynth === process { inlineSynth = nil }
        lock.unlock()
    }

    private func registerSynth(_ synthesizer: AVSpeechSynthesizer, entry: PrefetchEntry, inline: Bool) {
        lock.lock()
        entry.synthesizer = synthesizer
        if inline { inlineSynthesizer = synthesizer }
        let cancelled = stopRequested
        lock.unlock()
        if cancelled { synthesizer.stopSpeaking(at: .immediate) }
    }

    private func unregisterSynth(_ synthesizer: AVSpeechSynthesizer, entry: PrefetchEntry, inline: Bool) {
        lock.lock()
        if entry.synthesizer === synthesizer { entry.synthesizer = nil }
        if inline, inlineSynthesizer === synthesizer { inlineSynthesizer = nil }
        lock.unlock()
    }

    private func terminate(_ process: Process) {
        guard process.isRunning else { return }
        let pid = process.processIdentifier
        process.terminate()
        // SIGTERM can be ignored or delayed; SIGKILL makes Stop feel instant.
        kill(pid, SIGKILL)
    }

    // MARK: - State helpers

    private func isStopped(_ mySession: Int) -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return stopRequested || session != mySession
    }

    private func stoppedNow() -> Bool {
        lock.lock()
        defer { lock.unlock() }
        return stopRequested
    }

    private func cleanup() {
        lock.lock()
        let entries = Array(prefetched.values)
        prefetched.removeAll()
        lock.unlock()

        for entry in entries {
            lock.lock()
            let process = entry.process
            let path = entry.path
            let synthesizer = entry.synthesizer
            lock.unlock()

            if let process { terminate(process) }
            synthesizer?.stopSpeaking(at: .immediate)
            if let path {
                try? FileManager.default.removeItem(atPath: path)
            }
            entry.semaphore.signal()
        }
    }

    private func finish(_ onFinish: @escaping () -> Void) {
        DispatchQueue.main.async { onFinish() }
    }

    // MARK: - Utilities

    private static func key(text: String, voice: VoiceOption, rate: Int, line: Int) -> String {
        "\(rate)|\(voice.id)|\(line)|\(text)"
    }

    private static func lengthScale(_ rate: Int) -> Double {
        guard rate > 0 else { return 1.0 }
        return max(0.5, min(2.0, 175.0 / Double(rate)))
    }

    private static func tempFileURL(ext: String) -> URL {
        FileManager.default.temporaryDirectory
            .appendingPathComponent("mdviewer-tts-\(UUID().uuidString).\(ext)")
    }
}
