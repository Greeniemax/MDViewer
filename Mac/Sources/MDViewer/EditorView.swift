import SwiftUI
import AppKit

final class EditorController: ObservableObject {
    weak var textView: NSTextView?

    func focus() {
        guard let textView else { return }
        textView.window?.makeFirstResponder(textView)
    }
}

struct MacEditor: NSViewRepresentable {
    @Binding var text: String
    var selection: EditorSelection?
    var controller: EditorController

    func makeCoordinator() -> Coordinator {
        Coordinator(parent: self)
    }

    func makeNSView(context: Context) -> NSScrollView {
        let scrollView = NSScrollView()
        scrollView.hasVerticalScroller = true
        scrollView.hasHorizontalScroller = false
        scrollView.autohidesScrollers = true
        scrollView.borderType = .noBorder
        scrollView.drawsBackground = true
        scrollView.backgroundColor = NSColor(hex: 0x1E262E)
        scrollView.scrollerStyle = .overlay
        scrollView.verticalScrollElasticity = .allowed

        let initialSize = NSSize(width: 800, height: 600)
        let textView = NSTextView(frame: NSRect(origin: .zero, size: initialSize))
        textView.isRichText = false
        textView.isEditable = true
        textView.isSelectable = true
        textView.allowsUndo = true
        textView.isVerticallyResizable = true
        textView.isHorizontallyResizable = false
        textView.autoresizingMask = [.width]
        textView.textContainerInset = NSSize(width: 12, height: 12)
        textView.textContainer?.lineFragmentPadding = 6
        textView.textContainer?.widthTracksTextView = true
        textView.textContainer?.containerSize = NSSize(
            width: initialSize.width,
            height: CGFloat.greatestFiniteMagnitude
        )
        textView.minSize = NSSize(width: 0, height: initialSize.height)
        textView.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)

        let font = NSFont.monospacedSystemFont(ofSize: 14, weight: .regular)
        textView.font = font
        textView.textColor = NSColor(hex: 0xD4D4D4)
        textView.backgroundColor = NSColor(hex: 0x1E262E)
        textView.insertionPointColor = NSColor(hex: 0xD4D4D4)
        textView.selectedTextAttributes = [
            .backgroundColor: NSColor(hex: 0x1177BB, alpha: 0.45),
            .foregroundColor: NSColor.white
        ]
        textView.typingAttributes = [
            .font: font,
            .foregroundColor: NSColor(hex: 0xD4D4D4)
        ]
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.isAutomaticDashSubstitutionEnabled = false
        textView.isAutomaticTextReplacementEnabled = false
        textView.isAutomaticSpellingCorrectionEnabled = false
        textView.isContinuousSpellCheckingEnabled = false
        textView.usesFindBar = false
        textView.isIncrementalSearchingEnabled = false
        textView.string = text
        textView.delegate = context.coordinator

        scrollView.documentView = textView

        let ruler = LineNumberRulerView(textView: textView)
        scrollView.verticalRulerView = ruler
        scrollView.hasVerticalRuler = true
        scrollView.rulersVisible = true

        scrollView.contentView.postsBoundsChangedNotifications = true
        NotificationCenter.default.addObserver(
            context.coordinator,
            selector: #selector(Coordinator.boundsChanged(_:)),
            name: NSView.boundsDidChangeNotification,
            object: scrollView.contentView
        )

