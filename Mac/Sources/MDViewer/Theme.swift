import SwiftUI
import AppKit

enum Theme {
    static let background = Color(hex: 0x1E262E)
    static let panel = Color(hex: 0x252D35)
    static let panelAlt = Color(hex: 0x2D3742)
    static let border = Color(hex: 0x3A4550)
    static let text = Color(hex: 0xD4D4D4)
    static let dimText = Color(hex: 0x8A949E)
    static let accent = Color(hex: 0x1177BB)
    static let accentDim = Color(hex: 0x0E639C)
    static let green = Color(hex: 0x28A745)
    static let red = Color(hex: 0xDC3545)
    static let codeBackground = Color(hex: 0x161C22)
    static let highlight = Color(hex: 0x1177BB).opacity(0.22)
    static let selection = Color(hex: 0x1177BB).opacity(0.12)
}

extension Color {
    init(hex: UInt32, alpha: Double = 1.0) {
        let r = Double((hex >> 16) & 0xFF) / 255.0
        let g = Double((hex >> 8) & 0xFF) / 255.0
        let b = Double(hex & 0xFF) / 255.0
        self.init(.sRGB, red: r, green: g, blue: b, opacity: alpha)
    }
}

extension NSColor {
    convenience init(hex: UInt32, alpha: CGFloat = 1.0) {
        let r = CGFloat((hex >> 16) & 0xFF) / 255.0
        let g = CGFloat((hex >> 8) & 0xFF) / 255.0
        let b = CGFloat(hex & 0xFF) / 255.0
        self.init(srgbRed: r, green: g, blue: b, alpha: alpha)
    }
}

struct MDButtonStyle: ButtonStyle {
    var color: Color = Theme.accent
    var compact: Bool = false

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: compact ? 11 : 12, weight: .semibold))
            .padding(.horizontal, compact ? 8 : 11)
            .padding(.vertical, compact ? 4 : 6)
            .foregroundColor(.white)
            .background(configuration.isPressed ? color.opacity(0.72) : color)
            .clipShape(RoundedRectangle(cornerRadius: 5, style: .continuous))
            .opacity(configuration.isPressed ? 0.9 : 1.0)
    }
}

struct MDSmallButtonStyle: ButtonStyle {
    var color: Color = Theme.panelAlt

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .font(.system(size: 11, weight: .medium))
            .padding(.horizontal, 8)
            .padding(.vertical, 3)
            .foregroundColor(Theme.text)
            .background(configuration.isPressed ? color.opacity(0.6) : color)
            .clipShape(RoundedRectangle(cornerRadius: 4, style: .continuous))
    }
}
