package engine

import (
	"testing"
)

func TestNestedDirParts(t *testing.T) {
	tests := []struct {
		id     int
		letter string
		pairs  string
	}{
		{1, "L", "00/00/01"},
		{42, "L", "00/00/42"},
		{1234, "L", "00/12/34"},
		{99999, "L", "09/99/99"},
		{100000, "K", "00/10/00/00"},
		{1234567, "K", "01/23/45/67"},
		{9999999, "K", "09/99/99/99"},
		{10000000, "J", "00/10/00/00/00"},
		{12345678, "J", "00/12/34/56/78"},
	}

	for _, tt := range tests {
		letter, pairs := nestedDirParts(tt.id)
		if letter != tt.letter || pairs != tt.pairs {
			t.Errorf("nestedDirParts(%d) = (%q, %q), want (%q, %q)", tt.id, letter, pairs, tt.letter, tt.pairs)
		}
	}
}

func TestCanonicalBase(t *testing.T) {
	tests := []struct {
		id   int
		want string
	}{
		{1, "000001"},
		{42, "000042"},
		{1234, "001234"},
		{99999, "099999"},
		{100000, "00100000"},
		{1234567, "01234567"},
		{10000000, "0010000000"},
		{12345678, "0012345678"},
	}
	for _, tt := range tests {
		got := canonicalBase(tt.id)
		if got != tt.want {
			t.Errorf("canonicalBase(%d) = %q, want %q", tt.id, got, tt.want)
		}
	}
}

func TestCanonicalDocName(t *testing.T) {
	tests := []struct {
		id   int
		ext  string
		want string
	}{
		{42, ".pdf", "000042.orig.pdf"},
		{1234, ".jpg", "001234.orig.jpg"},
		{100000, ".png", "00100000.orig.png"},
	}
	for _, tt := range tests {
		got := CanonicalDocName(tt.id, tt.ext)
		if got != tt.want {
			t.Errorf("CanonicalDocName(%d, %q) = %q, want %q", tt.id, tt.ext, got, tt.want)
		}
	}
}

func TestSidecarBasePath(t *testing.T) {
	tests := []struct {
		docPath string
		want    string
	}{
		{"L/00/12/34/001234.orig.pdf", "L/00/12/34/001234"},
		{"L/00/00/42/000042.orig.jpg", "L/00/00/42/000042"},
		{"K/01/23/45/67/01234567.orig.png", "K/01/23/45/67/01234567"},
	}
	for _, tt := range tests {
		got := SidecarBasePath(tt.docPath)
		if got != tt.want {
			t.Errorf("SidecarBasePath(%q) = %q, want %q", tt.docPath, got, tt.want)
		}
	}
}

func TestComputeNestedPath(t *testing.T) {
	tests := []struct {
		id       int
		ext      string
		root     string
		wantPath string
		wantDir  string
	}{
		{42, ".pdf", "/docs", "/docs/L/00/00/42/000042.orig.pdf", "/docs/L/00/00/42"},
		{1234, ".pdf", "/docs", "/docs/L/00/12/34/001234.orig.pdf", "/docs/L/00/12/34"},
		{99999, ".png", "/data/documents", "/data/documents/L/09/99/99/099999.orig.png", "/data/documents/L/09/99/99"},
		{100000, ".pdf", "/docs", "/docs/K/00/10/00/00/00100000.orig.pdf", "/docs/K/00/10/00/00"},
		{1234567, ".pdf", "/docs", "/docs/K/01/23/45/67/01234567.orig.pdf", "/docs/K/01/23/45/67"},
	}

	for _, tt := range tests {
		path, folder := ComputeNestedPath(tt.id, tt.ext, tt.root)
		if path != tt.wantPath {
			t.Errorf("ComputeNestedPath(%d, %q, %q) path = %q, want %q", tt.id, tt.ext, tt.root, path, tt.wantPath)
		}
		if folder != tt.wantDir {
			t.Errorf("ComputeNestedPath(%d, %q, %q) folder = %q, want %q", tt.id, tt.ext, tt.root, folder, tt.wantDir)
		}
	}
}

func TestSidecarPaths(t *testing.T) {
	docPath := "/docs/L/00/12/34/001234.orig.pdf"

	if got := getOCRPath(docPath); got != "/docs/L/00/12/34/001234.ocr.txt" {
		t.Errorf("getOCRPath = %q", got)
	}
	if got := getThumbPath(docPath); got != "/docs/L/00/12/34/001234.thumb.png" {
		t.Errorf("getThumbPath = %q", got)
	}
	if got := getTagsPath(docPath); got != "/docs/L/00/12/34/001234.tags.json" {
		t.Errorf("getTagsPath = %q", got)
	}
}
