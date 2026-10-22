import Foundation
import AppKit
import UniformTypeIdentifiers
import Combine

final class AppModel: ObservableObject {
    static let shared = AppModel()

    // Documents
    @Published var tabs: [DocumentTab] = []
    @Published var activeTabID: UUID?
    @Published var mode: PaneMode = .editor

    // Viewer / speech state
    @Published var selectedLine: Int = 0
    @Published var highlightedLine: Int?
    @Published var readingStatus: String = ""
    @Published var isSpeaking: Bool = false
    @Published var blocks: [MarkdownBlock] = []

    // Voices
    @Published var voiceOptions: [VoiceOption] = []
    @Published var selectedVoiceID: String = "" {
        didSet { UserDefaults.standard.set(selectedVoiceID, forKey: "selectedVoiceID") }
    }
    @Published var rate: Double = 150 {
        didSet { UserDefaults.standard.set(rate, forKey: "speechRate") }
    }

    // Recent files
    @Published var recentFiles: [URL] = []

    // Find
    @Published var isFindVisible = false
    @Published var findQuery = ""
    @Published var findMatchCase = false
    @Published var findWholeWord = false
    @Published var findMatches: [NSRange] = []
    @Published var findIndex = 0
    @Published var editorSelection: EditorSelection?

    let editorController = EditorController()
    let speech = SpeechEngine()

    private var watchers: [UUID: FileWatcher] = [:]
    private var lastOwnWrite: [String: Date] = [:]
    private var renderWork: DispatchWorkItem?
    private var renderGeneration = 0
    private var speechSession = 0
    private var voiceLoadScheduled = false
    private var selectionToken = 0

    private let defaults = UserDefaults.standard
    private let recentKey = "mdviewer-recent-files-v1"
    private let maxRecentFiles = 10

    private init() {
        loadRecentFiles()

        if let stored = defaults.object(forKey: "speechRate") as? Double {
            rate = stored
        }
        selectedVoiceID = defaults.string(forKey: "selectedVoiceID") ?? ""

        ensureUntitledTab()
        scheduleVoiceLoad()

        // Give SwiftUI a chance to install the window before opening a file
        // passed on the command line.
        DispatchQueue.main.async { [weak self] in
            self?.handleLaunchArguments()
        }
    }

    // MARK: - Active document

    var activeTabIndex: Int? {
        guard let activeTabID else { return nil }
        return tabs.firstIndex(where: { $0.id == activeTabID })
    }

    var activeTab: DocumentTab? {
        guard let index = activeTabIndex else { return nil }
        return tabs[index]
    }

    var activeText: String { activeTab?.text ?? "" }
    var activePath: URL? { activeTab?.path }

    var wordCount: Int {
        activeText.split(whereSeparator: { $0.isWhitespace }).count
    }

    var statusFileName: String {
        activeTab?.path?.path ?? "No file open"
    }

    func updateActiveText(_ text: String) {
        guard let index = activeTabIndex else { return }
        tabs[index].text = text
        scheduleRender(text)
    }

    func ensureUntitledTab() {
        guard tabs.isEmpty else { return }
        let tab = DocumentTab.untitled()
        tabs = [tab]
        activeTabID = tab.id
        blocks = []
    }

    func newTab() {
        if let empty = tabs.first(where: { $0.path == nil && $0.text.isEmpty }) {
            selectTab(empty.id)
            return
        }
        let tab = DocumentTab.untitled()
        tabs.append(tab)
        activeTabID = tab.id
        selectedLine = 0
        highlightedLine = nil
        blocks = []
        syncWatchers()
    }

    func selectTab(_ id: UUID) {
        guard let index = tabs.firstIndex(where: { $0.id == id }) else { return }
        if activeTabID != id {
            if isSpeaking { stopSpeaking() }
            activeTabID = id
            selectedLine = 0
            highlightedLine = nil
        }
        scheduleRender(tabs[index].text)
    }

    func closeActiveTab() {
        guard let id = activeTabID else { return }
        closeTab(id)
    }

