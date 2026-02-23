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
		{1, "A", ""},
		{7, "A", ""},
		{10, "A", ""},
		{11, "B", "01"},
		{42, "B", "04"},
		{1000, "B", "99"},
		{1001, "C", "01/00"},
		{1234, "C", "01/23"},
		{100000, "C", "99/99"},
		{100001, "D", "01/00/00"},
		{1234567, "D", "12/34/56"},
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
		{1, "1"},
		{42, "42"},
		{1234, "1234"},
		{100000, "100000"},
		{1234567, "1234567"},
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
		{42, ".pdf", "42.orig.pdf"},
		{1234, ".jpg", "1234.orig.jpg"},
		{100000, ".png", "100000.orig.png"},
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
		{"C/01/23/1234.orig.pdf", "C/01/23/1234"},
		{"A/42.orig.jpg", "A/42"},
		{"D/12/34/56/1234567.orig.png", "D/12/34/56/1234567"},
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
		{1, ".pdf", "/docs", "/docs/A/1.orig.pdf", "/docs/A"},
		{7, ".pdf", "/docs", "/docs/A/7.orig.pdf", "/docs/A"},
		{10, ".pdf", "/docs", "/docs/A/10.orig.pdf", "/docs/A"},
		{11, ".pdf", "/docs", "/docs/B/01/11.orig.pdf", "/docs/B/01"},
		{42, ".pdf", "/docs", "/docs/B/04/42.orig.pdf", "/docs/B/04"},
		{1000, ".pdf", "/docs", "/docs/B/99/1000.orig.pdf", "/docs/B/99"},
		{1001, ".pdf", "/docs", "/docs/C/01/00/1001.orig.pdf", "/docs/C/01/00"},
		{1234, ".pdf", "/docs", "/docs/C/01/23/1234.orig.pdf", "/docs/C/01/23"},
		{100000, ".png", "/data/documents", "/data/documents/C/99/99/100000.orig.png", "/data/documents/C/99/99"},
		{100001, ".pdf", "/docs", "/docs/D/01/00/00/100001.orig.pdf", "/docs/D/01/00/00"},
		{1234567, ".pdf", "/docs", "/docs/D/12/34/56/1234567.orig.pdf", "/docs/D/12/34/56"},
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
	docPath := "/docs/C/01/23/1234.orig.pdf"

	if got := getOCRPath(docPath); got != "/docs/C/01/23/1234.ocr.txt" {
		t.Errorf("getOCRPath = %q", got)
	}
	if got := getThumbPath(docPath); got != "/docs/C/01/23/1234.thumb.png" {
		t.Errorf("getThumbPath = %q", got)
	}
	if got := getTagsPath(docPath); got != "/docs/C/01/23/1234.tags.json" {
		t.Errorf("getTagsPath = %q", got)
	}
}
