import SwiftUI
import UniformTypeIdentifiers
import AppKit

struct ContentView: View {
    @ObservedObject var model: AppModel
    @State private var dropTargeted = false

    var body: some View {
        VStack(spacing: 0) {
            header
            modeTabs
            toolbar
            documentTabs
            if model.isFindVisible {
                FindBar(model: model)
            }
            Divider().background(Theme.border)
            content
            statusBar
        }
        .background(Theme.background)
        .preferredColorScheme(.dark)
        .onDrop(of: [UTType.fileURL], isTargeted: $dropTargeted) { providers in
            handleDrop(providers)
        }
        .overlay {
            if dropTargeted {
                RoundedRectangle(cornerRadius: 12, style: .continuous)
                    .stroke(
                        Theme.accent,
                        style: StrokeStyle(lineWidth: 2, dash: [8, 5])
                    )
                    .padding(10)
                    .allowsHitTesting(false)
            }
        }
    }

    // MARK: - Header

    private var header: some View {
        HStack(spacing: 8) {
            Text("📄")
                .font(.system(size: 15))
            Text("MDViewer")
                .font(.system(size: 15, weight: .bold))
                .foregroundColor(Theme.text)
            Spacer()
            Text(model.activeTab?.isDirty == true ? "Unsaved changes" : "")
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 9)
        .background(Theme.panel)
    }

    // MARK: - Mode tabs

    private var modeTabs: some View {
        HStack(spacing: 0) {
            modeTab("MD Editor", mode: .editor)
            modeTab("MD Viewer", mode: .viewer)
            Spacer()
        }
        .padding(.horizontal, 10)
        .background(Theme.panel)
        .overlay(alignment: .bottom) {
            Rectangle().fill(Theme.border).frame(height: 1)
        }
    }

    private func modeTab(_ title: String, mode: PaneMode) -> some View {
        Button {
            if model.mode != mode {
                model.mode = mode
                if mode == .viewer {
                    model.scheduleRender()
                }
            }
        } label: {
            Text(title)
                .font(.system(size: 12, weight: .semibold))
                .foregroundColor(model.mode == mode ? Theme.text : Theme.dimText)
                .padding(.horizontal, 16)
                .padding(.vertical, 8)
                .overlay(alignment: .bottom) {
                    Rectangle()
                        .fill(model.mode == mode ? Theme.accent : Color.clear)
                        .frame(height: 2)
                }
                .contentShape(Rectangle())
        }
        .buttonStyle(.plain)
    }

    // MARK: - Toolbar

    private var toolbar: some View {
        HStack(spacing: 8) {
            Button {
                model.openPanel()
            } label: {
                Label("Open", systemImage: "folder")
            }
            .buttonStyle(MDButtonStyle(color: Theme.accent))

            Button {
                model.save()
            } label: {
                Label("Save", systemImage: "square.and.arrow.down")
            }
            .buttonStyle(MDButtonStyle(color: Theme.accent))
            .disabled(model.activeTab == nil)

            Button {
                model.showFind()
            } label: {
                Label("Find", systemImage: "magnifyingglass")
            }
            .buttonStyle(MDButtonStyle(color: Theme.accent))

            recentMenu

            Spacer(minLength: 8)

            if model.mode == .viewer {
                speechControls
            }
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 8)
        .background(Theme.background)
    }

    private var recentMenu: some View {
        Menu {
            if model.recentFiles.isEmpty {
                Text("No recent files")
            } else {
                ForEach(model.recentFiles, id: \.self) { url in
                    Button(url.lastPathComponent) {
                        model.open(url: url)
                    }
                }
                Divider()
                Button("Clear Recent") {
                    model.clearRecentFiles()
                }
            }
        } label: {
            Label("Recent", systemImage: "clock.arrow.circlepath")
                .font(.system(size: 12, weight: .semibold))
                .padding(.horizontal, 11)
                .padding(.vertical, 6)
                .foregroundColor(.white)
                .background(Theme.accent)
                .clipShape(RoundedRectangle(cornerRadius: 5, style: .continuous))
        }
        .menuStyle(.borderlessButton)
        .menuIndicator(.hidden)
        .fixedSize()
    }

    @ViewBuilder
    private var speechControls: some View {
        let natural = model.voiceOptions.filter { $0.kind == .piper }
        let system = model.voiceOptions.filter { $0.kind == .system }

        Picker("Voice", selection: $model.selectedVoiceID) {
            Text("Default Voice").tag("system:")
            if model.voiceOptions.isEmpty {
                Text("Loading voices…").tag("")
            } else {
                if !natural.isEmpty {
                    Section("Natural") {
                        ForEach(natural) { voice in
                            Text(voice.label).tag(voice.id)
                        }
                    }
                }
                if !system.isEmpty {
                    Section("System voices") {
                        ForEach(system) { voice in
                            Text(voice.label).tag(voice.id)
                        }
                    }
                }
            }
        }
        .labelsHidden()
        .pickerStyle(.menu)
        .frame(width: 230)

        HStack(spacing: 6) {
            Text("Rate")
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)
            Slider(value: $model.rate, in: 100...250, step: 1)
                .frame(width: 90)
                .tint(Theme.accent)
            Text("\(Int(model.rate))")
                .font(.system(size: 11, design: .monospaced))
                .foregroundColor(Theme.text)
                .frame(width: 28, alignment: .leading)
        }

        Button {
            model.startSpeaking()
        } label: {
            Label("Speak", systemImage: "play.fill")
        }
        .buttonStyle(MDButtonStyle(color: Theme.green, compact: true))
        .disabled(model.isSpeaking || model.activeText.isEmpty)

