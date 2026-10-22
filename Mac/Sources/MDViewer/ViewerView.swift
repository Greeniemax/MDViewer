import SwiftUI

struct ViewerPane: View {
    @ObservedObject var model: AppModel

    var body: some View {
        Group {
            if model.activeText.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
                emptyState
            } else {
                renderedDocument
            }
        }
        .background(Theme.background)
    }

    private var renderedDocument: some View {
        ScrollViewReader { proxy in
            ScrollView {
                LazyVStack(alignment: .leading, spacing: 10) {
                    ForEach(model.blocks) { block in
                        MarkdownBlockView(
                            block: block,
                            isSelected: block.sourceLine == model.selectedLine,
                            isHighlighted: isHighlighted(block)
                        )
                        .id(block.id)
                        // simultaneousGesture (rather than onTapGesture) keeps
                        // click-to-set-position working even though the block's
                        // text is selectable.
                        .simultaneousGesture(
                            TapGesture().onEnded {
                                model.selectedLine = block.sourceLine
                            }
                        )
                    }
                }
                .padding(.horizontal, 36)
                .padding(.vertical, 28)
                .frame(maxWidth: 940, alignment: .leading)
                .frame(maxWidth: .infinity, alignment: .center)
            }
            .onChange(of: model.highlightedLine) { line in
                guard
                    let line,
                    let block = model.blocks.first(where: {
                        $0.sourceLine <= line && line <= $0.sourceEndLine
                    })
                else { return }

                withAnimation(.easeInOut(duration: 0.25)) {
                    proxy.scrollTo(block.id, anchor: .center)
                }
            }
        }
    }

    private var emptyState: some View {
        VStack(spacing: 10) {
            Image(systemName: "doc.richtext")
                .font(.system(size: 42, weight: .light))
                .foregroundColor(Theme.dimText.opacity(0.6))
            Text("No file loaded")
                .font(.system(size: 16, weight: .semibold))
                .foregroundColor(Theme.text)
            Text("Open a markdown file or start typing in the editor.")
                .font(.system(size: 12))
                .foregroundColor(Theme.dimText)
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity)
    }

    private func isHighlighted(_ block: MarkdownBlock) -> Bool {
        guard let line = model.highlightedLine else { return false }
        return block.sourceLine <= line && line <= block.sourceEndLine
    }
}

struct MarkdownBlockView: View {
    let block: MarkdownBlock
    let isSelected: Bool
    let isHighlighted: Bool

    var body: some View {
        HStack(alignment: .top, spacing: 10) {
            RoundedRectangle(cornerRadius: 1.5, style: .continuous)
                .fill(isSelected ? Theme.accent : Color.clear)
                .frame(width: 3)
                .padding(.vertical, 2)

            blockBody
        }
        .padding(.horizontal, 10)
        .padding(.vertical, 5)
        .background(isHighlighted ? Theme.highlight : Color.clear)
        .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
        .animation(.easeInOut(duration: 0.2), value: isHighlighted)
        .animation(.easeInOut(duration: 0.2), value: isSelected)
    }

    @ViewBuilder
    private var blockBody: some View {
        switch block.kind {
        case .heading(let level):
            heading(level)
        case .paragraph:
            paragraph
        case .bullet(let depth):
            bullet(depth)
        case .numbered(let number):
            numbered(number)
        case .quote:
            quote
        case .code(let language):
            code(language)
        case .table(let rows):
            table(rows)
        case .rule:
            Rectangle()
                .fill(Theme.border)
                .frame(height: 1)
                .padding(.vertical, 10)
        }
    }

    private var paragraph: some View {
        textBody
    }

