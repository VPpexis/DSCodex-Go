package codexconfig

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"

	"github.com/VPpexis/dscodex-go/internal/constants"
	"github.com/VPpexis/dscodex-go/internal/keystore"
)

type fakeCatalog struct {
	path      string
	builds    int
	writes    int
	lastCache map[string]any
	lastBuilt map[string]any
}

func (f *fakeCatalog) Build(cache map[string]any) (map[string]any, error) {
	f.builds++
	f.lastCache = cache
	return map[string]any{"models": []any{
		map[string]any{"slug": "gpt-5.6-sol"},
		map[string]any{"slug": "deepseek/deepseek-flash"},
	}}, nil
}

func (f *fakeCatalog) Write(catalog map[string]any) error {
	f.writes++
	f.lastBuilt = catalog
	data, err := json.Marshal(catalog)
	if err != nil {
		return err
	}
	return os.WriteFile(f.path, data, 0o600)
}

func newTestPaths(t *testing.T) constants.Paths {
	t.Helper()
	return constants.PathsFor(t.TempDir())
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestBuildInstalledConfigIsRootCorrectReversibleAndPreservesUserConfig(t *testing.T) {
	original := "personality = \"pragmatic\"\n\n[features]\nmulti_agent = true\n\n[desktop]\ntheme = \"light\"\n"
	token := routerToken()
	options := RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json", RouterToken: token}

	installed, err := BuildInstalledConfig(original, options)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Index(installed, "openai_base_url") > strings.Index(installed, "[features]") {
		t.Error("managed root block was not injected before the first table")
	}
	if !strings.Contains(installed, `model_catalog_json = "/tmp/models.json"`) {
		t.Errorf("installed config missing catalog path:\n%s", installed)
	}
	if !strings.Contains(installed, "enabled-reasoning-efforts = [") || !strings.Contains(installed, `"max"`) {
		t.Errorf("installed config missing the desktop reasoning list:\n%s", installed)
	}
	if got := ReadManagedRouterToken(installed); got != token {
		t.Errorf("ReadManagedRouterToken(installed) = %q, want %q", got, token)
	}
	matched, err := ManagedRouterConfigMatches(installed, options)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("ManagedRouterConfigMatches(installed) = false, want true")
	}
	if got := StripManagedConfig(installed); got != original {
		t.Errorf("StripManagedConfig(installed) =\n%q\nwant\n%q", got, original)
	}
}

func TestBuildInstalledConfigRefusesUserOwnedRootKeys(t *testing.T) {
	options := RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json", RouterToken: routerToken()}
	contents := []string{
		"openai_base_url = \"https://example.test/v1\"\n",
		"model_catalog_json = \"/tmp/user.json\"\n",
		"personality = \"pragmatic\"\nopenai_base_url = \"https://example.test/v1\"\n[features]\n",
	}
	for _, content := range contents {
		if _, err := BuildInstalledConfig(content, options); err == nil || !strings.Contains(err.Error(), "user-owned root key") {
			t.Errorf("BuildInstalledConfig(%q) error = %v, want the user-owned root key error", content, err)
		}
	}
}

func TestBuildInstalledConfigDoesNotInstallUnauthenticatedURL(t *testing.T) {
	_, err := BuildInstalledConfig("", RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json"})
	if err == nil || !strings.Contains(err.Error(), "requires a router token") {
		t.Errorf("BuildInstalledConfig(no token) error = %v, want the router token error", err)
	}
}

func TestInjectDesktopReasoning(t *testing.T) {
	block := constants.ManagedMarker + "\n" + desktopKey + " = " + reasoningEfforts + "\n"
	tests := []struct {
		name    string
		content string
		want    string
		wantErr bool
	}{
		{
			name:    "appends a desktop table when missing",
			content: "model = \"gpt-5.6-sol\"\n",
			want:    "model = \"gpt-5.6-sol\"\n\n[desktop]\n" + block,
		},
		{
			name:    "inserts into an existing desktop table",
			content: "[desktop]\ntheme = \"light\"\n",
			want:    "[desktop]\ntheme = \"light\"\n" + block,
		},
		{
			name:    "inserts before trailing blank lines",
			content: "[desktop]\ntheme = \"light\"\n\n[features]\nmulti_agent = true\n",
			want:    "[desktop]\ntheme = \"light\"\n" + block + "\n[features]\nmulti_agent = true\n",
		},
		{
			name:    "keeps an existing list that exposes max",
			content: "[desktop]\nenabled-reasoning-efforts = [\"low\", \"max\"]\n",
			want:    "[desktop]\nenabled-reasoning-efforts = [\"low\", \"max\"]\n",
		},
		{
			name:    "errors when an existing list lacks max",
			content: "[desktop]\nenabled-reasoning-efforts = [\"low\", \"high\"]\n",
			wantErr: true,
		},
		{
			name:    "adds a separating newline when the file has none",
			content: "model = \"gpt-5.6-sol\"",
			want:    "model = \"gpt-5.6-sol\"\n\n[desktop]\n" + block,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := injectDesktopReasoning(test.content)
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "does not expose max") {
					t.Fatalf("injectDesktopReasoning() error = %v, want the max error", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Errorf("injectDesktopReasoning() =\n%q\nwant\n%q", got, test.want)
			}
		})
	}
}

