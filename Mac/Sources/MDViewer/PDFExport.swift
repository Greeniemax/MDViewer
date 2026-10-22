import Foundation
import AppKit

/// Renders a markdown document into a paginated PDF using AppKit's print
/// pipeline. The PDF is generated entirely offline: blocks are turned into a
/// styled `NSAttributedString`, laid out in an offscreen `NSTextView`, and
/// printed into PDF data.
enum PDFExporter {

    enum ExportError: LocalizedError {
        case couldNotRender

        var errorDescription: String? {
            "The document could not be rendered as a PDF."
        }
    }

    private static let bodyFontSize: CGFloat = 11.5
    private static let pageWidth: CGFloat = 595.28   // A4 width in points
    private static let pageHeight: CGFloat = 841.89  // A4 height in points
    private static let margin: CGFloat = 48

    static func writePDF(markdown: String, to url: URL) throws {
        let attributed = attributedString(for: markdown)

        let info = NSPrintInfo.shared.copy() as! NSPrintInfo
        info.paperSize = NSSize(width: pageWidth, height: pageHeight)
        info.topMargin = margin
        info.bottomMargin = margin
        info.leftMargin = margin
        info.rightMargin = margin
        info.orientation = .portrait
        info.horizontalPagination = .fit
        info.verticalPagination = .automatic
        info.isHorizontallyCentered = false
        info.isVerticallyCentered = false

        let contentWidth = pageWidth - margin * 2
        let contentHeight = pageHeight - margin * 2

        let textView = NSTextView(frame: NSRect(x: 0, y: 0, width: contentWidth, height: contentHeight))
        textView.appearance = NSAppearance(named: .aqua)
        textView.isEditable = false
        textView.isSelectable = false
        textView.drawsBackground = false
        textView.textContainerInset = .zero
        textView.textContainer?.lineFragmentPadding = 0
        textView.textContainer?.widthTracksTextView = true
        textView.textContainer?.containerSize = NSSize(
            width: contentWidth,
            height: CGFloat.greatestFiniteMagnitude
        )
        textView.isVerticallyResizable = true
        textView.isHorizontallyResizable = false
        textView.minSize = NSSize(width: contentWidth, height: 0)
        textView.maxSize = NSSize(width: contentWidth, height: CGFloat.greatestFiniteMagnitude)
        textView.textStorage?.setAttributedString(attributed)

        if let layout = textView.layoutManager, let container = textView.textContainer {
            layout.ensureLayout(for: container)
            let used = layout.usedRect(for: container)
            let height = max(contentHeight, used.height.rounded(.up) + 4)
            textView.frame = NSRect(x: 0, y: 0, width: contentWidth, height: height)
            layout.ensureLayout(for: container)
        }

        let data = NSMutableData()
        let operation = NSPrintOperation.pdfOperation(
            with: textView,
            inside: textView.bounds,
            to: data,
            printInfo: info
        )
        operation.showsPrintPanel = false
        operation.showsProgressPanel = false
        guard operation.run() else {
            throw ExportError.couldNotRender
        }

        try (data as Data).write(to: url, options: .atomic)
    }

    // MARK: - Attributed string

