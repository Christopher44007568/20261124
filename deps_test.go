package main_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestImportsAreStandardLibraryOnly(t *testing.T) {
	root := "."
	var goFiles []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") {
			goFiles = append(goFiles, path)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk go files: %v", err)
	}

	for _, file := range goFiles {
		data, err := os.ReadFile(file)
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		text := string(data)
		blockStart := strings.Index(text, "import (")
		if blockStart < 0 {
			continue
		}
		blockEnd := strings.Index(text[blockStart:], ")")
		if blockEnd < 0 {
			t.Fatalf("%s has unterminated import block", file)
		}
		block := text[blockStart : blockStart+blockEnd]
		for _, line := range strings.Split(block, "\n") {
			line = strings.TrimSpace(line)
			if !strings.HasPrefix(line, "\"") {
				continue
			}
			importPath := strings.Trim(line, "\"")
			if !isStandardLibrary(importPath) {
				t.Fatalf("%s imports external package %q", file, importPath)
			}
		}
	}
}

func isStandardLibrary(importPath string) bool {
	if importPath == "" || strings.ContainsRune(importPath, ' ') {
		return false
	}
	first := strings.Split(importPath, "/")[0]
	return !strings.Contains(first, ".")
}

func TestGoModHasNoRequires(t *testing.T) {
	data, err := os.ReadFile("go.mod")
	if err != nil {
		t.Fatalf("read go.mod: %v", err)
	}
	if strings.Contains(string(data), "require") {
		t.Fatal("go.mod must not require any external module")
	}
}
