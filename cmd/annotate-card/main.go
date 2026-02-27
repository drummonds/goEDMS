package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

// annotation defines a label and its leader line endpoints.
type annotation struct {
	Text string
	// Label position (top-left of rect)
	LabelX, LabelY float64
	// Leader line: from label edge to target element centre
	LineX1, LineY1 float64 // label edge
	LineX2, LineY2 float64 // target centre
}

// SVG root element for parsing width/height/viewBox.
type svgRoot struct {
	XMLName xml.Name `xml:"svg"`
	Width   string   `xml:"width,attr"`
	Height  string   `xml:"height,attr"`
	ViewBox string   `xml:"viewBox,attr"`
}

func main() {
	input := flag.String("input", "", "input SVG file (captured card)")
	output := flag.String("output", "", "output SVG file (annotated card)")
	flag.Parse()

	if *input == "" || *output == "" {
		flag.Usage()
		os.Exit(1)
	}

	data, err := os.ReadFile(*input)
	if err != nil {
		log.Fatalf("read %s: %v", *input, err)
	}

	// Parse to get dimensions
	var root svgRoot
	if err := xml.Unmarshal(data, &root); err != nil {
		log.Fatalf("parse SVG: %v", err)
	}

	// Parse width/height from the SVG attributes
	cardW, cardH := parseSize(root.Width), parseSize(root.Height)
	if cardW == 0 || cardH == 0 {
		log.Fatal("could not determine card width/height from SVG")
	}

	// Margins around the card for annotation space
	const (
		marginTop    = 40
		marginBottom = 50
		marginLeft   = 20
		marginRight  = 200
	)

	totalW := marginLeft + cardW + marginRight
	totalH := marginTop + cardH + marginBottom

	// Shorthand for float64 conversions
	cw := float64(cardW)
	ch := float64(cardH)
	ml := float64(marginLeft)
	mt := float64(marginTop)

	// Annotation definitions — coordinates relative to the card's top-left
	// The card is placed at (marginLeft, marginTop) in the output SVG
	annotations := []annotation{
		// Top: Thumbnail
		{Text: "Thumbnail (click to view)",
			LabelX: ml, LabelY: mt - 28,
			LineX1: ml + 120, LineY1: mt - 6,
			LineX2: ml + 120, LineY2: mt + 80},
		// Right side: Edit
		{Text: "Edit \u270E",
			LabelX: ml + cw + 20, LabelY: mt + 8,
			LineX1: ml + cw + 18, LineY1: mt + 19,
			LineX2: ml + 243, LineY2: mt + 21},
		// Right side: Select toggle
		{Text: "Select toggle \u2611/\u2610",
			LabelX: ml + cw + 20, LabelY: mt + 38,
			LineX1: ml + cw + 18, LineY1: mt + 49,
			LineX2: ml + 281, LineY2: mt + 21},
		// Right side: View
		{Text: "View \U0001F441",
			LabelX: ml + cw + 20, LabelY: mt + 68,
			LineX1: ml + cw + 18, LineY1: mt + 79,
			LineX2: ml + 243, LineY2: mt + 55},
		// Right side: Download
		{Text: "Download \u2B07",
			LabelX: ml + cw + 20, LabelY: mt + 90,
			LineX1: ml + cw + 18, LineY1: mt + 101,
			LineX2: ml + 281, LineY2: mt + 55},
		// Right side: Tags
		{Text: "Tags",
			LabelX: ml + cw + 20, LabelY: mt + 110,
			LineX1: ml + cw + 18, LineY1: mt + 121,
			LineX2: ml + 248, LineY2: mt + 100},
		// Bottom: Name (click to edit)
		{Text: "Name (click to edit)",
			LabelX: ml, LabelY: mt + ch + 10,
			LineX1: ml + 40, LineY1: mt + ch + 8,
			LineX2: ml + 130, LineY2: mt + ch - 14},
		// Bottom: Type badge
		{Text: "Type badge",
			LabelX: ml + 240, LabelY: mt + ch + 10,
			LineX1: ml + 275, LineY1: mt + ch + 8,
			LineX2: ml + 275, LineY2: mt + ch - 14},
	}

	// Build output SVG
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`, totalW, totalH, totalW, totalH)
	b.WriteString("\n")

	// White background
	fmt.Fprintf(&b, `  <rect width="%d" height="%d" fill="white"/>`, totalW, totalH)
	b.WriteString("\n")

	// Embed the captured card SVG at offset
	cardSVG := string(data)
	// Replace the outer <svg> tag with a nested one at position
	cardSVG = replaceSVGRoot(cardSVG, marginLeft, marginTop, cardW, cardH)
	b.WriteString(cardSVG)
	b.WriteString("\n")

	// Draw leader lines first (behind labels)
	for _, a := range annotations {
		fmt.Fprintf(&b, `  <line x1="%.0f" y1="%.0f" x2="%.0f" y2="%.0f" stroke="#f5c518" stroke-width="1" stroke-dasharray="4,3"/>`,
			a.LineX1, a.LineY1, a.LineX2, a.LineY2)
		b.WriteString("\n")
	}

	// Draw label boxes
	const (
		padX     = 6
		padY     = 3
		fontSize = 11
	)
	for _, a := range annotations {
		textW := float64(len(a.Text)) * 6.2 // approximate character width
		rectW := textW + 2*padX
		rectH := float64(fontSize) + 2*padY

		// Background rect
		fmt.Fprintf(&b, `  <rect x="%.0f" y="%.0f" width="%.0f" height="%.0f" rx="3" fill="#fffde7" stroke="#f5c518" stroke-width="1"/>`,
			a.LabelX, a.LabelY, rectW, rectH)
		b.WriteString("\n")

		// Text
		textX := a.LabelX + padX
		textY := a.LabelY + padY + fontSize - 1
		fmt.Fprintf(&b, `  <text x="%.0f" y="%.0f" font-family="system-ui, sans-serif" font-size="%d" font-weight="600" fill="#333">%s</text>`,
			textX, textY, fontSize, xmlEscape(a.Text))
		b.WriteString("\n")
	}

	b.WriteString("</svg>\n")

	if err := os.WriteFile(*output, []byte(b.String()), 0644); err != nil {
		log.Fatalf("write %s: %v", *output, err)
	}
	fmt.Printf("Annotated card written to %s\n", *output)
}

// parseSize extracts a numeric pixel value from an SVG size attribute like "320" or "320px".
func parseSize(s string) int {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	var v int
	fmt.Sscanf(s, "%d", &v)
	return v
}

// replaceSVGRoot transforms the root <svg ...> into a nested <svg x="..." y="..." ...>
// It also strips any <?xml ...?> or <!DOCTYPE ...> declarations that would be invalid inside a nested SVG.
func replaceSVGRoot(svg string, x, y, w, h int) string {
	// Strip XML declaration
	if i := strings.Index(svg, "<?xml"); i >= 0 {
		if j := strings.Index(svg[i:], "?>"); j >= 0 {
			svg = strings.TrimSpace(svg[:i] + svg[i+j+2:])
		}
	}
	// Strip DOCTYPE
	if i := strings.Index(svg, "<!DOCTYPE"); i >= 0 {
		if j := strings.Index(svg[i:], ">"); j >= 0 {
			svg = strings.TrimSpace(svg[:i] + svg[i+j+1:])
		}
	}

	// Find the opening <svg tag
	start := strings.Index(svg, "<svg")
	if start < 0 {
		return svg
	}
	end := strings.Index(svg[start:], ">")
	if end < 0 {
		return svg
	}
	end += start

	// Build a new opening tag with position
	newOpen := fmt.Sprintf(`<svg x="%d" y="%d" width="%d" height="%d"`, x, y, w, h)

	// Preserve viewBox and all xmlns attributes from the original tag
	oldTag := svg[start : end+1]
	for _, attr := range []string{"viewBox", "xmlns", "xmlns:xlink"} {
		needle := attr + `="`
		if i := strings.Index(oldTag, needle); i >= 0 {
			if j := strings.Index(oldTag[i+len(needle):], `"`); j >= 0 {
				newOpen += " " + oldTag[i:i+len(needle)+j+1]
			}
		}
	}

	newOpen += ">"
	return svg[:start] + newOpen + svg[end+1:]
}

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}
