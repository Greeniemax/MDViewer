import Foundation

struct MarkdownBlock: Identifiable, Equatable {
    enum Kind: Equatable {
        case heading(Int)
        case paragraph
        case bullet(Int)
        case numbered(Int)
        case quote
        case code(String)
        case table([[String]])
        case rule
    }

    let id = UUID()
    let kind: Kind
    let text: String
    let sourceLine: Int
    let sourceEndLine: Int
}

enum MarkdownParser {
    static func parse(_ source: String) -> [MarkdownBlock] {
        let lines = source.components(separatedBy: "\n")
        var blocks: [MarkdownBlock] = []
        var i = 0

        while i < lines.count {
            let raw = lines[i]
            let trimmed = raw.trimmingCharacters(in: .whitespaces)
            if trimmed.isEmpty {
                i += 1
                continue
            }
            let start = i

            // Fenced code block.
            if trimmed.hasPrefix("```") {
                let language = String(trimmed.dropFirst(3))
                    .trimmingCharacters(in: .whitespaces)
                var codeLines: [String] = []
                i += 1
                while i < lines.count {
                    let t = lines[i].trimmingCharacters(in: .whitespaces)
                    if t.hasPrefix("```") { break }
                    codeLines.append(lines[i])
                    i += 1
                }
                if i < lines.count { i += 1 }
                blocks.append(MarkdownBlock(
                    kind: .code(language),
                    text: codeLines.joined(separator: "\n"),
                    sourceLine: start,
                    sourceEndLine: max(start, i - 1)
                ))
                continue
            }

            // Heading.
            if let level = headingLevel(trimmed) {
                let content = trimmed.replacingOccurrences(
                    of: #"^#{1,6}\s+"#,
                    with: "",
                    options: .regularExpression
                )
                blocks.append(MarkdownBlock(
                    kind: .heading(level),
                    text: content,
                    sourceLine: start,
                    sourceEndLine: start
                ))
                i += 1
                continue
            }

            // Horizontal rule.
            if isRule(trimmed) {
                blocks.append(MarkdownBlock(kind: .rule, text: "", sourceLine: start, sourceEndLine: start))
                i += 1
                continue
            }

            // Table (header line followed by a separator line).
            if trimmed.contains("|"), i + 1 < lines.count, isTableSeparator(lines[i + 1]) {
                var rows: [[String]] = [splitRow(trimmed)]
                i += 2
                while i < lines.count {
                    let t = lines[i].trimmingCharacters(in: .whitespaces)
                    if t.isEmpty || !t.contains("|") { break }
                    rows.append(splitRow(t))
                    i += 1
                }
                blocks.append(MarkdownBlock(
                    kind: .table(rows),
                    text: "",
                    sourceLine: start,
                    sourceEndLine: max(start, i - 1)
                ))
                continue
            }

            // Block quote.
            if trimmed.hasPrefix(">") {
                var parts: [String] = []
                while i < lines.count {
                    let t = lines[i].trimmingCharacters(in: .whitespaces)
                    guard t.hasPrefix(">") else { break }
                    let body = t.replacingOccurrences(
                        of: #"^>\s?"#,
                        with: "",
                        options: .regularExpression
                    )
                    parts.append(body)
                    i += 1
                }
                blocks.append(MarkdownBlock(
                    kind: .quote,
                    text: parts.joined(separator: " "),
                    sourceLine: start,
                    sourceEndLine: max(start, i - 1)
                ))
                continue
            }

            // Bullet list item.
            if let bullet = bulletInfo(raw) {
                blocks.append(MarkdownBlock(
                    kind: .bullet(bullet.depth),
                    text: bullet.text,
                    sourceLine: start,
                    sourceEndLine: start
                ))
                i += 1
                continue
            }

            // Numbered list item.
            if let numbered = numberedInfo(raw) {
                blocks.append(MarkdownBlock(
                    kind: .numbered(numbered.number),
                    text: numbered.text,
                    sourceLine: start,
                    sourceEndLine: start
                ))
                i += 1
                continue
            }

            // Paragraph: gather until a blank line or another block starts.
            var parts: [String] = [trimmed]
            i += 1
            while i < lines.count {
                let next = lines[i]
                if isBlockStart(next) { break }
                parts.append(next.trimmingCharacters(in: .whitespaces))
                i += 1
            }
            blocks.append(MarkdownBlock(
                kind: .paragraph,
                text: parts.joined(separator: " "),
                sourceLine: start,
                sourceEndLine: max(start, i - 1)
            ))
        }

        return blocks
    }

    // MARK: - Helpers

    private static func headingLevel(_ line: String) -> Int? {
        guard let range = line.range(of: #"^(#{1,6})\s+"#, options: .regularExpression) else {
            return nil
        }
        return line[range].prefix(while: { $0 == "#" }).count
    }

    private static func isRule(_ line: String) -> Bool {
        line.range(
            of: #"^\s*([-*_])(?:\s*\1){2,}\s*$"#,
            options: .regularExpression
        ) != nil
    }

    private static func isTableSeparator(_ line: String) -> Bool {
        let t = line.trimmingCharacters(in: .whitespaces)
        guard t.contains("-") else { return false }
        return t.range(
            of: #"^\s*\|?[\s:|-]+\|?\s*$"#,
            options: .regularExpression
        ) != nil
    }

    private static func isBlockStart(_ line: String) -> Bool {
        let t = line.trimmingCharacters(in: .whitespaces)
        if t.isEmpty { return true }
        if t.hasPrefix("```") { return true }
        if t.hasPrefix(">") { return true }
        if t.contains("|") { return true }
        if headingLevel(t) != nil { return true }
        if t.range(of: #"^\s*[-*+]\s+"#, options: .regularExpression) != nil { return true }
        if t.range(of: #"^\s*\d+\.\s+"#, options: .regularExpression) != nil { return true }
        if isRule(t) { return true }
        return false
    }

    private static func bulletInfo(_ line: String) -> (depth: Int, text: String)? {
        guard let range = line.range(of: #"^\s*[-*+]\s+"#, options: .regularExpression) else {
            return nil
        }
        let indent = line.prefix(while: { $0 == " " || $0 == "\t" }).count
        let body = String(line[range.upperBound...]).trimmingCharacters(in: .whitespaces)
        return (indent / 2, body)
    }

    private static let numberedRegex = try! NSRegularExpression(
        pattern: #"^(\s*)(\d+)\.\s+(.*)$"#
    )

    private static func numberedInfo(_ line: String) -> (number: Int, depth: Int, text: String)? {
        let ns = line as NSString
        guard let match = numberedRegex.firstMatch(
            in: line,
            range: NSRange(location: 0, length: ns.length)
        ) else { return nil }

        guard
            let numberRange = Range(match.range(at: 2), in: line),
            let number = Int(line[numberRange]),
            let textRange = Range(match.range(at: 3), in: line)
        else { return nil }

        let indent = line.prefix(while: { $0 == " " || $0 == "\t" }).count
        return (number, indent / 2, String(line[textRange]))
    }

    private static func splitRow(_ line: String) -> [String] {
        var t = line.trimmingCharacters(in: .whitespaces)
        if t.hasPrefix("|") { t.removeFirst() }
        if t.hasSuffix("|") { t.removeLast() }
        return t.split(separator: "|", omittingEmptySubsequences: false).map {
            String($0).trimmingCharacters(in: .whitespaces)
        }
    }
}