    func closeTab(_ id: UUID) {
        guard let index = tabs.firstIndex(where: { $0.id == id }) else { return }

        if tabs[index].isDirty {
            let alert = NSAlert()
            alert.messageText = "Save changes to \(tabs[index].title)?"
            alert.informativeText = "Your changes will be lost if you do not save them."
            alert.alertStyle = .warning
            alert.addButton(withTitle: "Save")
            alert.addButton(withTitle: "Cancel")
            alert.addButton(withTitle: "Don't Save")
            let response = alert.runModal()

            if response == .alertFirstButtonReturn {
                activeTabID = id
                save()
                guard let refreshed = tabs.firstIndex(where: { $0.id == id }),
                      !tabs[refreshed].isDirty else { return }
            } else if response == .alertSecondButtonReturn {
                return
            }
        }

        if activeTabID == id { stopSpeaking() }

        if tabs.count == 1 {
            let replacement = DocumentTab.untitled()
            tabs = [replacement]
            activeTabID = replacement.id
            blocks = []
            selectedLine = 0
            highlightedLine = nil
        } else {
            tabs.remove(at: index)
            if activeTabID == id {
                let next = min(index, tabs.count - 1)
                activeTabID = tabs[next].id
                selectedLine = 0
                highlightedLine = nil
            }
            scheduleRender(activeText)
        }

        syncWatchers()
    }

    // MARK: - Open / save

    func openPanel() {
        let panel = NSOpenPanel()
        panel.canChooseFiles = true
        panel.canChooseDirectories = false
        panel.allowsMultipleSelection = false
        panel.title = "Open Markdown File"

        var types: [UTType] = [.plainText]
        if let md = UTType(filenameExtension: "md") { types.append(md) }
        if let markdown = UTType(filenameExtension: "markdown") { types.append(markdown) }
        panel.allowedContentTypes = types

        if panel.runModal() == .OK, let url = panel.url {
            open(url: url)
        }
    }

    func open(urls: [URL]) {
        for url in urls { open(url: url) }
    }

    func open(url: URL) {
        let standardized = url.standardizedFileURL
        var isDirectory: ObjCBool = false
        guard FileManager.default.fileExists(atPath: standardized.path, isDirectory: &isDirectory),
              !isDirectory.boolValue else { return }

        if let existing = tabs.first(where: {
            $0.path?.standardizedFileURL.path == standardized.path
        }) {
            selectTab(existing.id)
            addRecent(standardized)
            return
        }

        guard let text = readText(at: standardized) else {
            presentError("Could not read \(standardized.lastPathComponent).")
            return
        }

        if isSpeaking { stopSpeaking() }

        if let emptyIndex = tabs.firstIndex(where: {
            $0.path == nil && $0.text.isEmpty && $0.savedText.isEmpty
        }) {
            tabs[emptyIndex].path = standardized
            tabs[emptyIndex].title = standardized.lastPathComponent
            tabs[emptyIndex].text = text
            tabs[emptyIndex].savedText = text
            activeTabID = tabs[emptyIndex].id
        } else {
            let tab = DocumentTab(
                id: UUID(),
                path: standardized,
                title: standardized.lastPathComponent,
                text: text,
                savedText: text
            )
            tabs.append(tab)
            activeTabID = tab.id
        }

        selectedLine = 0
        highlightedLine = nil
        addRecent(standardized)
        scheduleRender(text)
        syncWatchers()
    }

    func save() {
        guard let index = activeTabIndex else { return }
        if let path = tabs[index].path {
            write(tabs[index].text, to: path, index: index)
        } else {
            saveAs()
        }
    }

    func saveAs() {
        guard let index = activeTabIndex else { return }

        let panel = NSSavePanel()
        panel.canCreateDirectories = true
        panel.title = "Save Markdown File"
        if let md = UTType(filenameExtension: "md") {
            panel.allowedContentTypes = [md]
        } else {
            panel.allowedContentTypes = [.plainText]
        }
        panel.nameFieldStringValue = tabs[index].title == "Untitled"
            ? "untitled.md"
            : tabs[index].title
        if let existing = tabs[index].path {
            panel.directoryURL = existing.deletingLastPathComponent()
        }

        if panel.runModal() == .OK, let url = panel.url {
            write(tabs[index].text, to: url, index: index)
        }
    }

    /// Converts the active document to a PDF in a separate file chosen by the
    /// user. The markdown source is never modified.
    func exportPDF() {
        guard let index = activeTabIndex else { return }
        let text = tabs[index].text
        guard !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty else { return }

        let panel = NSSavePanel()
        panel.canCreateDirectories = true
        panel.title = "Convert to PDF"
        panel.allowedContentTypes = [.pdf]
        panel.nameFieldStringValue = pdfDefaultName(for: tabs[index])
        if let existing = tabs[index].path {
            panel.directoryURL = existing.deletingLastPathComponent()
        }

        guard panel.runModal() == .OK, let url = panel.url else { return }

        do {
            try PDFExporter.writePDF(markdown: text, to: url)

            let alert = NSAlert()
            alert.messageText = "PDF saved"
            alert.informativeText = url.path
            alert.addButton(withTitle: "Show in Finder")
            alert.addButton(withTitle: "Done")
            if alert.runModal() == .alertFirstButtonReturn {
                NSWorkspace.shared.activateFileViewerSelecting([url])
            }
        } catch {
            presentError("Could not convert to PDF.\n\(error.localizedDescription)")
        }
    }

