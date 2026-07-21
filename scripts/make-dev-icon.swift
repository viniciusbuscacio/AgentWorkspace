import AppKit

// make-dev-icon.swift <src.png> <out.png>
// Composites the word "DEV" (white, bold) below the glasses on the app icon.
let args = CommandLine.arguments
guard args.count >= 3 else { fputs("usage: make-dev-icon <src.png> <out.png>\n", stderr); exit(2) }
let srcPath = args[1]
let outPath = args[2]

guard let src = NSImage(contentsOfFile: srcPath) else { fputs("cannot load \(srcPath)\n", stderr); exit(1) }
let size = NSSize(width: 1024, height: 1024)

let rep = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: 1024, pixelsHigh: 1024,
                           bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                           colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0)!
NSGraphicsContext.saveGraphicsState()
NSGraphicsContext.current = NSGraphicsContext(bitmapImageRep: rep)

// Base icon
src.draw(in: NSRect(origin: .zero, size: size))

// "DEV" text — white, heavy, tracked, centered horizontally, below the glasses.
let text = "DEV"
let fontSize: CGFloat = 200
let font = NSFont.systemFont(ofSize: fontSize, weight: .heavy)
let para = NSMutableParagraphStyle()
para.alignment = .center
let attrs: [NSAttributedString.Key: Any] = [
  .font: font,
  .foregroundColor: NSColor.white,
  .paragraphStyle: para,
  .kern: 12.0,
]
let attr = NSAttributedString(string: text, attributes: attrs)
let textSize = attr.size()
// Bottom-left origin. Glasses sit near vertical centre (~y 500-560); place DEV
// lower. Center the text box around y≈330 (≈68% down from top).
let cx = (1024 - textSize.width) / 2
let cy: CGFloat = 330 - textSize.height / 2
attr.draw(in: NSRect(x: cx, y: cy, width: textSize.width, height: textSize.height))

NSGraphicsContext.restoreGraphicsState()

guard let png = rep.representation(using: .png, properties: [:]) else { fputs("png encode failed\n", stderr); exit(1) }
do { try png.write(to: URL(fileURLWithPath: outPath)) } catch { fputs("write failed: \(error)\n", stderr); exit(1) }
print("wrote \(outPath)")
