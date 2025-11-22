package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/drummonds/godocs/engine/pdfrenderer"
	"github.com/jung-kurt/gofpdf"
	"github.com/nfnt/resize"
	"image"
	"image/color"
	"image/draw"
	"image/png"
)

func main() {
	// Create testdocs directory
	testDir := "testdocs"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		fmt.Printf("Error creating testdocs directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating test documents in testdocs/ directory...")

	// List of PDFs to generate
	pdfGenerators := []struct {
		name string
		fn   func(string) error
		path string
	}{
		{"empty PDF", generateEmptyPDF, filepath.Join(testDir, "1-empty.pdf")},
		{"Hello World PDF", generateHelloWorldPDF, filepath.Join(testDir, "2-hello.pdf")},
		{"diagram PDF", generateDiagramPDF, filepath.Join(testDir, "3-diagram.pdf")},
		{"long text PDF", generateLongTextPDF, filepath.Join(testDir, "4-longtext.pdf")},
		{"two-page PDF", generateTwoPagePDF, filepath.Join(testDir, "5-twopage.pdf")},
		{"five-page PDF", generateFivePagePDF, filepath.Join(testDir, "6-fivepage.pdf")},
	}

	// Generate PDFs and thumbnails
	for _, gen := range pdfGenerators {
		if err := gen.fn(testDir); err != nil {
			fmt.Printf("Error generating %s: %v\n", gen.name, err)
			continue
		}

		// Generate thumbnail
		if err := generateThumbnail(gen.path); err != nil {
			fmt.Printf("Warning: Failed to generate thumbnail for %s: %v\n", gen.name, err)
		}
	}

	fmt.Println("\n✓ Test document generation complete!")
	fmt.Println("\nGenerated files:")
	fmt.Println("  testdocs/1-empty.pdf (and .txt, .tn_64.png)")
	fmt.Println("  testdocs/2-hello.pdf (and .txt, .tn_64.png)")
	fmt.Println("  testdocs/3-diagram.pdf (and .txt, .tn_64.png)")
	fmt.Println("  testdocs/4-longtext.pdf (and .txt, .tn_64.png)")
	fmt.Println("  testdocs/5-twopage.pdf (and .txt, .tn_64.png)")
	fmt.Println("  testdocs/6-fivepage.pdf (and .txt, .tn_64.png)")
}

// generateEmptyPDF creates an empty PDF with no content
func generateEmptyPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	// Don't add any content - just an empty page

	pdfPath := filepath.Join(dir, "1-empty.pdf")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}

	// Create empty .txt file
	txtPath := filepath.Join(dir, "1-empty.txt")
	if err := os.WriteFile(txtPath, []byte(""), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Generated 1-empty.pdf")
	return nil
}

// generateHelloWorldPDF creates a simple PDF with "Hello World" text
func generateHelloWorldPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 24)
	pdf.Cell(0, 10, "Hello World")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 5, "This is a simple test document with basic text content.\n\nIt contains multiple lines and demonstrates basic PDF text rendering.", "", "", false)

	pdfPath := filepath.Join(dir, "2-hello.pdf")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}

	// Create .txt file with the same content
	txtContent := "Hello World\n\nThis is a simple test document with basic text content.\n\nIt contains multiple lines and demonstrates basic PDF text rendering."
	txtPath := filepath.Join(dir, "2-hello.txt")
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Generated 2-hello.pdf")
	return nil
}

// generateDiagramPDF creates a PDF with a simple SVG-like diagram
func generateDiagramPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "System Architecture Diagram")
	pdf.Ln(15)

	// Draw a simple flowchart-style diagram
	// Box 1: Client
	pdf.SetFillColor(200, 220, 255)
	pdf.Rect(20, 40, 60, 20, "FD")
	pdf.SetXY(20, 45)
	pdf.SetFont("Arial", "", 12)
	pdf.CellFormat(60, 10, "Client Browser", "", 0, "C", false, 0, "")

	// Arrow down
	pdf.Line(50, 60, 50, 75)
	pdf.Line(50, 75, 45, 70)
	pdf.Line(50, 75, 55, 70)

	// Box 2: Server
	pdf.SetFillColor(255, 220, 200)
	pdf.Rect(20, 75, 60, 20, "FD")
	pdf.SetXY(20, 80)
	pdf.CellFormat(60, 10, "Web Server", "", 0, "C", false, 0, "")

	// Arrow down
	pdf.Line(50, 95, 50, 110)
	pdf.Line(50, 110, 45, 105)
	pdf.Line(50, 110, 55, 105)

	// Box 3: Database
	pdf.SetFillColor(200, 255, 200)
	pdf.Rect(20, 110, 60, 20, "FD")
	pdf.SetXY(20, 115)
	pdf.CellFormat(60, 10, "Database", "", 0, "C", false, 0, "")

	// Add description
	pdf.SetXY(100, 40)
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(90, 5, "This diagram shows a typical three-tier architecture:\n\n1. Client tier (presentation)\n2. Application tier (logic)\n3. Data tier (storage)\n\nData flows from top to bottom through the system.", "", "", false)

	pdfPath := filepath.Join(dir, "3-diagram.pdf")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}

	// Create .txt file with diagram description
	txtContent := `System Architecture Diagram

This diagram shows a typical three-tier architecture:

1. Client tier (presentation) - Client Browser
2. Application tier (logic) - Web Server
3. Data tier (storage) - Database

Data flows from top to bottom through the system.`
	txtPath := filepath.Join(dir, "3-diagram.txt")
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Generated 3-diagram.pdf")
	return nil
}

