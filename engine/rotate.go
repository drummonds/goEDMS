package engine

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"codeberg.org/hum3/godocs/database"
	pdfcpuapi "github.com/pdfcpu/pdfcpu/pkg/api"
)

// RotateDocumentFile rotates a document file in place.
// degrees must be 90, 180, or 270 (clockwise).
func RotateDocumentFile(docPath string, degrees int) error {
	if degrees != 90 && degrees != 180 && degrees != 270 {
		return fmt.Errorf("invalid rotation: %d (must be 90, 180, or 270)", degrees)
	}

	ext := strings.ToLower(filepath.Ext(docPath))
	switch ext {
	case ".pdf":
		return rotatePDF(docPath, degrees)
	case ".jpg", ".jpeg", ".png", ".tiff":
		return rotateImage(docPath, degrees)
	default:
		return fmt.Errorf("rotation not supported for %s files", ext)
	}
}

// rotatePDF rotates all pages of a PDF file using pdfcpu.
func rotatePDF(docPath string, degrees int) error {
	tmpFile := docPath + ".tmp"
	if err := pdfcpuapi.RotateFile(docPath, tmpFile, degrees, nil, nil); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("PDF rotation failed: %w", err)
	}
	if err := os.Rename(tmpFile, docPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to replace original PDF: %w", err)
	}
	return nil
}

// rotateImage rotates an image file in place.
// imaging uses counter-clockwise, so we invert: CW 90 = CCW 270, CW 270 = CCW 90.
func rotateImage(docPath string, degrees int) error {
	src, err := imaging.Open(docPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}

	var rotated *image.NRGBA
	switch degrees {
	case 90:
		rotated = imaging.Rotate270(src) // CW 90 = CCW 270
	case 180:
		rotated = imaging.Rotate180(src)
	case 270:
		rotated = imaging.Rotate90(src) // CW 270 = CCW 90
	}

	ext := strings.ToLower(filepath.Ext(docPath))
	tmpFile := docPath + ".tmp"
	out, err := os.Create(tmpFile)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}

	switch ext {
	case ".jpg", ".jpeg":
		err = jpeg.Encode(out, rotated, &jpeg.Options{Quality: 95})
	case ".png":
		err = png.Encode(out, rotated)
	case ".tiff":
		// imaging doesn't have a TIFF encoder; save as PNG instead won't work.
		// Use imaging.Save which handles format detection.
		out.Close()
		os.Remove(tmpFile)
		return imaging.Save(rotated, docPath)
	}

	out.Close()
	if err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to encode rotated image: %w", err)
	}

	if err := os.Rename(tmpFile, docPath); err != nil {
		os.Remove(tmpFile)
		return fmt.Errorf("failed to replace original image: %w", err)
	}
	return nil
}

// ensureOCRNeededTag returns the ID of the "OCR Needed" system tag, creating it if necessary.
func ensureOCRNeededTag(db database.Repository) (int, error) {
	tag, err := db.GetTagByName("OCR Needed")
	if err != nil {
		return 0, fmt.Errorf("failed to look up OCR Needed tag: %w", err)
	}
	if tag != nil {
		return tag.ID, nil
	}

	// Create it
	group := "System"
	newTag := &database.Tag{
		Name:     "OCR Needed",
		Color:    "#e67e22",
		TagGroup: &group,
	}
	if err := db.CreateTag(newTag); err != nil {
		return 0, fmt.Errorf("failed to create OCR Needed tag: %w", err)
	}

	// Re-fetch to get the ID
	tag, err = db.GetTagByName("OCR Needed")
	if err != nil || tag == nil {
		return 0, fmt.Errorf("failed to retrieve newly created OCR Needed tag")
	}
	return tag.ID, nil
}
