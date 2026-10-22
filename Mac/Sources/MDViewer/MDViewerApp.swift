import SwiftUI
import AppKit
import Darwin
import UniformTypeIdentifiers

@main
struct MDViewerApp: App {
    @NSApplicationDelegateAdaptor(AppDelegate.self) private var appDelegate
    @StateObject private var model = AppModel.shared

    var body: some Scene {
        WindowGroup("MDViewer") {
            ContentView(model: model)
                .frame(minWidth: 1000, minHeight: 640)
        }
        .defaultSize(width: 1220, height: 820)
        .commands {
            AppCommands(model: model)
        }
    }
}

final class AppDelegate: NSObject, NSApplicationDelegate {
    func applicationDidFinishLaunching(_ notification: Notification) {
        // A failed Piper/stdin write must not terminate the app.
        signal(SIGPIPE, SIG_IGN)

        _ = NSApp.setActivationPolicy(.regular)
        NSApp.activate(ignoringOtherApps: true)

        // `open -a MDViewer --args --set-default-markdown` asks macOS to make
        // this app the default handler for Markdown files. macOS shows its own
        // confirmation prompt; a plain CLI call cannot change the association.
        if CommandLine.arguments.contains("--set-default-markdown") {
            Self.requestDefaultMarkdownHandler()
        }
    }

    static func requestDefaultMarkdownHandler() {
        guard let markdown = UTType("net.daringfireball.markdown") else { return }
        let app = Bundle.main.bundleURL
        _ = try? NSWorkspace.shared.setDefaultApplication(at: app, toOpen: markdown)
    }

    func application(_ application: NSApplication, open urls: [URL]) {
        AppModel.shared.open(urls: urls)
    }

    func applicationWillTerminate(_ notification: Notification) {
        AppModel.shared.stopSpeaking()
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        true
    }
}

struct AppCommands: Commands {
    @ObservedObject var model: AppModel

    var body: some Commands {
        CommandGroup(replacing: .newItem) {
            Button("New Tab") {
                model.newTab()
            }
            .keyboardShortcut("n", modifiers: .command)

            Button("Open…") {
                model.openPanel()
            }
            .keyboardShortcut("o", modifiers: .command)
        }

        CommandGroup(replacing: .saveItem) {
            Button("Save") {
                model.save()
            }
            .keyboardShortcut("s", modifiers: .command)

            Button("Save As…") {
                model.saveAs()
            }
            .keyboardShortcut("s", modifiers: [.command, .shift])

            Button("Convert to PDF…") {
                model.exportPDF()
            }
            .keyboardShortcut("e", modifiers: [.command, .shift])
            .disabled(model.activeText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)

            Divider()

            Button("Set as Default Markdown App") {
                AppDelegate.requestDefaultMarkdownHandler()
            }

            Divider()

            Button("Close Tab") {
                model.closeActiveTab()
            }
            .keyboardShortcut("w", modifiers: .command)
        }

        CommandGroup(after: .textEditing) {
            Divider()
            Button("Find…") {
                model.showFind()
            }
            .keyboardShortcut("f", modifiers: .command)
        }

        CommandMenu("Speech") {
            Button("Speak") {
                if model.mode != .viewer {
                    model.mode = .viewer
                    model.scheduleRender()
                }
                model.startSpeaking()
            }
            .keyboardShortcut(.return, modifiers: .command)
            .disabled(model.isSpeaking || model.activeText.isEmpty)

            Button("Stop") {
                model.stopSpeaking()
            }
            .keyboardShortcut(".", modifiers: .command)
            .disabled(!model.isSpeaking)

            Divider()

            Button("Refresh Voices") {
                model.refreshVoices()
            }

            Button("Download Natural Voices…") {
                if let url = URL(
                    string: "x-apple.systempreferences:com.apple.preference.universalaccess?SpokenContent"
                ) {
                    NSWorkspace.shared.open(url)
                }
            }
        }

        CommandMenu("Pane") {
            Button("MD Editor") {
                model.mode = .editor
            }
            .keyboardShortcut("1", modifiers: .command)

            Button("MD Viewer") {
                model.mode = .viewer
                model.scheduleRender()
            }
            .keyboardShortcut("2", modifiers: .command)
        }
    }
}
