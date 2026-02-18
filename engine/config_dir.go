package engine

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"

	"github.com/drummonds/godocs/database"
)

// tagAliasesFile is the on-disk JSON structure for tag_aliases.json
type tagAliasesFile struct {
	TagAliases []database.TagAliasEntry `json:"tag_aliases"`
}

// EnsureConfigDir creates the config directory if it doesn't exist.
func EnsureConfigDir(configPath string) error {
	return os.MkdirAll(configPath, 0755)
}

// WriteTagAliases exports all tag aliases from the database to config/tag_aliases.json.
func WriteTagAliases(configPath string, db database.Repository) error {
	aliases, err := db.GetAllTagAliases()
	if err != nil {
		return err
	}
	if aliases == nil {
		aliases = []database.TagAliasEntry{}
	}

	if err := EnsureConfigDir(configPath); err != nil {
		return err
	}

	data, err := json.MarshalIndent(tagAliasesFile{TagAliases: aliases}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(configPath, "tag_aliases.json"), data, 0644)
}

// ReadTagAliases reads tag aliases from config/tag_aliases.json.
// Returns an empty slice if the file doesn't exist.
func ReadTagAliases(configPath string) ([]database.TagAliasEntry, error) {
	data, err := os.ReadFile(filepath.Join(configPath, "tag_aliases.json"))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	var f tagAliasesFile
	if err := json.Unmarshal(data, &f); err != nil {
		return nil, err
	}
	return f.TagAliases, nil
}

// ApplyTagAliasesFromConfig reads tag_aliases.json and inserts aliases into the database.
// For each entry it looks up the tag by name and inserts the alias.
// Entries whose tag_name doesn't exist in the DB are silently skipped.
func ApplyTagAliasesFromConfig(configPath string, db database.Repository) error {
	entries, err := ReadTagAliases(configPath)
	if err != nil {
		return err
	}

	for _, e := range entries {
		tag, err := db.GetTagByName(e.TagName)
		if err != nil {
			return err
		}
		if tag == nil {
			continue // tag doesn't exist yet
		}
		if err := db.InsertTagAlias(tag.ID, e.AliasName); err != nil {
			return err
		}
	}
	return nil
}