    private static func attributedString(for markdown: String) -> NSAttributedString {
        let result = NSMutableAttributedString()
        let blocks = MarkdownParser.parse(markdown)
        let bodyFont = NSFont.systemFont(ofSize: bodyFontSize)
        let bodyColor = NSColor(calibratedWhite: 0.10, alpha: 1)

        for block in blocks {
            switch block.kind {
            case .heading(let level):
                let sizes: [CGFloat] = [24, 20, 16.5, 14, 12.5, 11.5]
                let size = sizes[max(0, min(level - 1, sizes.count - 1))]
                let font = NSFont.systemFont(ofSize: size, weight: .bold)
                let style = baseStyle()
                style.paragraphSpacingBefore = level == 1 ? 14 : 10
                style.paragraphSpacing = 5
                appendInline(
                    block.text,
                    baseFont: font,
                    color: NSColor(calibratedWhite: 0.05, alpha: 1),
                    paragraph: style,
                    to: result
                )
                result.append(NSAttributedString(
                    string: "\n",
                    attributes: [.font: font, .paragraphStyle: style]
                ))

            case .paragraph:
                let style = baseStyle()
                appendInline(block.text, baseFont: bodyFont, color: bodyColor, paragraph: style, to: result)
                result.append(NSAttributedString(
                    string: "\n",
                    attributes: [.font: bodyFont, .paragraphStyle: style]
                ))

            case .bullet(let depth):
                let level = max(0, min(depth, 5))
                let indent = CGFloat(level) * 16
                let style = baseStyle()
                style.headIndent = indent + 16
                style.firstLineHeadIndent = indent
                style.tabStops = [NSTextTab(textAlignment: .left, location: indent + 16, options: [:])]
                let attributes: [NSAttributedString.Key: Any] = [
                    .font: bodyFont,
                    .foregroundColor: bodyColor,
                    .paragraphStyle: style
                ]
                result.append(NSAttributedString(string: "•\t", attributes: attributes))
                appendInline(block.text, baseFont: bodyFont, color: bodyColor, paragraph: style, to: result)
                result.append(NSAttributedString(string: "\n", attributes: attributes))

            case .numbered(let number):
                let style = baseStyle()
                style.headIndent = 24
                style.firstLineHeadIndent = 0
                style.tabStops = [NSTextTab(textAlignment: .left, location: 24, options: [:])]
                let attributes: [NSAttributedString.Key: Any] = [
                    .font: bodyFont,
                    .foregroundColor: bodyColor,
                    .paragraphStyle: style
                ]
                result.append(NSAttributedString(string: "\(number).\t", attributes: attributes))
                appendInline(block.text, baseFont: bodyFont, color: bodyColor, paragraph: style, to: result)
                result.append(NSAttributedString(string: "\n", attributes: attributes))

            case .quote:
                let style = baseStyle()
                style.headIndent = 18
                style.firstLineHeadIndent = 18
                style.paragraphSpacing = 8
                let font = NSFontManager.shared.convert(bodyFont, toHaveTrait: .italicFontMask)
                let color = NSColor(calibratedWhite: 0.35, alpha: 1)
                appendInline(block.text, baseFont: font, color: color, paragraph: style, to: result)
                result.append(NSAttributedString(
                    string: "\n",
                    attributes: [.font: font, .paragraphStyle: style]
                ))

            case .code(let language):
                let font = NSFont.monospacedSystemFont(ofSize: 9.5, weight: .regular)
                let style = baseStyle()
                style.lineSpacing = 1
                style.paragraphSpacing = 8
                style.headIndent = 10
                style.firstLineHeadIndent = 10

                if !language.isEmpty {
                    let labelFont = NSFont.systemFont(ofSize: 8.5, weight: .semibold)
                    result.append(NSAttributedString(
                        string: language.uppercased() + "\n",
                        attributes: [
                            .font: labelFont,
                            .foregroundColor: NSColor(calibratedWhite: 0.45, alpha: 1),
                            .paragraphStyle: style
                        ]
                    ))
                }

                let code = block.text.isEmpty ? " " : block.text
                result.append(NSAttributedString(
                    string: code + "\n",
                    attributes: [
                        .font: font,
                        .foregroundColor: NSColor(calibratedWhite: 0.12, alpha: 1),
                        .backgroundColor: NSColor(calibratedWhite: 0.95, alpha: 1),
                        .paragraphStyle: style
                    ]
                ))

            case .table(let rows):
                appendTable(rows, bodyColor: bodyColor, to: result)

            case .rule:
                let style = baseStyle()
                style.paragraphSpacing = 10
                let font = NSFont.monospacedSystemFont(ofSize: 9, weight: .regular)
                result.append(NSAttributedString(
                    string: String(repeating: "─", count: 80) + "\n",
                    attributes: [
                        .font: font,
                        .foregroundColor: NSColor(calibratedWhite: 0.72, alpha: 1),
                        .paragraphStyle: style
                    ]
                ))
            }
        }

        return result
    }

