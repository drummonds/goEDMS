package engine

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ComputeNestedPath returns the nested directory path for a document based on its DB ID.
// The scheme uses letter prefixes (L/K/J) and 2-digit pair directory levels:
//
//	L: IDs 1–99,999       → L/XX/YY/ZZ/NNNNNN.orig.ext
//	K: IDs 100,000–9,999,999 → K/XX/YY/ZZ/WW/NNNNNNNN.orig.ext
//	J: IDs 10,000,000+    → J/XX/YY/ZZ/WW/VV/NNNNNNNNNN.orig.ext
//
// ext should include the leading dot (e.g. ".pdf").
func ComputeNestedPath(id int, ext string, documentRoot string) (path string, folder string) {
	letter, pairs := nestedDirParts(id)
	dir := filepath.Join(documentRoot, letter, filepath.FromSlash(pairs))
	name := CanonicalDocName(id, ext)
	return filepath.ToSlash(filepath.Join(dir, name)), filepath.ToSlash(dir)
}

func nestedDirParts(id int) (letter string, pairs string) {
	switch {
	case id < 100000:
		s := fmt.Sprintf("%06d", id)
		return "L", fmt.Sprintf("%s/%s/%s", s[0:2], s[2:4], s[4:6])
	case id < 10000000:
		s := fmt.Sprintf("%08d", id)
		return "K", fmt.Sprintf("%s/%s/%s/%s", s[0:2], s[2:4], s[4:6], s[6:8])
	default:
		s := fmt.Sprintf("%010d", id)
		return "J", fmt.Sprintf("%s/%s/%s/%s/%s", s[0:2], s[2:4], s[4:6], s[6:8], s[8:10])
	}
}

// canonicalBase returns the zero-padded ID string matching the tier width.
//
//	L tier (< 100k):  6 digits  → "001234"
//	K tier (< 10M):   8 digits  → "00123456"
//	J tier (≥ 10M):  10 digits  → "0012345678"
func canonicalBase(id int) string {
	switch {
	case id < 100000:
		return fmt.Sprintf("%06d", id)
	case id < 10000000:
		return fmt.Sprintf("%08d", id)
	default:
		return fmt.Sprintf("%010d", id)
	}
}

// CanonicalDocName returns the canonical filename for a document: "001234.orig.pdf".
// ext should include the leading dot.
func CanonicalDocName(id int, ext string) string {
	return canonicalBase(id) + ".orig" + ext
}

// SidecarBasePath strips the ".orig.{ext}" suffix from a canonical document path,
// returning the base used for all sidecar files.
// e.g. "L/00/12/34/001234.orig.pdf" → "L/00/12/34/001234"
func SidecarBasePath(docPath string) string {
	ext := filepath.Ext(docPath) // ".pdf"
	withoutExt := docPath[:len(docPath)-len(ext)]
	return strings.TrimSuffix(withoutExt, ".orig")
}

// getOCRPath returns the OCR/extracted text sidecar path for a document.
func getOCRPath(docPath string) string {
	return SidecarBasePath(docPath) + ".ocr.txt"
}

// getThumbPath returns the thumbnail sidecar path for a document.
func getThumbPath(docPath string) string {
	return SidecarBasePath(docPath) + ".thumb.png"
}

// getTagsPath returns the tags metadata sidecar path for a document.
func getTagsPath(docPath string) string {
	return SidecarBasePath(docPath) + ".tags.json"
}
