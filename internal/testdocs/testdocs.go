package testdocs

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jung-kurt/gofpdf"
)

// Files lists the 6 test PDF basenames (without directory prefix).
var Files = []string{
	"1-empty.pdf",
	"2-hello.pdf",
	"3-diagram.pdf",
	"4-longtext.pdf",
	"5-twopage.pdf",
	"6-fivepage.pdf",
}

// Generate creates all 6 test PDFs and their .txt sidecars in dir.
// It skips generation if 1-empty.pdf already exists.
func Generate(dir string) error {
	if _, err := os.Stat(filepath.Join(dir, "1-empty.pdf")); err == nil {
		return nil // already generated
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("create testdocs dir: %w", err)
	}

	generators := []func(string) error{
		generateEmptyPDF,
		generateHelloWorldPDF,
		generateDiagramPDF,
		generateLongTextPDF,
		generateTwoPagePDF,
		generateFivePagePDF,
	}

	for _, gen := range generators {
		if err := gen(dir); err != nil {
			return err
		}
	}
	return nil
}

func generateEmptyPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	if err := pdf.OutputFileAndClose(filepath.Join(dir, "1-empty.pdf")); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "1-empty.txt"), []byte(""), 0644)
}

func generateHelloWorldPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 24)
	pdf.Cell(0, 10, "Hello World")
	pdf.Ln(12)
	pdf.SetFont("Arial", "", 12)
	pdf.MultiCell(0, 5, "This is a simple test document with basic text content.\n\nIt contains multiple lines and demonstrates basic PDF text rendering.", "", "", false)

	if err := pdf.OutputFileAndClose(filepath.Join(dir, "2-hello.pdf")); err != nil {
		return err
	}

	txtContent := "Hello World\n\nThis is a simple test document with basic text content.\n\nIt contains multiple lines and demonstrates basic PDF text rendering."
	return os.WriteFile(filepath.Join(dir, "2-hello.txt"), []byte(txtContent), 0644)
}

func generateDiagramPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(0, 10, "System Architecture Diagram")
	pdf.Ln(15)

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

	// Description
	pdf.SetXY(100, 40)
	pdf.SetFont("Arial", "", 10)
	pdf.MultiCell(90, 5, "This diagram shows a typical three-tier architecture:\n\n1. Client tier (presentation)\n2. Application tier (logic)\n3. Data tier (storage)\n\nData flows from top to bottom through the system.", "", "", false)

	if err := pdf.OutputFileAndClose(filepath.Join(dir, "3-diagram.pdf")); err != nil {
		return err
	}

	txtContent := `System Architecture Diagram

This diagram shows a typical three-tier architecture:

1. Client tier (presentation) - Client Browser
2. Application tier (logic) - Web Server
3. Data tier (storage) - Database

Data flows from top to bottom through the system.`
	return os.WriteFile(filepath.Join(dir, "3-diagram.txt"), []byte(txtContent), 0644)
}

func generateLongTextPDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()

	pdf.SetFont("Arial", "B", 14)
	pdf.Cell(0, 10, "Go Source Code Sample")
	pdf.Ln(12)

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

	if err := pdf.OutputFileAndClose(filepath.Join(dir, "4-longtext.pdf")); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "4-longtext.txt"), []byte(sourceCode), 0644)
}

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

	if err := pdf.OutputFileAndClose(filepath.Join(dir, "5-twopage.pdf")); err != nil {
		return err
	}

	txtContent := `Two-Page Document - Page 1

This is a test document with two pages. It's designed to test thumbnail generation for multi-page documents.

This is the first page with some introductory content.

Two-Page Document - Page 2

This is the second and final page of this document.

It contains different content to make the pages visually distinct in thumbnails.`
	return os.WriteFile(filepath.Join(dir, "5-twopage.txt"), []byte(txtContent), 0644)
}

func generateFivePagePDF(dir string) error {
	pdf := gofpdf.New("P", "mm", "A4", "")
	txtContent := ""

	for i := 1; i <= 5; i++ {
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		title := fmt.Sprintf("Five-Page Document - Page %d", i)
		pdf.Cell(0, 10, title)
		pdf.Ln(12)
		pdf.SetFont("Arial", "", 12)
		content := fmt.Sprintf("This is page %d of a 5-page document.\n\nThis document tests thumbnail generation with the '+' indicator for documents with more than 4 pages.", i)
		pdf.MultiCell(0, 5, content, "", "", false)

		if i > 1 {
			txtContent += "\n\n"
		}
		txtContent += title + "\n\n" + content
	}

	if err := pdf.OutputFileAndClose(filepath.Join(dir, "6-fivepage.pdf")); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "6-fivepage.txt"), []byte(txtContent), 0644)
}