    private static func baseStyle() -> NSMutableParagraphStyle {
        let style = NSMutableParagraphStyle()
        style.lineSpacing = 3
        style.paragraphSpacing = 7
        return style
    }

    private static func appendTable(
        _ rows: [[String]],
        bodyColor: NSColor,
        to result: NSMutableAttributedString
    ) {
        let columnCount = rows.map { $0.count }.max() ?? 0
        guard columnCount > 0 else { return }

        var widths = Array(repeating: 3, count: columnCount)
        for row in rows {
            for (index, cell) in row.enumerated() where index < columnCount {
                widths[index] = max(widths[index], cell.count)
            }
        }

        let style = baseStyle()
        style.lineSpacing = 1
        style.paragraphSpacing = 0

        for (rowIndex, row) in rows.enumerated() {
            let cells = (0..<columnCount).map { index -> String in
                let cell = index < row.count ? row[index] : ""
                return cell.padding(toLength: widths[index], withPad: " ", startingAt: 0)
            }
            let font = NSFont.monospacedSystemFont(
                ofSize: 9,
                weight: rowIndex == 0 ? .semibold : .regular
            )
            result.append(NSAttributedString(
                string: cells.joined(separator: " │ ") + "\n",
                attributes: [
                    .font: font,
                    .foregroundColor: bodyColor,
                    .paragraphStyle: style
                ]
            ))

            if rowIndex == 0 {
                let separator = widths
                    .map { String(repeating: "─", count: $0) }
                    .joined(separator: "─┼─")
                result.append(NSAttributedString(
                    string: separator + "\n",
                    attributes: [
                        .font: font,
                        .foregroundColor: NSColor(calibratedWhite: 0.70, alpha: 1),
                        .paragraphStyle: style
                    ]
                ))
            }
        }

        result.append(NSAttributedString(
            string: "\n",
            attributes: [.font: NSFont.systemFont(ofSize: bodyFontSize), .paragraphStyle: baseStyle()]
        ))
    }

    /// Converts inline markdown (bold/italic/code/links/strikethrough) into
    /// attributes while keeping the paragraph style supplied by the caller.
    private static func appendInline(
        _ markdown: String,
        baseFont: NSFont,
        color: NSColor,
        paragraph: NSParagraphStyle,
        to result: NSMutableAttributedString
    ) {
        let options = AttributedString.MarkdownParsingOptions(
            interpretedSyntax: .inlineOnlyPreservingWhitespace
        )

        guard let parsed = try? AttributedString(markdown: markdown, options: options) else {
            result.append(NSAttributedString(
                string: markdown,
                attributes: [.font: baseFont, .foregroundColor: color, .paragraphStyle: paragraph]
            ))
            return
        }

        for run in parsed.runs {
            let text = String(parsed[run.range].characters)
            var font = baseFont

            if let intent = run.inlinePresentationIntent {
                if intent.contains(.stronglyEmphasized) {
                    font = NSFontManager.shared.convert(font, toHaveTrait: .boldFontMask)
                }
                if intent.contains(.emphasized) {
                    font = NSFontManager.shared.convert(font, toHaveTrait: .italicFontMask)
                }
                if intent.contains(.code) {
                    font = NSFont.monospacedSystemFont(
                        ofSize: max(8, baseFont.pointSize - 1),
                        weight: .regular
                    )
                }
            }

            var attributes: [NSAttributedString.Key: Any] = [
                .font: font,
                .foregroundColor: color,
                .paragraphStyle: paragraph
            ]

            if let intent = run.inlinePresentationIntent, intent.contains(.strikethrough) {
                attributes[.strikethroughStyle] = NSUnderlineStyle.single.rawValue
            }
            if let link = run.link {
                attributes[.link] = link
                attributes[.foregroundColor] = NSColor.linkColor
                attributes[.underlineStyle] = NSUnderlineStyle.single.rawValue
            }

            result.append(NSAttributedString(string: text, attributes: attributes))
        }
    }
}