// generateLongTextPDF creates a PDF with lots of text (using Go source code)
func generateLongTextPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 10, "Go Source Code Sample")
	pdf.Ln(12)

	// Read some actual Go source code from this file
	sourceCode := `package main

import (
	"fmt"
	"os"
	"path/filepath"
	"github.com/jung-kurt/gofpdf"
)

// This is a sample of Go source code that demonstrates
// various language features and syntax patterns.

type Document struct {
	ID       int
	Name     string
	Path     string
	Content  []byte
	Created  time.Time
}

func main() {
	fmt.Println("Hello, World!")

	// Process documents
	docs := []Document{
		{ID: 1, Name: "doc1.pdf", Path: "/docs/1.pdf"},
		{ID: 2, Name: "doc2.pdf", Path: "/docs/2.pdf"},
	}

	for _, doc := range docs {
		if err := processDocument(doc); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
	}
}

func processDocument(doc Document) error {
	// Open the document
	file, err := os.Open(doc.Path)
	if err != nil {
		return fmt.Errorf("failed to open %s: %w", doc.Name, err)
	}
	defer file.Close()

	// Read content
	content, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("failed to read content: %w", err)
	}

	// Process content
	fmt.Printf("Processing document: %s (%d bytes)\n", doc.Name, len(content))

	return nil
}

// Additional helper functions
func validatePath(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func createDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// Constants
const (
	MaxFileSize = 10 * 1024 * 1024 // 10MB
	BufferSize  = 4096
)

// Error types
var (
	ErrInvalidPath   = errors.New("invalid path")
	ErrFileTooLarge  = errors.New("file too large")
	ErrNotFound      = errors.New("not found")
)

This sample code demonstrates:
- Package declarations and imports
- Type definitions (structs)
- Function definitions with error handling
- Loops and conditionals
- Constants and variables
- Error handling patterns
- Comments and documentation
- Go idioms and best practices

The code shows a typical document processing workflow
with proper error handling and resource cleanup using defer.`

	pdf.SetFont("Courier", "", 8)
	pdf.MultiCell(0, 4, sourceCode, "", "", false)

	pdfPath := filepath.Join(dir, "4-longtext.pdf")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}

	// Create .txt file with the same content
	txtPath := filepath.Join(dir, "4-longtext.txt")
	if err := os.WriteFile(txtPath, []byte(sourceCode), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Generated 4-longtext.pdf")
	return nil
}

// generateTwoPagePDF creates a 2-page PDF for testing multi-page thumbnails
func generateTwoPagePDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")

	// Page 1
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Two-Page Document - Page 1")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 5, "This is a test document with two pages. It's designed to test thumbnail generation for multi-page documents.\n\nThis is the first page with some introductory content.", "", "", false)

	// Page 2
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "Two-Page Document - Page 2")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 5, "This is the second and final page of this document.\n\nIt contains different content to make the pages visually distinct in thumbnails.", "", "", false)

	pdfPath := filepath.Join(dir, "5-twopage.pdf")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}

	// Create .txt file with the combined content
	txtContent := `Two-Page Document - Page 1

This is a test document with two pages. It's designed to test thumbnail generation for multi-page documents.

This is the first page with some introductory content.

Two-Page Document - Page 2

This is the second and final page of this document.

It contains different content to make the pages visually distinct in thumbnails.`
	txtPath := filepath.Join(dir, "5-twopage.txt")
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Generated 5-twopage.pdf")
	return nil
}

