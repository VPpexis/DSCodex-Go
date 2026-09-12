package codexconfig

import (
	"errors"
	"os"
	"regexp"
	"strings"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

const bridgeEnvKey = "CODEX_CLI_PATH"

var mcpEnvTablePattern = regexp.MustCompile(`^\s*\[mcp_servers\.[^\]]+\.env\]\s*$`)

// StripBridgeCliPath removes DSCodex-owned CODEX_CLI_PATH entries from
// [mcp_servers.*.env] tables. The Codex app snapshots CODEX_CLI_PATH into those
// tables while the bridge is active, persisting the hijack past
// `launchctl unsetenv`; only the shim or wrapper values are removed, never a
// user override. Non-MCP tables and user-owned values are preserved.
func StripBridgeCliPath(content string, ownedValues []string) string {
	owned := make(map[string]bool, len(ownedValues))
	for _, value := range ownedValues {
		if value != "" {
			owned[value] = true
		}
	}
	if len(owned) == 0 {
		return content
	}
	lines := strings.Split(normalizeNewlines(content), "\n")
	inMcpEnv := false
	kept := make([]string, 0, len(lines))
	for _, line := range lines {
		if tablePattern.MatchString(line) {
			inMcpEnv = mcpEnvTablePattern.MatchString(line)
		}
		if inMcpEnv && keyOf(line) == bridgeEnvKey && owned[assignedString(line)] {
			continue
		}
		kept = append(kept, line)
	}
	return collapseBlankLines(strings.Join(kept, "\n"))
}

// StripBridgeCliPathFromConfig rewrites config.toml only when a DSCodex-owned
// CODEX_CLI_PATH entry is present. It reports whether the file changed.
func StripBridgeCliPathFromConfig(paths constants.Paths, ownedValues []string) (bool, error) {
	data, err := os.ReadFile(paths.Config)
	switch {
	case err == nil:
	case errors.Is(err, os.ErrNotExist):
		return false, nil
	default:
		return false, err
	}
	original := string(data)
	repaired := StripBridgeCliPath(original, ownedValues)
	if repaired == original {
		return false, nil
	}
	if !strings.HasSuffix(repaired, "\n") {
		repaired += "\n"
	}
	if err := atomicWrite(paths.Config, repaired, 0o600); err != nil {
		return false, err
	}
	return true, nil
}
