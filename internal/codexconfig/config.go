// Package codexconfig manages the DSCodex-owned lines in Codex's
// ~/.codex/config.toml.
//
// It is the Go port of upstream src/config.mjs. Edits are line-oriented: the
// marker-managed root block (openai_base_url, model_catalog_json) and the
// marker-owned [desktop].enabled-reasoning-efforts entry are the only lines
// ever added, rewritten, or removed. The rest of the file is preserved apart
// from CRLF normalization and upstream's blank-line collapsing.
package codexconfig

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

// RouterOptions describes the marker-managed router binding.
type RouterOptions struct {
	Port        int
	CatalogPath string
	RouterToken string
}

const (
	desktopKey       = "enabled-reasoning-efforts"
	reasoningEfforts = `["low", "medium", "high", "xhigh", "max", "ultra"]`
)

var (
	rootKeys            = map[string]bool{"openai_base_url": true, "model_catalog_json": true}
	keyPattern          = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+)\s*=`)
	tablePattern        = regexp.MustCompile(`^\s*\[`)
	desktopTablePattern = regexp.MustCompile(`^\s*\[desktop\]\s*$`)
	routerTokenPattern  = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)
	routerPathPattern   = regexp.MustCompile(`^/([A-Za-z0-9_-]{43})/v1/?$`)
	maxEffortPattern    = regexp.MustCompile(`\bmax\b`)
	excessBlankLines    = regexp.MustCompile(`\n{3,}`)
)

type rootBlock struct {
	lines  []string
	start  int
	end    int
	values map[string]string
}

func normalizeNewlines(content string) string {
	return strings.ReplaceAll(content, "\r\n", "\n")
}

func keyOf(line string) string {
	match := keyPattern.FindStringSubmatch(line)
	if match == nil {
		return ""
	}
	return match[1]
}

func firstTableIndex(lines []string) int {
	for index, line := range lines {
		if tablePattern.MatchString(line) {
			return index
		}
	}
	return len(lines)
}

func quoteToml(value string) string {
	var buffer bytes.Buffer
	encoder := json.NewEncoder(&buffer)
	encoder.SetEscapeHTML(false)
	_ = encoder.Encode(value)
	return strings.TrimSuffix(buffer.String(), "\n")
}

func validRouterToken(value string) bool {
	return routerTokenPattern.MatchString(value)
}

func assignedString(line string) string {
	equals := strings.Index(line, "=")
	if equals == -1 {
		return ""
	}
	var value any
	if err := json.Unmarshal([]byte(strings.TrimSpace(line[equals+1:])), &value); err != nil {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func managedRootBlock(content string) *rootBlock {
	lines := strings.Split(normalizeNewlines(content), "\n")
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != constants.ManagedMarker {
			continue
		}
		next := index + 1
		if next >= len(lines) || !rootKeys[keyOf(lines[next])] {
			continue
		}
		values := map[string]string{}
		for next < len(lines) && rootKeys[keyOf(lines[next])] {
			key := keyOf(lines[next])
			values[key] = assignedString(lines[next])
			next++
		}
		return &rootBlock{lines: lines, start: index, end: next, values: values}
	}
	return nil
}

func routerBaseURL(options RouterOptions) (string, error) {
	if options.Port < 1 || options.Port > 65535 {
		return "", fmt.Errorf("Invalid port: %d", options.Port)
	}
	if !validRouterToken(options.RouterToken) {
		return "", errors.New("DSCodex install requires a router token")
	}
	return fmt.Sprintf("http://%s:%d/%s/v1", constants.Host, options.Port, options.RouterToken), nil
}

func managedRootLines(options RouterOptions) ([]string, error) {
	baseURL, err := routerBaseURL(options)
	if err != nil {
		return nil, err
	}
	return []string{
		constants.ManagedMarker,
		fmt.Sprintf("openai_base_url = %s", quoteToml(baseURL)),
		fmt.Sprintf("model_catalog_json = %s", quoteToml(options.CatalogPath)),
	}, nil
}

// ReadManagedRouterToken returns the router token embedded in the managed
// openai_base_url value, or "" when the block is missing or the URL does not
// match the loopback router shape.
func ReadManagedRouterToken(content string) string {
	block := managedRootBlock(content)
	if block == nil {
		return ""
	}
	baseURL := block.values["openai_base_url"]
	if baseURL == "" {
		return ""
	}
	parsed, err := url.Parse(baseURL)
	if err != nil {
		return ""
	}
	if parsed.Scheme != "http" || !strings.EqualFold(parsed.Hostname(), constants.Host) {
		return ""
	}
	if parsed.User != nil {
		if parsed.User.Username() != "" {
			return ""
		}
		if _, hasPassword := parsed.User.Password(); hasPassword {
			return ""
		}
	}
	if parsed.RawQuery != "" || parsed.Fragment != "" {
		return ""
	}
	match := routerPathPattern.FindStringSubmatch(parsed.EscapedPath())
	if match == nil {
		return ""
	}
	return match[1]
}

// ManagedRouterConfigMatches reports whether the managed block already points
// at the given port, catalog, and router token.
func ManagedRouterConfigMatches(content string, options RouterOptions) (bool, error) {
	block := managedRootBlock(content)
	if block == nil {
		return false, nil
	}
	baseURL, err := routerBaseURL(options)
	if err != nil {
		return false, err
	}
	return block.values["openai_base_url"] == baseURL &&
		block.values["model_catalog_json"] == options.CatalogPath, nil
}

// RewriteManagedRouterConfig replaces the managed root block with the values
// in options, preserving every other line of the file.
func RewriteManagedRouterConfig(content string, options RouterOptions) (string, error) {
	block := managedRootBlock(content)
	if block == nil {
		return "", errors.New("DSCodex managed router config is missing; run `dscodex install`")
	}
	rootLines, err := managedRootLines(options)
	if err != nil {
		return "", err
	}
	lines := make([]string, 0, len(block.lines)-(block.end-block.start)+len(rootLines))
	lines = append(lines, block.lines[:block.start]...)
	lines = append(lines, rootLines...)
	lines = append(lines, block.lines[block.end:]...)
	return strings.Join(lines, "\n"), nil
}
