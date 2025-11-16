package engine

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"path/filepath"

	"github.com/drummonds/godocs/engine/pdfrenderer"
	"github.com/nfnt/resize"
)

// getThumbnailPath returns the path to the thumbnail file for a document
func getThumbnailPath(docPath string) string {
	ext := filepath.Ext(docPath)
	return docPath[:len(docPath)-len(ext)] + ".tn_64.png"
}

// generateThumbnail creates a 64-pixel high thumbnail showing up to 4 pages in a row
// If the document has more than 4 pages, shows first 4 pages plus a "+" indicator
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
	if err := os.MkdirAll(filepath.Dir(thumbnailPath), 0755); err != nil {
		return fmt.Errorf("failed to create thumbnail directory: %w", err)
	}

	outFile, err := os.Create(thumbnailPath)
	if err != nil {
		return fmt.Errorf("failed to create thumbnail file: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, composite); err != nil {
		return fmt.Errorf("failed to encode thumbnail: %w", err)
	}

	if Logger != nil {
		Logger.Info("Generated thumbnail", "pdf", pdfPath, "thumbnail", thumbnailPath, "pages", len(pages))
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

	for y := size/4; y < 3*size/4; y++ {
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

// saveThumbnailFile generates and saves a thumbnail for a PDF document
func saveThumbnailFile(pdfPath string) error {
	return generateThumbnail(pdfPath)
}