func TestStripManagedConfig(t *testing.T) {
	marker := constants.ManagedMarker
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "removes root block and desktop entry",
			content: "personality = \"pragmatic\"\n" + marker + "\nopenai_base_url = \"http://127.0.0.1:10110/" + routerToken() + "/v1\"\nmodel_catalog_json = \"/tmp/models.json\"\n\n" + marker + "\n" + desktopKey + " = " + reasoningEfforts + "\n\n[features]\nmulti_agent = true\n",
			want:    "personality = \"pragmatic\"\n\n[features]\nmulti_agent = true\n",
		},
		{
			name:    "keeps an unmanaged desktop entry",
			content: "[desktop]\n" + desktopKey + " = [\"low\"]\n",
			want:    "[desktop]\n" + desktopKey + " = [\"low\"]\n",
		},
		{
			name:    "normalizes CRLF",
			content: marker + "\r\nopenai_base_url = \"http://127.0.0.1:10110/" + routerToken() + "/v1\"\r\nmodel_catalog_json = \"/tmp/models.json\"\r\n",
			want:    "",
		},
		{
			name:    "collapses excessive blank lines",
			content: "a = 1\n\n\n\nb = 2\n",
			want:    "a = 1\n\nb = 2\n",
		},
		{
			name:    "content without a marker is unchanged",
			content: "model = \"gpt-5.6-sol\"\n",
			want:    "model = \"gpt-5.6-sol\"\n",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := StripManagedConfig(test.content); got != test.want {
				t.Errorf("StripManagedConfig() =\n%q\nwant\n%q", got, test.want)
			}
		})
	}
}

func TestInstallAndUninstallTouchOnlyDSCodexOwnedFilesAndLines(t *testing.T) {
	paths := newTestPaths(t)
	original := "[features]\nmulti_agent = true\n\n[desktop]\ntheme = \"light\"\n"
	writeFile(t, paths.Config, original)
	writeFile(t, paths.Cache, `{"models":[{"slug":"gpt-5.6-sol"}]}`)

	catalog := &fakeCatalog{path: paths.Catalog}
	result, err := Install(paths, 10110, catalog)
	if err != nil {
		t.Fatal(err)
	}
	if catalog.builds != 1 || catalog.writes != 1 {
		t.Errorf("catalog calls = %d builds / %d writes, want 1/1", catalog.builds, catalog.writes)
	}
	models, _ := result.Catalog["models"].([]any)
	if len(models) != 2 {
		t.Errorf("installed catalog has %d models, want 2", len(models))
	}
	if !routerTokenPattern.MatchString(result.RouterToken) {
		t.Errorf("result.RouterToken = %q, want a 43-character base64url token", result.RouterToken)
	}
	if stored := keystore.ReadRouterToken(paths.KeyFile); stored != result.RouterToken {
		t.Errorf("stored router token = %q, want %q", stored, result.RouterToken)
	}
	config := readFile(t, paths.Config)
	if !strings.Contains(config, "127.0.0.1:10110/"+result.RouterToken+"/v1") {
		t.Errorf("config does not point at the authenticated router:\n%s", config)
	}
	if !strings.Contains(config, "DSCodex managed") {
		t.Errorf("config is missing the managed marker:\n%s", config)
	}
	if got := readFile(t, paths.Backup); got != original {
		t.Errorf("backup = %q, want the original config", got)
	}
	if _, err := os.Stat(paths.Catalog); err != nil {
		t.Errorf("catalog file missing after install: %v", err)
	}

	writeFile(t, paths.SelectionState, "{}\n")
	writeFile(t, paths.BridgeShim, "#!/bin/sh\n")

	if err := Uninstall(paths); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{paths.Catalog, paths.KeyFile, paths.SelectionState, paths.BridgeShim} {
		if _, err := os.Stat(path); !os.IsNotExist(err) {
			t.Errorf("%s still exists after uninstall (stat error = %v)", path, err)
		}
	}
	if got := readFile(t, paths.Config); got != original {
		t.Errorf("config after uninstall = %q, want %q", got, original)
	}
	if got := readFile(t, paths.Backup); got != original {
		t.Errorf("backup after uninstall = %q, want %q", got, original)
	}
}

