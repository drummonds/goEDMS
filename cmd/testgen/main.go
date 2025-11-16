package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jung-kurt/gofpdf"
)

func main() {
	// Create testdocs directory
	testDir := "testdocs"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		fmt.Printf("Error creating testdocs directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Generating test documents in testdocs/ directory...")

	// Generate test documents
	if err := generateEmptyPDF(testDir); err != nil {
		fmt.Printf("Error generating empty PDF: %v\n", err)
	}

	if err := generateHelloWorldPDF(testDir); err != nil {
		fmt.Printf("Error generating Hello World PDF: %v\n", err)
	}

	if err := generateDiagramPDF(testDir); err != nil {
		fmt.Printf("Error generating diagram PDF: %v\n", err)
	}

	if err := generateLongTextPDF(testDir); err != nil {
		fmt.Printf("Error generating long text PDF: %v\n", err)
	}

	fmt.Println("✓ Test document generation complete!")
	fmt.Println("\nGenerated files:")
	fmt.Println("  testdocs/1-empty.pdf (and .txt)")
	fmt.Println("  testdocs/2-hello.pdf (and .txt)")
	fmt.Println("  testdocs/3-diagram.pdf (and .txt)")
	fmt.Println("  testdocs/4-longtext.pdf (and .txt)")
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
