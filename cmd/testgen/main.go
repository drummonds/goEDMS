package main

import (
	"fmt"
	"os"
	"path/filepath"

	thumbnails "github.com/drummonds/go-thumbnails"
	"github.com/drummonds/godocs/internal/testdocs"
)

func main() {
	testDir := "testdocs"

	fmt.Println("Generating test documents in testdocs/ directory...")
	if err := testdocs.Generate(testDir); err != nil {
		fmt.Printf("Error generating test docs: %v\n", err)
		os.Exit(1)
	}

	// Generate thumbnails for each PDF
	for _, name := range testdocs.Files {
		pdfPath := filepath.Join(testDir, name)
		outputPath := thumbnails.DefaultThumbnailPath(pdfPath, 256)
		if err := thumbnails.GenerateStyledAndSave(pdfPath, outputPath, 256, thumbnails.StyleUniform); err != nil {
			fmt.Printf("Warning: Failed to generate thumbnail for %s: %v\n", name, err)
		}
	}

	fmt.Println("\nTest document generation complete!")
}
