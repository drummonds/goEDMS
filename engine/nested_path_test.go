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

func TestComputeNestedPath(t *testing.T) {
	tests := []struct {
		id       int
		fileName string
		root     string
		wantPath string
		wantDir  string
	}{
		{42, "doc.pdf", "/docs", "/docs/L/00/00/42/doc.pdf", "/docs/L/00/00/42"},
		{1234, "invoice.pdf", "/docs", "/docs/L/00/12/34/invoice.pdf", "/docs/L/00/12/34"},
		{99999, "scan.png", "/data/documents", "/data/documents/L/09/99/99/scan.png", "/data/documents/L/09/99/99"},
		{100000, "big.pdf", "/docs", "/docs/K/00/10/00/00/big.pdf", "/docs/K/00/10/00/00"},
		{1234567, "huge.pdf", "/docs", "/docs/K/01/23/45/67/huge.pdf", "/docs/K/01/23/45/67"},
	}

	for _, tt := range tests {
		path, folder := ComputeNestedPath(tt.id, tt.fileName, tt.root)
		if path != tt.wantPath {
			t.Errorf("ComputeNestedPath(%d, %q, %q) path = %q, want %q", tt.id, tt.fileName, tt.root, path, tt.wantPath)
		}
		if folder != tt.wantDir {
			t.Errorf("ComputeNestedPath(%d, %q, %q) folder = %q, want %q", tt.id, tt.fileName, tt.root, folder, tt.wantDir)
		}
	}
}
