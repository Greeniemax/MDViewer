import Foundation

/// Lightweight polling file watcher. Polling once every 750 ms is cheap and
/// far more reliable than kqueue/FSEvents when editors replace files by
/// renaming them (vim, VS Code, git checkout, etc.).
final class FileWatcher {
    let url: URL
    private let onChange: () -> Void
    private var timer: DispatchSourceTimer?
    private var lastModified: Date?
    private var lastSize: Int64?

    init(url: URL, onChange: @escaping () -> Void) {
        self.url = url.standardizedFileURL
        self.onChange = onChange
        let attrs = attributes()
        lastModified = attrs.modified
        lastSize = attrs.size
    }

    func start() {
        guard timer == nil else { return }
        let source = DispatchSource.makeTimerSource(queue: DispatchQueue.global(qos: .utility))
        source.schedule(deadline: .now() + 0.75, repeating: 0.75, leeway: .milliseconds(250))
        source.setEventHandler { [weak self] in self?.poll() }
        timer = source
        source.resume()
    }

    func stop() {
        timer?.cancel()
        timer = nil
    }

    deinit {
        stop()
    }

    private func attributes() -> (modified: Date?, size: Int64?) {
        guard let attrs = try? FileManager.default.attributesOfItem(atPath: url.path) else {
            return (nil, nil)
        }
        let modified = attrs[.modificationDate] as? Date
        let size = (attrs[.size] as? NSNumber)?.int64Value
        return (modified, size)
    }

    private func poll() {
        let attrs = attributes()
        guard let modified = attrs.modified else { return }
        let changed = (lastModified != modified) || (lastSize != attrs.size)
        lastModified = modified
        lastSize = attrs.size
        if changed { onChange() }
    }
}
