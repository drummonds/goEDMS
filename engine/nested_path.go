package engine

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
)

// ComputeNestedPath returns the nested directory path for a document based on its DB ID.
// The scheme uses letter prefixes (A–Z) with base-100 pair directory levels.
// leaf = (id-1)/10 determines the tier: A (leaf 0), B (1–99), C (100–9999), etc.
// ext should include the leading dot (e.g. ".pdf").
func ComputeNestedPath(id int, ext string, documentRoot string) (path string, folder string) {
	letter, pairs := nestedDirParts(id)
	dir := filepath.Join(documentRoot, letter, filepath.FromSlash(pairs))
	name := CanonicalDocName(id, ext)
	return filepath.ToSlash(filepath.Join(dir, name)), filepath.ToSlash(dir)
}

func nestedDirParts(id int) (letter string, pairs string) {
	leaf := (id - 1) / 10
	if leaf == 0 {
		return "A", ""
	}
	// Decompose leaf into base-100 pairs (right-to-left)
	var parts []string
	for n := leaf; n > 0; n /= 100 {
		parts = append(parts, fmt.Sprintf("%02d", n%100))
	}
	// Reverse to get most-significant first
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	letter = string(rune('A' + len(parts)))
	return letter, strings.Join(parts, "/")
}

// canonicalBase returns the plain decimal ID string (no zero-padding).
func canonicalBase(id int) string {
	return strconv.Itoa(id)
}

// CanonicalDocName returns the canonical filename for a document: "1234.orig.pdf".
// ext should include the leading dot.
func CanonicalDocName(id int, ext string) string {
	return canonicalBase(id) + ".orig" + ext
}

// SidecarBasePath strips the ".orig.{ext}" suffix from a canonical document path,
// returning the base used for all sidecar files.
// e.g. "C/01/23/1234.orig.pdf" → "C/01/23/1234"
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
