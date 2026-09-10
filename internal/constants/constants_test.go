package constants

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDeepSeekModelsMatchUpstream(t *testing.T) {
	want := []Model{
		{
			PickerSlug:  "deepseek/deepseek-v4-flash",
			WireModel:   "deepseek-v4-flash",
			DisplayName: "🐳 V4 Flash",
			ProductName: "DeepSeek V4 Flash",
		},
		{
			PickerSlug:  "deepseek/deepseek-v4-pro",
			WireModel:   "deepseek-v4-pro",
			DisplayName: "🐳 V4 Pro",
			ProductName: "DeepSeek V4 Pro",
		},
	}
	if len(DeepSeekModels) != len(want) {
		t.Fatalf("DeepSeekModels length = %d, want %d", len(DeepSeekModels), len(want))
	}
	for i := range want {
		if DeepSeekModels[i] != want[i] {
			t.Errorf("DeepSeekModels[%d] = %+v, want %+v", i, DeepSeekModels[i], want[i])
		}
	}
}

func TestDeepSeekModelFor(t *testing.T) {
	tests := []struct {
		name  string
		model string
		want  string
	}{
		{"picker slug flash", "deepseek/deepseek-v4-flash", "deepseek-v4-flash"},
		{"picker slug pro", "deepseek/deepseek-v4-pro", "deepseek-v4-pro"},
		{"wire model flash", "deepseek-v4-flash", "deepseek-v4-flash"},
		{"wire model pro", "deepseek-v4-pro", "deepseek-v4-pro"},
		{"unknown model", "gpt-5.6-sol", ""},
		{"empty model", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := DeepSeekModelFor(test.model)
			if test.want == "" {
				if got != nil {
					t.Fatalf("DeepSeekModelFor(%q) = %+v, want nil", test.model, got)
				}
				return
			}
			if got == nil {
				t.Fatalf("DeepSeekModelFor(%q) = nil, want wire model %q", test.model, test.want)
			}
			if got.WireModel != test.want {
				t.Errorf("DeepSeekModelFor(%q).WireModel = %q, want %q", test.model, got.WireModel, test.want)
			}
		})
	}
}

func TestResolveCodexHome(t *testing.T) {
	t.Run("CODEX_HOME override is trimmed", func(t *testing.T) {
		env := func(key string) string {
			if key == "CODEX_HOME" {
				return "  /custom/codex  "
			}
			return ""
		}
		if got := ResolveCodexHome(env); got != "/custom/codex" {
			t.Errorf("ResolveCodexHome() = %q, want %q", got, "/custom/codex")
		}
	})

	t.Run("empty CODEX_HOME falls back to home/.codex", func(t *testing.T) {
		home, err := os.UserHomeDir()
		if err != nil || home == "" {
			t.Skip("home directory unavailable")
		}
		env := func(string) string { return "   " }
		want := filepath.Join(home, ".codex")
		if got := ResolveCodexHome(env); got != want {
			t.Errorf("ResolveCodexHome() = %q, want %q", got, want)
		}
	})
}

func TestNeedsShellSpawn(t *testing.T) {
	tests := []struct {
		name string
		path string
		goos string
		want bool
	}{
		{"windows cmd", `C:\tools\codex.cmd`, "windows", true},
		{"windows bat uppercase", `C:\tools\CODEX.BAT`, "windows", true},
		{"windows exe", `C:\tools\codex.exe`, "windows", false},
		{"windows no extension", `C:\tools\codex`, "windows", false},
		{"darwin cmd", "/usr/local/bin/codex.cmd", "darwin", false},
		{"linux bat", "/usr/local/bin/codex.bat", "linux", false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := NeedsShellSpawn(test.path, test.goos); got != test.want {
				t.Errorf("NeedsShellSpawn(%q, %q) = %v, want %v", test.path, test.goos, got, test.want)
			}
		})
	}
}

func TestPathsFor(t *testing.T) {
	home := filepath.Join("root", "codex-home")
	stateDir := filepath.Join(home, "dscodex")
	want := Paths{
		Config:         filepath.Join(home, "config.toml"),
		Cache:          filepath.Join(home, "models_cache.json"),
		Catalog:        filepath.Join(home, "dscodex-models.json"),
		Backup:         filepath.Join(home, "config.toml.pre-dscodex.bak"),
		StateDir:       stateDir,
		KeyFile:        filepath.Join(stateDir, "config.json"),
		SelectionState: filepath.Join(stateDir, "model-selections.json"),
		BridgeShim:     filepath.Join(stateDir, "codex-cli-bridge.sh"),
		PID:            filepath.Join(stateDir, "server.pid"),
		Log:            filepath.Join(stateDir, "server.log"),
	}
	if got := PathsFor(home); got != want {
		t.Errorf("PathsFor(%q) = %+v, want %+v", home, got, want)
	}
}