    private func pdfDefaultName(for tab: DocumentTab) -> String {
        if let path = tab.path {
            return path.deletingPathExtension().lastPathComponent + ".pdf"
        }
        let title = tab.title.trimmingCharacters(in: .whitespacesAndNewlines)
        let base = title.isEmpty || title == "Untitled" ? "untitled" : title
        return base + ".pdf"
    }

    private func readText(at url: URL) -> String? {
        let text: String
        if let utf8 = try? String(contentsOf: url, encoding: .utf8) {
            text = utf8
        } else if let latin = try? String(contentsOf: url, encoding: .isoLatin1) {
            text = latin
        } else {
            return nil
        }
        // Editors on other platforms often write CRLF. Normalize so the
        // gutter, parser, and TTS line splitting all agree.
        return text
            .replacingOccurrences(of: "\r\n", with: "\n")
            .replacingOccurrences(of: "\r", with: "\n")
    }

    private func write(_ text: String, to url: URL, index: Int) {
        let standardized = url.standardizedFileURL
        let tabID = tabs[index].id
        do {
            try text.write(to: standardized, atomically: true, encoding: .utf8)
            guard let current = tabs.firstIndex(where: { $0.id == tabID }) else { return }
            tabs[current].path = standardized
            tabs[current].title = standardized.lastPathComponent
            tabs[current].savedText = text
            lastOwnWrite[standardized.path] = Date()
            addRecent(standardized)
            syncWatchers()
        } catch {
            presentError("Could not save \(standardized.lastPathComponent).")
        }
    }

    // MARK: - Rendering

    func scheduleRender(_ text: String? = nil) {
        let source = text ?? activeText
        renderGeneration += 1
        let generation = renderGeneration
        renderWork?.cancel()

        let work = DispatchWorkItem { [weak self] in
            let parsed = MarkdownParser.parse(source)
            DispatchQueue.main.async {
                guard let self, self.renderGeneration == generation else { return }
                self.blocks = parsed
            }
        }
        renderWork = work
        DispatchQueue.global(qos: .userInitiated).asyncAfter(
            deadline: .now() + 0.12,
            execute: work
        )
    }

    // MARK: - File watching

    private func syncWatchers() {
        for watcher in watchers.values { watcher.stop() }
        watchers.removeAll()

        for tab in tabs {
            guard let path = tab.path else { continue }
            let id = tab.id
            let watcher = FileWatcher(url: path) { [weak self] in
                DispatchQueue.main.async {
                    self?.handleExternalChange(tabID: id)
                }
            }
            watchers[id] = watcher
            watcher.start()
        }
    }

    private func handleExternalChange(tabID: UUID) {
        guard
            let index = tabs.firstIndex(where: { $0.id == tabID }),
            let path = tabs[index].path
        else { return }

        if let own = lastOwnWrite[path.standardizedFileURL.path],
           Date().timeIntervalSince(own) < 1.0 {
            return
        }

        // Do not clobber unsaved edits.
        guard tabs[index].text == tabs[index].savedText else { return }
        guard let text = readText(at: path) else { return }

        tabs[index].text = text
        tabs[index].savedText = text
        if activeTabID == tabID {
            scheduleRender(text)
        }
    }

    // MARK: - Voice loading