    private func heading(_ level: Int) -> some View {
        let sizes: [Int: CGFloat] = [1: 30, 2: 24, 3: 20, 4: 17, 5: 15, 6: 13]
        return Text(inline(block.text))
            .font(.system(size: sizes[level] ?? 16, weight: .bold))
            .foregroundColor(Theme.text)
            .padding(.top, level == 1 ? 8 : 3)
            .padding(.bottom, 2)
            .textSelection(.enabled)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func bullet(_ depth: Int) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Text("•")
                .font(.system(size: 15, weight: .semibold))
                .foregroundColor(Theme.dimText)
            textBody
        }
        .padding(.leading, CGFloat(depth) * 16)
    }

    private func numbered(_ number: Int) -> some View {
        HStack(alignment: .firstTextBaseline, spacing: 8) {
            Text("\(number).")
                .font(.system(size: 14, design: .monospaced))
                .foregroundColor(Theme.dimText)
                .frame(minWidth: 24, alignment: .trailing)
            textBody
        }
    }

    private var quote: some View {
        HStack(alignment: .top, spacing: 12) {
            RoundedRectangle(cornerRadius: 1.5, style: .continuous)
                .fill(Theme.accent.opacity(0.7))
                .frame(width: 3)
            Text(inline(block.text))
                .font(.system(size: 15))
                .italic()
                .foregroundColor(Theme.dimText)
                .lineSpacing(4)
                .textSelection(.enabled)
                .fixedSize(horizontal: false, vertical: true)
                .frame(maxWidth: .infinity, alignment: .leading)
        }
        .padding(.vertical, 4)
        .padding(.leading, 4)
    }

    private func code(_ language: String) -> some View {
        VStack(alignment: .leading, spacing: 7) {
            if !language.isEmpty {
                Text(language.uppercased())
                    .font(.system(size: 10, weight: .semibold))
                    .foregroundColor(Theme.dimText)
            }
            ScrollView(.horizontal, showsIndicators: false) {
                Text(block.text)
                    .font(.system(size: 13, design: .monospaced))
                    .foregroundColor(Color(hex: 0xC9D1D9))
                    .textSelection(.enabled)
                    .fixedSize(horizontal: true, vertical: false)
            }
        }
        .padding(12)
        .frame(maxWidth: .infinity, alignment: .leading)
        .background(Theme.codeBackground)
        .clipShape(RoundedRectangle(cornerRadius: 7, style: .continuous))
    }

    private func table(_ rows: [[String]]) -> some View {
        VStack(alignment: .leading, spacing: 0) {
            ForEach(Array(rows.enumerated()), id: \.offset) { pair in
                let rowIndex = pair.offset
                let row = pair.element

                HStack(alignment: .top, spacing: 16) {
                    ForEach(Array(row.enumerated()), id: \.offset) { cellPair in
                        Text(inline(cellPair.element))
                            .font(.system(
                                size: 14,
                                weight: rowIndex == 0 ? .semibold : .regular
                            ))
                            .foregroundColor(Theme.text)
                            .textSelection(.enabled)
                            .fixedSize(horizontal: false, vertical: true)
                            .frame(maxWidth: .infinity, alignment: .leading)
                    }
                }
                .padding(.vertical, 7)
                .padding(.horizontal, 10)
                .background(rowIndex == 0 ? Theme.panelAlt.opacity(0.7) : Color.clear)

                if rowIndex < rows.count - 1 {
                    Rectangle()
                        .fill(Theme.border)
                        .frame(height: 1)
                }
            }
        }
        .background(Theme.panelAlt.opacity(0.22))
        .clipShape(RoundedRectangle(cornerRadius: 6, style: .continuous))
        .overlay(
            RoundedRectangle(cornerRadius: 6, style: .continuous)
                .stroke(Theme.border, lineWidth: 0.5)
        )
    }

    private var textBody: some View {
        Text(inline(block.text))
            .font(.system(size: 15))
            .foregroundColor(Theme.text)
            .lineSpacing(4)
            .textSelection(.enabled)
            .tint(Theme.accent)
            .fixedSize(horizontal: false, vertical: true)
            .frame(maxWidth: .infinity, alignment: .leading)
    }

    private func inline(_ text: String) -> AttributedString {
        let options = AttributedString.MarkdownParsingOptions(
            interpretedSyntax: .inlineOnlyPreservingWhitespace
        )
        return (try? AttributedString(markdown: text, options: options))
            ?? AttributedString(text)
    }
}
