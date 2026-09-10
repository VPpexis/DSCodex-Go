package keystore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReadRouterConfigMissingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	for _, strict := range []bool{false, true} {
		config, err := ReadRouterConfig(path, strict)
		if err != nil {
			t.Fatalf("ReadRouterConfig(strict=%v) error = %v", strict, err)
		}
		if len(config) != 0 {
			t.Errorf("ReadRouterConfig(strict=%v) = %v, want empty", strict, config)
		}
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "dscodex", "config.json")
	written := map[string]any{
		fieldKey:         "sk-test-round-trip",
		fieldKeyEncoding: encodingPlain,
		fieldRouterToken: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
	}
	if err := WriteRouterConfig(path, written); err != nil {
		t.Fatalf("WriteRouterConfig() error = %v", err)
	}
	config, err := ReadRouterConfig(path, true)
	if err != nil {
		t.Fatalf("ReadRouterConfig() error = %v", err)
	}
	if len(config) != len(written) {
		t.Fatalf("ReadRouterConfig() = %v, want %v", config, written)
	}
	for key, want := range written {
		if got := config[key]; got != want {
			t.Errorf("config[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestReadRouterConfigEmptyObject(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	config, err := ReadRouterConfig(path, true)
	if err != nil {
		t.Fatalf("ReadRouterConfig() error = %v", err)
	}
	if len(config) != 0 {
		t.Errorf("ReadRouterConfig() = %v, want empty", config)
	}
}

func TestWriteEmptyConfigRemovesFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"k":"v"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := WriteRouterConfig(path, map[string]any{}); err != nil {
		t.Fatalf("WriteRouterConfig() error = %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("config file still exists after empty write (stat error = %v)", err)
	}
	if err := WriteRouterConfig(path, map[string]any{}); err != nil {
		t.Errorf("removing a missing config must not fail, got %v", err)
	}
}

func TestWriteNilConfigErrors(t *testing.T) {
	err := WriteRouterConfig(filepath.Join(t.TempDir(), "config.json"), nil)
	if err == nil {
		t.Fatal("WriteRouterConfig(nil) error = nil, want error")
	}
	if want := "DSCodex config must contain a JSON object"; err.Error() != want {
		t.Errorf("WriteRouterConfig(nil) error = %q, want %q", err.Error(), want)
	}
}

func TestReadRouterConfigCorruptDoesNotLeakContent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	fragment := "sk-fake-fragment"
	if err := os.WriteFile(path, []byte(`{"deepseek_api_key": "`+fragment), 0o600); err != nil {
		t.Fatal(err)
	}

	config, err := ReadRouterConfig(path, false)
	if err != nil {
		t.Fatalf("ReadRouterConfig(non-strict) error = %v", err)
	}
	if len(config) != 0 {
		t.Errorf("ReadRouterConfig(non-strict) = %v, want empty", config)
	}

	_, err = ReadRouterConfig(path, true)
	if err == nil {
		t.Fatal("ReadRouterConfig(strict) error = nil, want error")
	}
	if !strings.Contains(err.Error(), "Could not read or parse DSCodex config at") {
		t.Errorf("strict error = %q, want the upstream message", err.Error())
	}
	if strings.Contains(err.Error(), fragment) {
		t.Errorf("strict error leaked file content: %v", err)
	}
}

func TestReadRouterConfigNonObject(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"array", "[1,2,3]"},
		{"string", `"value"`},
		{"number", "42"},
		{"null", "null"},
		{"boolean", "true"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "config.json")
			if err := os.WriteFile(path, []byte(test.content), 0o600); err != nil {
				t.Fatal(err)
			}
			config, err := ReadRouterConfig(path, false)
			if err != nil {
				t.Fatalf("ReadRouterConfig(non-strict) error = %v", err)
			}
			if len(config) != 0 {
				t.Errorf("ReadRouterConfig(non-strict) = %v, want empty", config)
			}
			_, err = ReadRouterConfig(path, true)
			if err == nil {
				t.Fatal("ReadRouterConfig(strict) error = nil, want error")
			}
			if want := "DSCodex config must contain a JSON object"; err.Error() != want {
				t.Errorf("strict error = %q, want %q", err.Error(), want)
			}
		})
	}
}

func TestWriteCreatesParentDirectories(t *testing.T) {
	path := filepath.Join(t.TempDir(), "a", "b", "config.json")
	if err := WriteRouterConfig(path, map[string]any{"k": "v"}); err != nil {
		t.Fatalf("WriteRouterConfig() error = %v", err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("config file not written: %v", err)
	}
}

func TestWriteReplacesExistingContent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := WriteRouterConfig(path, map[string]any{"k": "v1"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteRouterConfig(path, map[string]any{"k": "v2"}); err != nil {
		t.Fatal(err)
	}
	config, err := ReadRouterConfig(path, true)
	if err != nil {
		t.Fatal(err)
	}
	if got := config["k"]; got != "v2" {
		t.Errorf("config[k] = %v, want v2", got)
	}
	temporaries, err := filepath.Glob(filepath.Join(dir, "config.json.dscodex-tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(temporaries) != 0 {
		t.Errorf("temporary files left behind: %v", temporaries)
	}
}

func TestFileModes(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX file modes are not enforced on Windows")
	}
	stateDir := filepath.Join(t.TempDir(), "dscodex")
	path := filepath.Join(stateDir, "config.json")
	if err := WriteRouterConfig(path, map[string]any{"k": "v"}); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("config file mode = %o, want 600", perm)
	}
	dirInfo, err := os.Stat(stateDir)
	if err != nil {
		t.Fatal(err)
	}
	if perm := dirInfo.Mode().Perm(); perm != 0o700 {
		t.Errorf("state dir mode = %o, want 700", perm)
	}
}
