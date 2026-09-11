package codexconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/VPpexis/dscodex-go/internal/constants"
	"github.com/VPpexis/dscodex-go/internal/keystore"
)

// Catalog builds the merged model catalog from the Codex model cache and
// persists it. internal/catalog implements it once ported; tests use fakes.
type Catalog interface {
	Build(cache map[string]any) (map[string]any, error)
	Write(catalog map[string]any) error
}

// InstallResult mirrors the upstream install() return value.
type InstallResult struct {
	Catalog     map[string]any
	ConfigPath  string
	CatalogPath string
	RouterToken string
}

// StripManagedConfig removes every marker-managed line: the root block and
// the marker-owned [desktop].enabled-reasoning-efforts entry. The rest of the
// file is preserved, with runs of three or more newlines collapsed to two.
func StripManagedConfig(content string) string {
	lines := strings.Split(normalizeNewlines(content), "\n")
	kept := make([]string, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != constants.ManagedMarker {
			kept = append(kept, lines[index])
			continue
		}
		next := index + 1
		for next < len(lines) {
			key := keyOf(lines[next])
			if !rootKeys[key] && key != desktopKey {
				break
			}
			next++
		}
		index = next - 1
	}
	return collapseBlankLines(strings.Join(kept, "\n"))
}

func collapseBlankLines(content string) string {
	return excessBlankLines.ReplaceAllString(content, "\n\n")
}

func assertNoRootConflict(content string) error {
	lines := strings.Split(normalizeNewlines(content), "\n")
	rootEnd := firstTableIndex(lines)
	for index := 0; index < rootEnd; index++ {
		key := keyOf(lines[index])
		if rootKeys[key] {
			return fmt.Errorf("Refusing to replace user-owned root key: %s", key)
		}
	}
	return nil
}

func injectRoot(content string, options RouterOptions) (string, error) {
	rootLines, err := managedRootLines(options)
	if err != nil {
		return "", err
	}
	lines := strings.Split(content, "\n")
	insertAt := firstTableIndex(lines)
	result := make([]string, 0, len(lines)+len(rootLines))
	result = append(result, lines[:insertAt]...)
	result = append(result, rootLines...)
	result = append(result, lines[insertAt:]...)
	return strings.Join(result, "\n"), nil
}

func injectDesktopReasoning(content string) (string, error) {
	lines := strings.Split(content, "\n")
	desktopStart := -1
	for index, line := range lines {
		if desktopTablePattern.MatchString(line) {
			desktopStart = index
			break
		}
	}
	if desktopStart == -1 {
		suffix := ""
		if !strings.HasSuffix(content, "\n") {
			suffix = "\n"
		}
		return fmt.Sprintf("%s%s\n[desktop]\n%s\n%s = %s\n",
			content, suffix, constants.ManagedMarker, desktopKey, reasoningEfforts), nil
	}
	desktopEnd := len(lines)
	for index := desktopStart + 1; index < len(lines); index++ {
		if tablePattern.MatchString(lines[index]) {
			desktopEnd = index
			break
		}
	}
	for index := desktopStart + 1; index < desktopEnd; index++ {
		if keyOf(lines[index]) != desktopKey {
			continue
		}
		if maxEffortPattern.MatchString(lines[index]) {
			return content, nil
		}
		return "", fmt.Errorf("Existing [desktop].%s does not expose max; update it manually", desktopKey)
	}
	insertAt := desktopEnd
	for insertAt > desktopStart+1 && strings.TrimSpace(lines[insertAt-1]) == "" {
		insertAt--
	}
	result := make([]string, 0, len(lines)+2)
	result = append(result, lines[:insertAt]...)
	result = append(result, constants.ManagedMarker, fmt.Sprintf("%s = %s", desktopKey, reasoningEfforts))
	result = append(result, lines[insertAt:]...)
	return strings.Join(result, "\n"), nil
}

// BuildInstalledConfig returns the user's config with the marker-managed root
// block and desktop reasoning entry installed. User-owned root keys are never
// replaced.
func BuildInstalledConfig(content string, options RouterOptions) (string, error) {
	clean := StripManagedConfig(content)
	if err := assertNoRootConflict(clean); err != nil {
		return "", err
	}
	rooted, err := injectRoot(clean, options)
	if err != nil {
		return "", err
	}
	return injectDesktopReasoning(rooted)
}

