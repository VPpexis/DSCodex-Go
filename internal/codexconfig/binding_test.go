package codexconfig

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/VPpexis/dscodex-go/internal/constants"
	"github.com/VPpexis/dscodex-go/internal/keystore"
)

func TestEnsureManagedRouterBindingUpgradesLegacyURLAndPreservesUserConfig(t *testing.T) {
	paths := newTestPaths(t)
	original := strings.Join([]string{
		"personality = \"pragmatic\"",
		constants.ManagedMarker,
		"openai_base_url = \"http://127.0.0.1:10110/v1\"",
		"model_catalog_json = " + quoteToml(paths.Catalog),
		"",
		"[features]",
		"multi_agent = true",
		"",
	}, "\n")
	writeFile(t, paths.Config, original)

	result, err := EnsureManagedRouterBinding(paths, 10110)
	if err != nil {
		t.Fatalf("EnsureManagedRouterBinding() error = %v", err)
	}
	if !result.Updated {
		t.Error("EnsureManagedRouterBinding() Updated = false, want true")
	}
	if stored := keystore.ReadRouterToken(paths.KeyFile); stored != result.RouterToken {
		t.Errorf("stored router token = %q, want %q", stored, result.RouterToken)
	}
	updated := readFile(t, paths.Config)
	if got := ReadManagedRouterToken(updated); got != result.RouterToken {
		t.Errorf("ReadManagedRouterToken(updated) = %q, want %q", got, result.RouterToken)
	}
	matched, err := ManagedRouterConfigMatches(updated, RouterOptions{
		Port:        10110,
		CatalogPath: paths.Catalog,
		RouterToken: result.RouterToken,
	})
	if err != nil || !matched {
		t.Errorf("ManagedRouterConfigMatches(updated) = %v, %v; want true, nil", matched, err)
	}
	if !strings.Contains(updated, "personality = \"pragmatic\"") {
		t.Errorf("user root keys were not preserved:\n%s", updated)
	}
	if !strings.Contains(updated, "[features]\nmulti_agent = true") {
		t.Errorf("user tables were not preserved:\n%s", updated)
	}
}

func TestEnsureManagedRouterBindingRefusesRunningLegacyRouterBeforeMutation(t *testing.T) {
	paths := newTestPaths(t)
	original := strings.Join([]string{
		constants.ManagedMarker,
		"openai_base_url = \"http://127.0.0.1:10110/v1\"",
		"model_catalog_json = " + quoteToml(paths.Catalog),
		"",
	}, "\n")
	writeFile(t, paths.Config, original)
	if err := os.MkdirAll(paths.StateDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.PID, fmt.Sprintf("{\"pid\":%d,\"port\":10110}\n", os.Getpid()))

	_, err := EnsureManagedRouterBinding(paths, 10110)
	if err == nil || !strings.Contains(err.Error(), "older or untrusted DSCodex state is still running") {
		t.Fatalf("EnsureManagedRouterBinding() error = %v, want the legacy router error", err)
	}
	if got := readFile(t, paths.Config); got != original {
		t.Errorf("config changed after a refused binding: %q", got)
	}
	if _, err := os.Stat(paths.KeyFile); !os.IsNotExist(err) {
		t.Errorf("key file created after a refused binding (stat error = %v)", err)
	}
}

func TestEnsureManagedRouterBindingAdoptsInstalledURLTokenWhenStateMissing(t *testing.T) {
	paths := newTestPaths(t)
	token := routerToken()
	configured, err := BuildInstalledConfig("", RouterOptions{
		Port:        10110,
		CatalogPath: paths.Catalog,
		RouterToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, paths.Config, configured+"\n")

	result, err := EnsureManagedRouterBinding(paths, 10110)
	if err != nil {
		t.Fatalf("EnsureManagedRouterBinding() error = %v", err)
	}
	if result.Updated {
		t.Error("EnsureManagedRouterBinding() Updated = true, want false")
	}
	if result.RouterToken != token {
		t.Errorf("RouterToken = %q, want the installed URL token %q", result.RouterToken, token)
	}
	if stored := keystore.ReadRouterToken(paths.KeyFile); stored != token {
		t.Errorf("stored router token = %q, want %q", stored, token)
	}
}

func TestEnsureManagedRouterBindingRequiresManagedBlock(t *testing.T) {
	paths := newTestPaths(t)
	writeFile(t, paths.Config, "model = \"gpt-5.6-sol\"\n")

	_, err := EnsureManagedRouterBinding(paths, 10110)
	if err == nil || !strings.Contains(err.Error(), "run `dscodex install`") {
		t.Fatalf("EnsureManagedRouterBinding() error = %v, want the missing-block error", err)
	}
}
