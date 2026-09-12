package catalog

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

// Write persists the merged catalog as 2-space-indented JSON with a trailing
// newline. The write is atomic (temp file + rename) and mode 0600.
func Write(catalogPath string, catalog map[string]any) error {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(catalog); err != nil {
		return err
	}
	temporary := fmt.Sprintf("%s.dscodex-tmp-%d", catalogPath, os.Getpid())
	if err := os.WriteFile(temporary, buffer.Bytes(), 0o600); err != nil {
		return err
	}
	if err := os.Rename(temporary, catalogPath); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}

// Store adapts the package functions to the codexconfig.Catalog interface.
type Store struct{}

// Build merges the DeepSeek entries into the Codex model cache.
func (Store) Build(cache map[string]any) (map[string]any, error) {
	return Build(cache)
}

// Write persists the merged catalog atomically.
func (Store) Write(catalogPath string, catalog map[string]any) error {
	return Write(catalogPath, catalog)
}

// Sync rebuilds the catalog from the Codex model cache and writes it.
func Sync(cachePath, catalogPath string) (map[string]any, error) {
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, err
	}
	cache := map[string]any{}
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, err
	}
	built, err := Build(cache)
	if err != nil {
		return nil, err
	}
	if err := Write(catalogPath, built); err != nil {
		return nil, err
	}
	return built, nil
}
