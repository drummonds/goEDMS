package engine

import (
	"fmt"
	"path/filepath"
)

// ComputeNestedPath returns the nested directory path for a document based on its DB ID.
// The scheme uses letter prefixes (L/K/J) and 2-digit pair directory levels:
//
//	L: IDs 1–99,999       → L/XX/YY/ZZ/filename
//	K: IDs 100,000–9,999,999 → K/XX/YY/ZZ/WW/filename
//	J: IDs 10,000,000+    → J/XX/YY/ZZ/WW/VV/filename
func ComputeNestedPath(id int, fileName string, documentRoot string) (path string, folder string) {
	letter, pairs := nestedDirParts(id)
	dir := filepath.Join(documentRoot, letter, filepath.FromSlash(pairs))
	return filepath.ToSlash(filepath.Join(dir, fileName)), filepath.ToSlash(dir)
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