// Install writes the managed config, the router token, and the merged catalog.
// It validates the stored key state and refuses to run while an untrusted
// legacy router is alive.
func Install(paths constants.Paths, port int, catalog Catalog) (InstallResult, error) {
	if err := assertNoActiveLegacyRouter(paths); err != nil {
		return InstallResult{}, err
	}
	if err := os.MkdirAll(filepath.Dir(paths.Config), 0o700); err != nil {
		return InstallResult{}, err
	}
	original := ""
	data, err := os.ReadFile(paths.Config)
	switch {
	case err == nil:
		original = string(data)
	case errors.Is(err, os.ErrNotExist):
	default:
		return InstallResult{}, err
	}
	if _, err := keystore.ReadRouterConfig(paths.KeyFile, true); err != nil {
		return InstallResult{}, err
	}
	candidateToken := keystore.ReadRouterToken(paths.KeyFile)
	if candidateToken == "" {
		candidateToken = ReadManagedRouterToken(original)
	}
	if candidateToken == "" {
		token, err := keystore.CreateRouterToken()
		if err != nil {
			return InstallResult{}, err
		}
		candidateToken = token
	}
	options := RouterOptions{Port: port, CatalogPath: paths.Catalog, RouterToken: candidateToken}
	configured, err := BuildInstalledConfig(original, options)
	if err != nil {
		return InstallResult{}, err
	}
	cacheData, err := os.ReadFile(paths.Cache)
	if err != nil {
		return InstallResult{}, err
	}
	cache := map[string]any{}
	if err := json.Unmarshal(cacheData, &cache); err != nil {
		return InstallResult{}, err
	}
	built, err := catalog.Build(cache)
	if err != nil {
		return InstallResult{}, err
	}
	if _, err := os.Stat(paths.Backup); errors.Is(err, os.ErrNotExist) {
		if _, err := os.Stat(paths.Config); err == nil {
			backup, err := os.ReadFile(paths.Config)
			if err != nil {
				return InstallResult{}, err
			}
			if err := os.WriteFile(paths.Backup, backup, 0o600); err != nil {
				return InstallResult{}, err
			}
		}
	}
	routerToken, err := keystore.EnsureRouterToken(paths.KeyFile, candidateToken)
	if err != nil {
		return InstallResult{}, err
	}
	if routerToken != candidateToken {
		options.RouterToken = routerToken
		configured, err = BuildInstalledConfig(original, options)
		if err != nil {
			return InstallResult{}, err
		}
	}
	if err := catalog.Write(built); err != nil {
		return InstallResult{}, err
	}
	if !strings.HasSuffix(configured, "\n") {
		configured += "\n"
	}
	if err := atomicWrite(paths.Config, configured, 0o600); err != nil {
		return InstallResult{}, err
	}
	return InstallResult{
		Catalog:     built,
		ConfigPath:  paths.Config,
		CatalogPath: paths.Catalog,
		RouterToken: routerToken,
	}, nil
}

// Uninstall strips the managed config lines and removes DSCodex-owned files.
func Uninstall(paths constants.Paths) error {
	data, err := os.ReadFile(paths.Config)
	switch {
	case err == nil:
		stripped := StripManagedConfig(string(data))
		if !strings.HasSuffix(stripped, "\n") {
			stripped += "\n"
		}
		if err := atomicWrite(paths.Config, stripped, 0o600); err != nil {
			return err
		}
	case errors.Is(err, os.ErrNotExist):
	default:
		return err
	}
	for _, path := range []string{paths.Catalog, paths.KeyFile, paths.SelectionState, paths.BridgeShim} {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

func atomicWrite(path, content string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	temporary := fmt.Sprintf("%s.dscodex-tmp-%d", path, os.Getpid())
	if err := os.WriteFile(temporary, []byte(content), mode); err != nil {
		return err
	}
	if err := os.Chmod(temporary, mode); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, path); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