func TestInstallWritesBackupOnce(t *testing.T) {
	paths := newTestPaths(t)
	first := "[features]\nmulti_agent = true\n"
	writeFile(t, paths.Config, first)
	writeFile(t, paths.Cache, `{"models":[]}`)

	if _, err := Install(paths, 10110, &fakeCatalog{path: paths.Catalog}); err != nil {
		t.Fatal(err)
	}
	second := "[features]\nmulti_agent = false\n"
	writeFile(t, paths.Config, second)
	if _, err := Install(paths, 10110, &fakeCatalog{path: paths.Catalog}); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, paths.Backup); got != first {
		t.Errorf("backup = %q, want the first install's config %q", got, first)
	}
}

func TestInstallValidatesRouterStateBeforeChangingConfigOrCatalog(t *testing.T) {
	paths := newTestPaths(t)
	original := "[features]\nmulti_agent = true\n"
	secret := "sk-fake-review-secret"
	writeFile(t, paths.Config, original)
	writeFile(t, paths.Cache, `{"models":[{"slug":"gpt-5.6-sol"}]}`)
	if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.KeyFile, secret)

	catalog := &fakeCatalog{path: paths.Catalog}
	_, err := Install(paths, 10110, catalog)
	if err == nil {
		t.Fatal("Install() = nil error, want the corrupt config error")
	}
	if !strings.Contains(err.Error(), "Could not read or parse DSCodex config at") {
		t.Errorf("Install() error = %v, want the upstream message", err)
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("Install() error leaked the stored secret: %v", err)
	}
	if got := readFile(t, paths.Config); got != original {
		t.Errorf("config changed after a failed install: %q", got)
	}
	if catalog.builds != 0 || catalog.writes != 0 {
		t.Errorf("catalog calls = %d builds / %d writes, want 0/0", catalog.builds, catalog.writes)
	}
	if _, err := os.Stat(paths.Catalog); !os.IsNotExist(err) {
		t.Errorf("catalog file created after a failed install (stat error = %v)", err)
	}
	if got := readFile(t, paths.KeyFile); got != secret {
		t.Errorf("stored key changed after a failed install: %q", got)
	}
}

func TestInstallRefusesRunningLegacyRouterBeforePublishingAuthenticatedState(t *testing.T) {
	paths := newTestPaths(t)
	original := constants.ManagedMarker + "\n" +
		"openai_base_url = \"http://127.0.0.1:10110/v1\"\n" +
		"model_catalog_json = " + quoteToml(paths.Catalog) + "\n"
	writeFile(t, paths.Config, original)
	writeFile(t, paths.Cache, `{"models":[{"slug":"gpt-5.6-sol"}]}`)
	if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.PID, fmt.Sprintf("{\"pid\":%d,\"port\":10110}\n", os.Getpid()))

	catalog := &fakeCatalog{path: paths.Catalog}
	_, err := Install(paths, 10110, catalog)
	if err == nil || !strings.Contains(err.Error(), "older or untrusted DSCodex state is still running") {
		t.Fatalf("Install() error = %v, want the legacy router error", err)
	}
	if got := readFile(t, paths.Config); got != original {
		t.Errorf("config changed after a refused install: %q", got)
	}
	if catalog.builds != 0 || catalog.writes != 0 {
		t.Errorf("catalog calls = %d builds / %d writes, want 0/0", catalog.builds, catalog.writes)
	}
	if _, err := os.Stat(paths.Catalog); !os.IsNotExist(err) {
		t.Errorf("catalog file created after a refused install (stat error = %v)", err)
	}
	if _, err := os.Stat(paths.KeyFile); !os.IsNotExist(err) {
		t.Errorf("key file created after a refused install (stat error = %v)", err)
	}
	if _, err := os.Stat(paths.Backup); !os.IsNotExist(err) {
		t.Errorf("backup created after a refused install (stat error = %v)", err)
	}
}