    private func scheduleVoiceLoad() {
        guard !voiceLoadScheduled else { return }
        voiceLoadScheduled = true

        if let cached = VoiceCatalog.loadCached(), !cached.isEmpty {
            applyVoices(cached)
        }

        // Keep the launch path fast. Native voice enumeration waits until the
        // window has settled, and its result is cached for the next launch.
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.8) { [weak self] in
            DispatchQueue.global(qos: .utility).async {
                let fresh = VoiceCatalog.refresh()
                VoiceCatalog.saveCache(fresh)
                DispatchQueue.main.async {
                    self?.applyVoices(fresh)
                }
            }
        }
    }

    private func applyVoices(_ voices: [VoiceOption]) {
        guard !voices.isEmpty else { return }
        let previous = selectedVoiceID
        voiceOptions = voices

        if previous.isEmpty {
            if let preferred = VoiceCatalog.defaultVoice(from: voices) {
                selectedVoiceID = preferred.id
            }
        } else if previous != "system:",
                  !voices.contains(where: { $0.id == previous }) {
            if let preferred = VoiceCatalog.defaultVoice(from: voices) {
                selectedVoiceID = preferred.id
            }
        }
    }

    // MARK: - Speech

    func startSpeaking() {
        guard !isSpeaking else { return }
        guard let tab = activeTab, !tab.text.isEmpty else { return }

        let voice: VoiceOption
        if selectedVoiceID == "system:" {
            voice = VoiceOption(
                id: "system:",
                label: "Default Voice",
                kind: .system,
                systemName: nil,
                piperModel: nil
            )
        } else {
            voice = voiceOptions.first(where: { $0.id == selectedVoiceID })
                ?? VoiceCatalog.defaultVoice(from: voiceOptions)
                ?? VoiceOption(
                    id: "system:",
                    label: "Default Voice",
                    kind: .system,
                    systemName: nil,
                    piperModel: nil
                )
        }

        let allLines = tab.text.components(separatedBy: "\n")
        let start = max(0, min(selectedLine, max(0, allLines.count - 1)))
        var jobs: [SpeechJob] = []

        for (index, raw) in allLines.enumerated() where index >= start {
            let cleaned = cleanMarkdownForSpeech(raw)
            if !cleaned.isEmpty {
                jobs.append(SpeechJob(index: index, text: cleaned))
            }
        }

        guard !jobs.isEmpty else { return }

        speechSession += 1
        let mySession = speechSession
        isSpeaking = true
        readingStatus = "Starting…"

        speech.speak(
            jobs: jobs,
            voice: voice,
            rate: Int(rate.rounded()),
            onLine: { [weak self] line in
                guard let self, self.speechSession == mySession else { return }
                self.highlightedLine = line
                self.readingStatus = "Reading line \(line + 1) of \(allLines.count)…"
            },
            onFinish: { [weak self] in
                guard let self, self.speechSession == mySession else { return }
                self.isSpeaking = false
                self.readingStatus = ""
                self.highlightedLine = nil
            }
        )
    }

    func stopSpeaking() {
        speechSession += 1
        speech.stop()
        isSpeaking = false
        readingStatus = ""
        highlightedLine = nil
    }

    func cleanMarkdownForSpeech(_ input: String) -> String {
        var text = input.trimmingCharacters(in: .whitespacesAndNewlines)
        if text.isEmpty { return "" }
        if text.hasPrefix("```") { return "" }
        if text.contains("-"),
           text.range(of: #"^\s*\|?[\s:|-]+\|?\s*$"#, options: .regularExpression) != nil {
            return ""
        }

        text = text.replacingOccurrences(of: #"^#{1,6}\s+"#, with: "", options: .regularExpression)
        text = text.replacingOccurrences(of: #"^\s*[-*+]\s+"#, with: "", options: .regularExpression)
        text = text.replacingOccurrences(of: #"^\s*\d+\.\s+"#, with: "", options: .regularExpression)
        text = text.replacingOccurrences(of: #"^>\s?"#, with: "", options: .regularExpression)
        text = text.replacingOccurrences(of: #"!\[([^\]]*)\]\([^)]+\)"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"\[([^\]]+)\]\([^)]+\)"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"\*\*(.+?)\*\*"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"__(.+?)__"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"\*(.+?)\*"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"_(.+?)_"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"~~(.+?)~~"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"`(.+?)`"#, with: "$1", options: .regularExpression)
        text = text.replacingOccurrences(of: #"<[^>]+>"#, with: "", options: .regularExpression)

        if text.contains("|") {
            text = text.replacingOccurrences(of: "|", with: ", ")
            text = text.replacingOccurrences(of: #",\s*,"#, with: ",", options: .regularExpression)
            text = text.trimmingCharacters(in: CharacterSet(charactersIn: ", "))
        }

        return text.trimmingCharacters(in: .whitespacesAndNewlines)
    }

    // MARK: - Find

    func showFind() {
        if let textView = editorController.textView {
            let selected = textView.selectedRange()
            if selected.length > 0, selected.length < 200,
               let range = Range(selected, in: textView.string) {
                let value = String(textView.string[range])
                if !value.contains("\n"), !value.trimmingCharacters(in: .whitespaces).isEmpty {
                    findQuery = value
                }
            }
        }
        isFindVisible = true
        runFind()
    }

    func hideFind() {
        isFindVisible = false
        findMatches = []
        findIndex = 0
        if !isSpeaking {
            highlightedLine = nil
        }
    }

    func runFind() {
        computeFindMatches()
        findIndex = 0
        focusFindMatch()
    }

    func findNext() {
        guard !findMatches.isEmpty else { return }
        findIndex = (findIndex + 1) % findMatches.count
        focusFindMatch()
    }

    func findPrevious() {
        guard !findMatches.isEmpty else { return }
        findIndex = (findIndex - 1 + findMatches.count) % findMatches.count
        focusFindMatch()
    }

    var findSummary: String {
        if findQuery.isEmpty { return "" }
        if findMatches.isEmpty { return "No matches" }
        return "\(findIndex + 1) of \(findMatches.count)"
    }

    private func computeFindMatches() {
        let query = findQuery
        guard !query.isEmpty else {
            findMatches = []
            return
        }

        let source = activeText as NSString
        let fullRange = NSRange(location: 0, length: source.length)
        var matches: [NSRange] = []

        if findWholeWord {
            let escaped = NSRegularExpression.escapedPattern(for: query)
            let pattern = "\\b" + escaped + "\\b"
            let options: NSRegularExpression.Options = findMatchCase ? [] : [.caseInsensitive]
            guard let regex = try? NSRegularExpression(pattern: pattern, options: options) else {
                findMatches = []
                return
            }
            regex.enumerateMatches(in: activeText, options: [], range: fullRange) { match, _, _ in
                if let match {
                    matches.append(match.range)
                }
            }
            findMatches = matches
            return
        }

        let options: NSString.CompareOptions = findMatchCase ? [] : [.caseInsensitive]
        var searchRange = fullRange

        while searchRange.length > 0 {
            let found = source.range(of: query, options: options, range: searchRange)
            if found.location == NSNotFound { break }
            matches.append(found)
            let next = found.location + max(found.length, 1)
            if next >= source.length { break }
            searchRange = NSRange(location: next, length: source.length - next)
        }

        findMatches = matches
    }

    private func focusFindMatch() {
        guard !findMatches.isEmpty else {
            editorSelection = nil
            if !isSpeaking {
                highlightedLine = nil
            }
            return
        }
        findIndex = max(0, min(findIndex, findMatches.count - 1))
        let range = findMatches[findIndex]

        if mode == .editor {
            selectionToken += 1
            editorSelection = EditorSelection(range: range, token: selectionToken)
        } else {
            highlightedLine = max(0, lineNumber(at: range.location) - 1)
        }
    }

    func lineNumber(at location: Int) -> Int {
        let source = activeText as NSString
        let safe = max(0, min(location, source.length))
        let prefix = source.substring(to: safe)
        return prefix.components(separatedBy: "\n").count
    }

    // MARK: - Recent files

    private func loadRecentFiles() {
        let paths = defaults.stringArray(forKey: recentKey) ?? []
        recentFiles = paths
            .map { URL(fileURLWithPath: $0) }
            .filter { FileManager.default.fileExists(atPath: $0.path) }
    }

    private func addRecent(_ url: URL) {
        let standardized = url.standardizedFileURL
        var updated = recentFiles.filter {
            $0.standardizedFileURL.path != standardized.path
        }
        updated.insert(standardized, at: 0)
        if updated.count > maxRecentFiles {
            updated = Array(updated.prefix(maxRecentFiles))
        }
        recentFiles = updated
        defaults.set(updated.map { $0.path }, forKey: recentKey)
    }

    func clearRecentFiles() {
        recentFiles = []
        defaults.removeObject(forKey: recentKey)
    }

    // MARK: - Launch / errors

    private func handleLaunchArguments() {
        for argument in CommandLine.arguments.dropFirst() {
            if argument.hasPrefix("-") { continue }
            let url = URL(fileURLWithPath: argument)
            var isDirectory: ObjCBool = false
            if FileManager.default.fileExists(atPath: url.path, isDirectory: &isDirectory),
               !isDirectory.boolValue {
                open(url: url)
                break
            }
        }
    }

    func presentError(_ message: String) {
        let alert = NSAlert()
        alert.messageText = "MDViewer"
        alert.informativeText = message
        alert.alertStyle = .warning
        alert.addButton(withTitle: "OK")
        alert.runModal()
    }
}
