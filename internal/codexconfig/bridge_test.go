package codexconfig

import (
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestStripBridgeCliPathRemovesOwnedValuesFromMCPEnvTables(t *testing.T) {
	shim := "/Users/x/.codex/dscodex/codex-cli-bridge.sh"
	wrapper := "/repo/src/codex-wrapper.mjs"
	content := strings.Join([]string{
		`model = "gpt-5.6-sol"`,
		"",
		"[mcp_servers.node_repl]",
		`command = "/Applications/ChatGPT.app/Contents/Resources/cua_node/bin/node_repl"`,
		"",
		"[mcp_servers.node_repl.env]",
		`NODE_REPL_TRUSTED_CODE_PATHS = "/Users/x/.codex"`,
		`CODEX_CLI_PATH = "` + shim + `"`,
		`CODEX_HOME = "/Users/x/.codex"`,
		"",
		"[mcp_servers.other.env]",
		`CODEX_CLI_PATH = "` + wrapper + `"`,
		"",
		"[desktop]",
		`theme = "light"`,
		"",
	}, "\n")

	stripped := StripBridgeCliPath(content, []string{shim, wrapper})
	if strings.Contains(stripped, "CODEX_CLI_PATH") {
		t.Errorf("StripBridgeCliPath() kept CODEX_CLI_PATH:\n%s", stripped)
	}
	for _, want := range []string{
		"NODE_REPL_TRUSTED_CODE_PATHS",
		"CODEX_HOME",
		"[mcp_servers.node_repl]",
		`theme = "light"`,
	} {
		if !strings.Contains(stripped, want) {
			t.Errorf("StripBridgeCliPath() dropped %q:\n%s", want, stripped)
		}
	}
}

func TestStripBridgeCliPathKeepsUserOwnedValuesAndNonMCPTables(t *testing.T) {
	shim := "/Users/x/.codex/dscodex/codex-cli-bridge.sh"
	content := strings.Join([]string{
		`CODEX_CLI_PATH = "/usr/local/bin/codex"`,
		"",
		"[mcp_servers.node_repl.env]",
		`CODEX_CLI_PATH = "/opt/user-owned/codex"`,
		"",
	}, "\n")

	if got := StripBridgeCliPath(content, []string{shim}); got != content {
		t.Errorf("StripBridgeCliPath() =\n%q\nwant\n%q", got, content)
	}
}

func TestStripBridgeCliPathWithNoOwnedValuesIsNoOp(t *testing.T) {
	content := "[mcp_servers.node_repl.env]\nCODEX_CLI_PATH = \"/x\"\n"
	if got := StripBridgeCliPath(content, nil); got != content {
		t.Errorf("StripBridgeCliPath() = %q, want %q", got, content)
	}
	if got := StripBridgeCliPath(content, []string{""}); got != content {
		t.Errorf("StripBridgeCliPath(empty value) = %q, want %q", got, content)
	}
}

func TestStripBridgeCliPathNormalizesCRLFAndBlankLines(t *testing.T) {
	shim := "/Users/x/.codex/dscodex/codex-cli-bridge.sh"
	content := "[mcp_servers.node_repl.env]\r\nCODEX_CLI_PATH = \"" + shim + "\"\r\n\r\n\r\n[desktop]\r\ntheme = \"light\"\r\n"
	got := StripBridgeCliPath(content, []string{shim})
	want := "[mcp_servers.node_repl.env]\n\n[desktop]\ntheme = \"light\"\n"
	if got != want {
		t.Errorf("StripBridgeCliPath() =\n%q\nwant\n%q", got, want)
	}
}

func TestStripBridgeCliPathFromConfigWritesOnlyWhenChanged(t *testing.T) {
	shim := "/Users/x/.codex/dscodex/codex-cli-bridge.sh"
	paths := newTestPaths(t)

	changed, err := StripBridgeCliPathFromConfig(paths, []string{shim})
	if err != nil || changed {
		t.Fatalf("StripBridgeCliPathFromConfig(missing) = %v, %v; want false, nil", changed, err)
	}

	original := "[mcp_servers.node_repl.env]\nCODEX_CLI_PATH = \"/opt/user-owned/codex\"\n"
	writeFile(t, paths.Config, original)
	changed, err = StripBridgeCliPathFromConfig(paths, []string{shim})
	if err != nil || changed {
		t.Fatalf("StripBridgeCliPathFromConfig(user-owned) = %v, %v; want false, nil", changed, err)
	}
	if got := readFile(t, paths.Config); got != original {
		t.Errorf("config changed without an owned value: %q", got)
	}

	owned := "[mcp_servers.node_repl.env]\nCODEX_CLI_PATH = \"" + shim + "\"\nCODEX_HOME = \"/Users/x/.codex\"\n"
	writeFile(t, paths.Config, owned)
	changed, err = StripBridgeCliPathFromConfig(paths, []string{shim})
	if err != nil || !changed {
		t.Fatalf("StripBridgeCliPathFromConfig(owned) = %v, %v; want true, nil", changed, err)
	}
	repaired := readFile(t, paths.Config)
	if strings.Contains(repaired, shim) {
		t.Errorf("owned CODEX_CLI_PATH still present: %q", repaired)
	}
	if !strings.HasSuffix(repaired, "\n") {
		t.Errorf("repaired config does not end with a newline: %q", repaired)
	}

	changed, err = StripBridgeCliPathFromConfig(paths, []string{shim})
	if err != nil || changed {
		t.Fatalf("StripBridgeCliPathFromConfig(second call) = %v, %v; want false, nil", changed, err)
	}
	if got := readFile(t, paths.Config); got != repaired {
		t.Errorf("second call rewrote the config: %q", got)
	}

	if _, err := os.Stat(paths.Config + ".dscodex-tmp-" + strconv.Itoa(os.Getpid())); !os.IsNotExist(err) {
		t.Errorf("temporary file left behind (stat error = %v)", err)
	}
}
