package catalog

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

func normalized(t *testing.T, value any) map[string]any {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	result := map[string]any{}
	if err := json.Unmarshal(data, &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func TestWriteCatalogAtomicFormattedAndPrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dscodex-models.json")
	built, err := Build(map[string]any{"models": []any{template()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(path, built); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(data), "\n") {
		t.Error("catalog does not end with a newline")
	}
	if !strings.Contains(string(data), "\n  \"models\"") {
		t.Errorf("catalog is not 2-space indented:\n%s", data)
	}
	onDisk := map[string]any{}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(onDisk, normalized(t, built)) {
		t.Error("catalog on disk does not match the written value")
	}
	if _, err := os.Stat(path + ".dscodex-tmp-" + strconv.Itoa(os.Getpid())); !os.IsNotExist(err) {
		t.Errorf("temporary file left behind (stat error = %v)", err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("catalog mode = %v, want 0600", info.Mode().Perm())
		}
	}
}

func TestWriteCatalogReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dscodex-models.json")
	if err := os.WriteFile(path, []byte("stale"), 0o600); err != nil {
		t.Fatal(err)
	}
	built, err := Build(map[string]any{"models": []any{template()}})
	if err != nil {
		t.Fatal(err)
	}
	if err := Write(path, built); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "stale") {
		t.Errorf("catalog still contains the previous content:\n%s", data)
	}
}

func TestSyncCatalogReadsCacheAndWritesCatalog(t *testing.T) {
	dir := t.TempDir()
	cachePath := filepath.Join(dir, "models_cache.json")
	catalogPath := filepath.Join(dir, "dscodex-models.json")
	if err := os.WriteFile(cachePath, []byte(`{"models":[{"slug":"gpt-5.6-sol","base_instructions":"You are Codex, based on GPT-5."}]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	built, err := Sync(cachePath, catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if len(built["models"].([]any)) != len(constants.DeepSeekModels)+1 {
		t.Errorf("Sync() catalog has %d models, want %d", len(built["models"].([]any)), len(constants.DeepSeekModels)+1)
	}
	onDisk := map[string]any{}
	data, err := os.ReadFile(catalogPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(onDisk, normalized(t, built)) {
		t.Error("catalog on disk does not match the value returned by Sync")
	}
}

func TestSyncCatalogErrors(t *testing.T) {
	dir := t.TempDir()
	catalogPath := filepath.Join(dir, "dscodex-models.json")

	if _, err := Sync(filepath.Join(dir, "missing.json"), catalogPath); err == nil {
		t.Error("Sync(missing cache) = nil error, want an error")
	}
	empty := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(empty, []byte(`{"models":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Sync(empty, catalogPath)
	if err == nil || !strings.Contains(err.Error(), "Codex models_cache.json has no model templates; open Codex once, then retry") {
		t.Errorf("Sync(empty cache) error = %v, want the upstream message", err)
	}
	if _, err := os.Stat(catalogPath); !os.IsNotExist(err) {
		t.Errorf("catalog written after a failed sync (stat error = %v)", err)
	}
}
