// Package constants holds the values shared across DSCodex-Go.
//
// It is the Go port of upstream src/constants.mjs. Model slugs, display
// names, and file paths are behavioral contracts: the Codex catalog and the
// router's model routing depend on them matching the upstream values exactly.
package constants

import (
	"os"
	"path/filepath"
	"strings"
)

// Version is the upstream-parity version string.
const Version = "1.1.0"

// DefaultPort is the loopback router port.
const DefaultPort = 10110

// Host is the loopback bind address.
const Host = "127.0.0.1"

// Upstream API bases.
const (
	DeepSeekBaseURL     = "https://api.deepseek.com"
	ChatGPTCodexBaseURL = "https://chatgpt.com/backend-api/codex"
)

// ManagedMarker marks the DSCodex-owned block in config.toml.
const ManagedMarker = "# DSCodex managed; remove with `dscodex uninstall`"

// Model describes a DeepSeek model exposed in the Codex picker.
type Model struct {
	PickerSlug  string
	WireModel   string
	DisplayName string
	ProductName string
}

// DeepSeekModels lists the models added to the Codex picker, in order.
var DeepSeekModels = []Model{
	{
		PickerSlug:  "deepseek/deepseek-v4-flash",
		WireModel:   "deepseek-v4-flash",
		DisplayName: "DeepSeek V4 Flash",
		ProductName: "DeepSeek V4 Flash",
	},
	{
		PickerSlug:  "deepseek/deepseek-v4-pro",
		WireModel:   "deepseek-v4-pro",
		DisplayName: "DeepSeek V4 Pro",
		ProductName: "DeepSeek V4 Pro",
	},
}

// DeepSeekModelFor returns the model matching a picker slug or wire model,
// or nil when the model is not a DeepSeek model.
func DeepSeekModelFor(model string) *Model {
	for i := range DeepSeekModels {
		if model == DeepSeekModels[i].PickerSlug || model == DeepSeekModels[i].WireModel {
			return &DeepSeekModels[i]
		}
	}
	return nil
}

// ResolveCodexHome returns the Codex home directory: CODEX_HOME when set,
// otherwise ~/.codex. Pass nil for env to read the process environment.
func ResolveCodexHome(env func(string) string) string {
	if env == nil {
		env = os.Getenv
	}
	if value := strings.TrimSpace(env("CODEX_HOME")); value != "" {
		return value
	}
	return filepath.Join(homeDir(), ".codex")
}

func homeDir() string {
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		return home
	}
	if home := os.Getenv("USERPROFILE"); home != "" {
		return home
	}
	return ""
}

// NeedsShellSpawn reports whether an executable must run through the command
// interpreter. Windows cannot spawn .cmd/.bat files directly (CreateProcess
// requires an .exe), while POSIX scripts need no shell.
func NeedsShellSpawn(executablePath string, goos string) bool {
	if goos != "windows" {
		return false
	}
	lower := strings.ToLower(executablePath)
	return strings.HasSuffix(lower, ".cmd") || strings.HasSuffix(lower, ".bat")
}

// Paths is the DSCodex file layout under a Codex home directory.
type Paths struct {
	Config         string
	Cache          string
	Catalog        string
	Backup         string
	StateDir       string
	KeyFile        string
	SelectionState string
	BridgeShim     string
	PID            string
	Log            string
}

// PathsFor returns the file layout for a Codex home directory.
func PathsFor(codexHome string) Paths {
	stateDir := filepath.Join(codexHome, "dscodex")
	return Paths{
		Config:         filepath.Join(codexHome, "config.toml"),
		Cache:          filepath.Join(codexHome, "models_cache.json"),
		Catalog:        filepath.Join(codexHome, "dscodex-models.json"),
		Backup:         filepath.Join(codexHome, "config.toml.pre-dscodex.bak"),
		StateDir:       stateDir,
		KeyFile:        filepath.Join(stateDir, "config.json"),
		SelectionState: filepath.Join(stateDir, "model-selections.json"),
		BridgeShim:     filepath.Join(stateDir, "codex-cli-bridge.sh"),
		PID:            filepath.Join(stateDir, "server.pid"),
		Log:            filepath.Join(stateDir, "server.log"),
	}
}