func TestInstallAllowsAuthenticatedPidState(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.Cache, `{"models":[]}`)
	if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	pid := os.Getpid()
	state := fmt.Sprintf(
		`{"pid":%d,"port":10110,"routerToken":%q,"shutdownToken":%q,"instanceId":%q}`,
		pid, routerToken(), strings.Repeat("B", 43), fmt.Sprintf("%d-1-0123456789abcdef", pid),
	)
	writeFile(t, paths.PID, state)

	if _, err := Install(paths, 10110, &fakeCatalog{path: paths.Catalog}); err != nil {
		t.Fatalf("Install() error = %v, want success over an authenticated pid state", err)
	}
}

func TestInstallAllowsUntrustedPidStateForDeadProcess(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.Cache, `{"models":[]}`)
	if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.PID, fmt.Sprintf("{\"pid\":%d,\"port\":10110}\n", deadProcessPID(t)))

	if _, err := Install(paths, 10110, &fakeCatalog{path: paths.Catalog}); err != nil {
		t.Fatalf("Install() error = %v, want stale pid state to be tolerated", err)
	}
}

func TestAssertNoActiveLegacyRouterRejectsMalformedState(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"invalid JSON", "not json", "Invalid DSCodex pid state at"},
		{"non-integer pid", `{"pid":"x","port":10110}`, "Untrusted DSCodex pid state at"},
		{"missing pid", `{"port":10110}`, "Untrusted DSCodex pid state at"},
		{"non-positive pid", `{"pid":0,"port":10110}`, "Untrusted DSCodex pid state at"},
		{"invalid port", `{"pid":1,"port":70000}`, "Untrusted DSCodex pid state at"},
		{"null state", `null`, "Untrusted DSCodex pid state at"},
		{"array state", `[]`, "Untrusted DSCodex pid state at"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := newTestPaths(t)
			if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
				t.Fatal(err)
			}
			writeFile(t, paths.PID, test.content)
			err := assertNoActiveLegacyRouter(paths)
			if err == nil || !strings.Contains(err.Error(), test.want) {
				t.Errorf("assertNoActiveLegacyRouter() error = %v, want %q", err, test.want)
			}
		})
	}
}

func TestInstallMigratesLegacyWindowsPlaintextKeyBeforeReturning(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("legacy plaintext migration is Windows-only")
	}
	paths := newTestPaths(t)
	legacyKey := "sk-fake-legacy-install-key"
	if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.KeyFile, fmt.Sprintf("{\"deepseek_api_key\":%q}\n", legacyKey))
	writeFile(t, paths.Cache, `{"models":[]}`)

	if _, err := Install(paths, 10110, &fakeCatalog{path: paths.Catalog}); err != nil {
		t.Fatal(err)
	}
	if got := keystore.ReadStoredKey(paths.KeyFile); got != legacyKey {
		t.Errorf("ReadStoredKey() = %q, want %q", got, legacyKey)
	}
	if strings.Contains(readFile(t, paths.KeyFile), legacyKey) {
		t.Error("legacy plaintext key still present after install")
	}
}

func TestUninstallWithoutConfig(t *testing.T) {
	paths := newTestPaths(t)
	if err := Uninstall(paths); err != nil {
		t.Fatalf("Uninstall() error = %v, want nil", err)
	}
}

func deadProcessPID(t *testing.T) int {
	t.Helper()
	cmd := exec.Command(os.Args[0], "-test.run=^TestCodexconfigHelperProcess$")
	cmd.Env = append(os.Environ(), "DSCODEX_CODECONFIG_HELPER=1")
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	if err := cmd.Wait(); err != nil {
		t.Fatal(err)
	}
	return cmd.Process.Pid
}

func TestCodexconfigHelperProcess(t *testing.T) {
	if os.Getenv("DSCODEX_CODECONFIG_HELPER") != "1" {
		t.Skip("helper process")
	}
	os.Exit(0)
}