        Button {
            model.stopSpeaking()
        } label: {
            Label("Stop", systemImage: "stop.fill")
        }
        .buttonStyle(MDButtonStyle(color: Theme.red, compact: true))
        .disabled(!model.isSpeaking)

        if model.isSpeaking {
            ProgressView()
                .controlSize(.small)
                .scaleEffect(0.75)
        }
    }

    // MARK: - Document tabs

    private var documentTabs: some View {
        ScrollView(.horizontal, showsIndicators: false) {
            HStack(spacing: 3) {
                ForEach(model.tabs) { tab in
                    documentTab(tab)
                }
            }
            .padding(.horizontal, 8)
            .padding(.vertical, 4)
        }
        .frame(height: 32)
        .background(Theme.background)
        .overlay(alignment: .bottom) {
            Rectangle().fill(Theme.border).frame(height: 1)
        }
    }

    private func documentTab(_ tab: DocumentTab) -> some View {
        let isActive = tab.id == model.activeTabID

        return HStack(spacing: 6) {
            if tab.isDirty {
                Circle()
                    .fill(Theme.accent)
                    .frame(width: 5, height: 5)
            }
            Text(tab.title)
                .font(.system(size: 11, weight: isActive ? .semibold : .regular))
                .foregroundColor(isActive ? Theme.text : Theme.dimText)
                .lineLimit(1)
                .truncationMode(.middle)
                .frame(maxWidth: 180, alignment: .leading)
            Button {
                model.closeTab(tab.id)
            } label: {
                Image(systemName: "xmark")
                    .font(.system(size: 8, weight: .bold))
            }
            .buttonStyle(.plain)
            .foregroundColor(isActive ? Theme.dimText : Theme.dimText.opacity(0.7))
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(isActive ? Theme.panelAlt : Color.clear)
        .clipShape(RoundedRectangle(cornerRadius: 4, style: .continuous))
        .contentShape(Rectangle())
        .onTapGesture {
            model.selectTab(tab.id)
        }
        .help(tab.path?.path ?? "Untitled")
    }

    // MARK: - Content

    @ViewBuilder
    private var content: some View {
        if model.mode == .editor {
            MacEditor(
                text: Binding(
                    get: { model.activeText },
                    set: { model.updateActiveText($0) }
                ),
                selection: model.editorSelection,
                controller: model.editorController
            )
            .frame(maxWidth: .infinity, maxHeight: .infinity)
        } else {
            ViewerPane(model: model)
                .frame(maxWidth: .infinity, maxHeight: .infinity)
        }
    }

    // MARK: - Status bar

    private var statusBar: some View {
        HStack(spacing: 16) {
            Text(model.statusFileName)
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)
                .lineLimit(1)
                .truncationMode(.middle)
                .layoutPriority(1)

            Spacer(minLength: 8)

            Text("\(model.wordCount) words")
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)

            Text("Line \(model.selectedLine + 1)")
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)

            if !model.readingStatus.isEmpty {
                Text(model.readingStatus)
                    .font(.system(size: 11))
                    .foregroundColor(Theme.accent)
            }
        }
        .padding(.horizontal, 14)
        .padding(.vertical, 6)
        .background(Theme.panel)
        .overlay(alignment: .top) {
            Rectangle().fill(Theme.border).frame(height: 1)
        }
    }

    // MARK: - Drop

    private func handleDrop(_ providers: [NSItemProvider]) -> Bool {
        guard let provider = providers.first else { return false }

        provider.loadDataRepresentation(forTypeIdentifier: UTType.fileURL.identifier) { data, _ in
            guard let data,
                  let url = URL(dataRepresentation: data, relativeTo: nil) else {
                return
            }
            DispatchQueue.main.async {
                model.open(url: url)
            }
        }
        return true
    }
}

struct FindBar: View {
    @ObservedObject var model: AppModel
    @FocusState private var focused: Bool

    var body: some View {
        HStack(spacing: 8) {
            Image(systemName: "magnifyingglass")
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)

            TextField("Find in document", text: $model.findQuery)
                .textFieldStyle(.plain)
                .font(.system(size: 12))
                .focused($focused)
                .onSubmit {
                    model.runFind()
                }

            Toggle("Match case", isOn: $model.findMatchCase)
                .toggleStyle(.checkbox)
                .font(.system(size: 11))
                .onChange(of: model.findMatchCase) { _ in
                    model.runFind()
                }

            Toggle("Whole word", isOn: $model.findWholeWord)
                .toggleStyle(.checkbox)
                .font(.system(size: 11))
                .onChange(of: model.findWholeWord) { _ in
                    model.runFind()
                }

            Text(model.findSummary)
                .font(.system(size: 11))
                .foregroundColor(Theme.dimText)
                .frame(minWidth: 68, alignment: .trailing)

            Button {
                model.findPrevious()
            } label: {
                Image(systemName: "chevron.up")
            }
            .buttonStyle(MDSmallButtonStyle())
            .disabled(model.findMatches.isEmpty)

            Button {
                model.findNext()
            } label: {
                Image(systemName: "chevron.down")
            }
            .buttonStyle(MDSmallButtonStyle())
            .disabled(model.findMatches.isEmpty)

            Button {
                model.hideFind()
            } label: {
                Image(systemName: "xmark")
            }
            .buttonStyle(MDSmallButtonStyle())
        }
        .padding(.horizontal, 12)
        .padding(.vertical, 7)
        .background(Theme.panelAlt)
        .overlay(alignment: .bottom) {
            Rectangle().fill(Theme.border).frame(height: 1)
        }
        .onAppear {
            focused = true
        }
        .onExitCommand {
            model.hideFind()
        }
        .onChange(of: model.findQuery) { _ in
            model.runFind()
        }
    }
}