        context.coordinator.textView = textView
        context.coordinator.ruler = ruler
        controller.textView = textView
        return scrollView
    }

    func updateNSView(_ scrollView: NSScrollView, context: Context) {
        guard let textView = scrollView.documentView as? NSTextView else { return }
        context.coordinator.parent = self
        controller.textView = textView

        if textView.string != text {
            textView.string = text
            textView.setSelectedRange(NSRange(location: 0, length: 0))
            textView.scrollRangeToVisible(NSRange(location: 0, length: 0))
            // A single text view is reused for all tabs. Clearing the undo
            // stack prevents an undo from restoring text from another tab.
            textView.undoManager?.removeAllActions()
            context.coordinator.ruler?.needsDisplay = true
        }

        if let selection, context.coordinator.appliedSelectionToken != selection.token {
            context.coordinator.appliedSelectionToken = selection.token
            let length = (text as NSString).length
            let location = max(0, min(selection.range.location, length))
            let selectedLength = min(selection.range.length, length - location)
            let safeRange = NSRange(location: location, length: max(0, selectedLength))
            textView.setSelectedRange(safeRange)
            textView.scrollRangeToVisible(safeRange)
        }
    }

    static func dismantleNSView(_ nsView: NSScrollView, coordinator: Coordinator) {
        NotificationCenter.default.removeObserver(coordinator)
        if coordinator.parent.controller.textView === coordinator.textView {
            coordinator.parent.controller.textView = nil
        }
    }

    final class Coordinator: NSObject, NSTextViewDelegate {
        var parent: MacEditor
        weak var textView: NSTextView?
        weak var ruler: LineNumberRulerView?
        var appliedSelectionToken = -1

        init(parent: MacEditor) {
            self.parent = parent
        }

        deinit {
            NotificationCenter.default.removeObserver(self)
        }

        func textDidChange(_ notification: Notification) {
            guard let textView = notification.object as? NSTextView else { return }
            parent.text = textView.string
            ruler?.needsDisplay = true
        }

        @objc func boundsChanged(_ notification: Notification) {
            ruler?.needsDisplay = true
        }
    }
}

final class LineNumberRulerView: NSRulerView {
    private weak var textView: NSTextView?
    private let numberFont = NSFont.monospacedSystemFont(ofSize: 10, weight: .regular)
    private let numberColor = NSColor(hex: 0x6E7A86)

    init(textView: NSTextView) {
        self.textView = textView
        super.init(scrollView: textView.enclosingScrollView, orientation: .verticalRuler)
        clientView = textView
        ruleThickness = 44
    }

    required init(coder: NSCoder) {
        fatalError("init(coder:) has not been implemented")
    }

    override func drawHashMarksAndLabels(in rect: NSRect) {
        NSColor(hex: 0x252D35).setFill()
        NSBezierPath(rect: bounds).fill()

        NSColor(hex: 0x3A4550).setFill()
        NSBezierPath(rect: NSRect(x: bounds.width - 1, y: 0, width: 1, height: bounds.height)).fill()

        guard
            let textView,
            let layoutManager = textView.layoutManager,
            let container = textView.textContainer
        else { return }

        let attributes: [NSAttributedString.Key: Any] = [
            .font: numberFont,
            .foregroundColor: numberColor
        ]

        // Position of the text view's origin in the ruler's coordinate space.
        // This accounts for both scroll offset and the text view's flipped
        // coordinate system.
        let relativeOrigin = convert(NSPoint.zero, from: textView)

        let content = textView.string as NSString
        if content.length == 0 {
            drawNumber(
                1,
                attributes: attributes,
                y: relativeOrigin.y + textView.textContainerInset.height
            )
            return
        }

        let visibleRect = textView.visibleRect
        let glyphRange = layoutManager.glyphRange(forBoundingRect: visibleRect, in: container)
        let charRange = layoutManager.characterRange(forGlyphRange: glyphRange, actualGlyphRange: nil)

        var index = 0
        var number = 1

        while index < charRange.location {
            let lineRange = content.lineRange(for: NSRange(location: index, length: 0))
            let next = NSMaxRange(lineRange)
            if next <= index { break }
            index = next
            number += 1
        }

        let end = min(NSMaxRange(charRange), content.length)
        while index < end {
            let lineRange = content.lineRange(for: NSRange(location: index, length: 0))
            let glyph = layoutManager.glyphIndexForCharacter(at: index)
            let fragment = layoutManager.lineFragmentRect(forGlyphAt: glyph, effectiveRange: nil)
            let y = relativeOrigin.y + fragment.minY + textView.textContainerInset.height
            drawNumber(number, attributes: attributes, y: y)

            let next = NSMaxRange(lineRange)
            if next <= index { break }
            index = next
            number += 1
        }
    }

    private func drawNumber(
        _ number: Int,
        attributes: [NSAttributedString.Key: Any],
        y: CGFloat
    ) {
        let label = "\(number)" as NSString
        let size = label.size(withAttributes: attributes)
        let x = bounds.width - size.width - 9
        label.draw(at: NSPoint(x: x, y: y + 1), withAttributes: attributes)
    }
}