// generateFivePagePDF creates a 5-page PDF for testing thumbnail "+  indicator
func generateFivePagePDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	txtContent := ""

	// Generate 5 pages
	for i := 1; i <= 5; i++ {
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		title := fmt.Sprintf("Five-Page Document - Page %d", i)
		pdf.Cell(0, 10, title)
		pdf.Ln(12)
		pdf.SetFont("Arial", "", 12)
		content := fmt.Sprintf("This is page %d of a 5-page document.\n\nThis document tests thumbnail generation with the '+' indicator for documents with more than 4 pages.", i)
		pdf.MultiCell(0, 5, content, "", "", false)

		// Add to text content
		if i > 1 {
			txtContent += "\n\n"
		}
		txtContent += title + "\n\n" + content
	}

	pdfPath := filepath.Join(dir, "6-fivepage.pdf")
	if err := pdf.OutputFileAndClose(pdfPath); err != nil {
		return err
	}

	// Create .txt file with the combined content
	txtPath := filepath.Join(dir, "6-fivepage.txt")
	if err := os.WriteFile(txtPath, []byte(txtContent), 0644); err != nil {
		return err
	}

	fmt.Println("✓ Generated 6-fivepage.pdf")
	return nil
}

// getThumbnailPath returns the path to the thumbnail file for a document
func getThumbnailPath(docPath string) string {
	ext := filepath.Ext(docPath)
	return docPath[:len(docPath)-len(ext)] + ".tn_64.png"
}

// generateThumbnail creates a 64-pixel high thumbnail showing up to 4 pages in a row
func generateThumbnail(pdfPath string) error {
	// Create PDF renderer
	renderer, err := pdfrenderer.NewPDFiumRenderer()
	if err != nil {
		return fmt.Errorf("failed to create PDF renderer: %w", err)
	}
	defer renderer.Close()

	// Render all pages
	pages, err := renderer.RenderPDF(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to render PDF pages: %w", err)
	}

	if len(pages) == 0 {
		return fmt.Errorf("PDF has no pages")
	}

	// Determine how many pages to show (max 4)
	numPagesToShow := len(pages)
	showPlusIndicator := false
	if numPagesToShow > 4 {
		numPagesToShow = 4
		showPlusIndicator = true
	}

	// Resize each page to 64 pixels high (maintaining aspect ratio)
	const thumbnailHeight = 64
	resizedPages := make([]image.Image, numPagesToShow)
	totalWidth := 0

	for i := 0; i < numPagesToShow; i++ {
		// Resize to 64 pixels high
		resized := resize.Resize(0, thumbnailHeight, pages[i], resize.Lanczos3)
		resizedPages[i] = resized
		totalWidth += resized.Bounds().Dx()
	}

	// Add width for "+" indicator if needed
	plusWidth := 0
	if showPlusIndicator {
		plusWidth = thumbnailHeight // Square "+" indicator
		totalWidth += plusWidth
	}

	// Create composite image
	composite := image.NewRGBA(image.Rect(0, 0, totalWidth, thumbnailHeight))

	// Fill with white background
	draw.Draw(composite, composite.Bounds(), &image.Uniform{color.White}, image.Point{}, draw.Src)

	// Draw each page thumbnail side by side
	currentX := 0
	for i := 0; i < numPagesToShow; i++ {
		bounds := resizedPages[i].Bounds()
		destRect := image.Rect(currentX, 0, currentX+bounds.Dx(), thumbnailHeight)
		draw.Draw(composite, destRect, resizedPages[i], bounds.Min, draw.Src)
		currentX += bounds.Dx()
	}

	// Draw "+" indicator if needed
	if showPlusIndicator {
		drawPlusIndicator(composite, currentX, thumbnailHeight)
	}

	// Save thumbnail as PNG
	thumbnailPath := getThumbnailPath(pdfPath)
	outFile, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, composite); err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	return nil
}

// drawPlusIndicator draws a simple "+" symbol in a square area
func drawPlusIndicator(img *image.RGBA, startX, size int) {
	// Light gray background for the indicator
	bgColor := color.RGBA{240, 240, 240, 255}
	plusColor := color.RGBA{100, 100, 100, 255}

	// Draw background
	for y := 0; y < size; y++ {
		for x := startX; x < startX+size; x++ {
			img.Set(x, y, bgColor)
		}
	}

	// Draw "+" symbol
	// Vertical line
	centerX := startX + size/2
	lineWidth := size / 8
	if lineWidth < 2 {
		lineWidth = 2
	}

	for y := size / 4; y < 3*size/4; y++ {
		for dx := -lineWidth / 2; dx <= lineWidth/2; dx++ {
			img.Set(centerX+dx, y, plusColor)
		}
	}

	// Horizontal line
	centerY := size / 2
	for x := startX + size/4; x < startX+3*size/4; x++ {
		for dy := -lineWidth / 2; dy <= lineWidth/2; dy++ {
			img.Set(x, centerY+dy, plusColor)
		}
	}
}
