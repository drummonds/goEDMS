package engine

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"codeberg.org/hum3/godocs/config"
	"codeberg.org/hum3/godocs/database"
)

func newTestRepo(t *testing.T) database.Repository {
	t.Helper()
	if database.Logger == nil {
		database.Logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	}
	db := database.NewRepository(config.ServerConfig{DatabaseType: "pglike", DatabaseDbname: ":memory:"})
	t.Cleanup(func() { db.Close() })
	return db
}

func TestWriteAndReadTagAliases(t *testing.T) {
	repo := newTestRepo(t)

	// Create a tag and rename it to generate an alias
	tag := &database.Tag{Name: "Receipt", Color: "#ff0000"}
	if err := repo.CreateTag(tag); err != nil {
		t.Fatal("Failed to create tag:", err)
	}
	tag.Name = "Receipts"
	if err := repo.UpdateTag(tag); err != nil {
		t.Fatal("Failed to update tag:", err)
	}

	// Write aliases to temp config dir
	configPath := filepath.Join(t.TempDir(), "config")

	if err := WriteTagAliases(configPath, repo); err != nil {
		t.Fatal("WriteTagAliases failed:", err)
	}

	// Verify the file exists
	data, err := os.ReadFile(filepath.Join(configPath, "tag_aliases.json"))
	if err != nil {
		t.Fatal("tag_aliases.json not found:", err)
	}
	t.Log("Written JSON:", string(data))

	// Read back
	entries, err := ReadTagAliases(configPath)
	if err != nil {
		t.Fatal("ReadTagAliases failed:", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Expected 1 alias, got %d", len(entries))
	}
	if entries[0].AliasName != "Receipt" || entries[0].TagName != "Receipts" {
		t.Fatalf("Unexpected alias: %+v", entries[0])
	}
}

func TestReadTagAliasesMissingFile(t *testing.T) {
	entries, err := ReadTagAliases(t.TempDir())
	if err != nil {
		t.Fatal("Expected no error for missing file, got:", err)
	}
	if entries != nil {
		t.Fatalf("Expected nil entries, got %v", entries)
	}
}

func TestApplyTagAliasesFromConfig(t *testing.T) {
	// DB1: create tag, rename it, write aliases
	repo1 := newTestRepo(t)

	tag := &database.Tag{Name: "Draft", Color: "#0000ff"}
	if err := repo1.CreateTag(tag); err != nil {
		t.Fatal(err)
	}
	tag.Name = "Review"
	if err := repo1.UpdateTag(tag); err != nil {
		t.Fatal(err)
	}

	configPath := filepath.Join(t.TempDir(), "config")
	if err := WriteTagAliases(configPath, repo1); err != nil {
		t.Fatal(err)
	}

	// DB2: fresh database, create the "Review" tag, then apply aliases
	repo2 := newTestRepo(t)

	newTag := &database.Tag{Name: "Review", Color: "#0000ff"}
	if err := repo2.CreateTag(newTag); err != nil {
		t.Fatal(err)
	}

	if err := ApplyTagAliasesFromConfig(configPath, repo2); err != nil {
		t.Fatal("ApplyTagAliasesFromConfig failed:", err)
	}

	// Verify alias resolves
	found, err := repo2.GetTagByName("Draft")
	if err != nil {
		t.Fatal(err)
	}
	if found == nil {
		t.Fatal("Expected alias 'Draft' to resolve to 'Review'")
	}
	if found.Name != "Review" {
		t.Fatalf("Expected tag name 'Review', got '%s'", found.Name)
	}
}
