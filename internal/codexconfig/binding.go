package codexconfig

import (
	"errors"
	"os"
	"strings"

	"github.com/VPpexis/dscodex-go/internal/constants"
	"github.com/VPpexis/dscodex-go/internal/keystore"
)

// RouterBinding is the result of reconciling the managed router binding.
type RouterBinding struct {
	RouterToken string
	Updated     bool
}

// EnsureManagedRouterBinding reconciles the managed router URL with the
// persisted token and port at every runtime entry point (start, serve,
// autostart). It refuses to publish an authenticated URL while a legacy router
// still owns the port, adopts the token already present in config.toml when
// the state file is missing, and rewrites the managed block only when it does
// not match.
func EnsureManagedRouterBinding(paths constants.Paths, port int) (RouterBinding, error) {
	if err := assertNoActiveLegacyRouter(paths); err != nil {
		return RouterBinding{}, err
	}
	original := ""
	data, err := os.ReadFile(paths.Config)
	switch {
	case err == nil:
		original = string(data)
	case errors.Is(err, os.ErrNotExist):
	default:
		return RouterBinding{}, err
	}
	if managedRootBlock(original) == nil {
		return RouterBinding{}, errors.New("DSCodex managed router config is missing; run `dscodex install`")
	}
	routerToken, err := keystore.EnsureRouterToken(paths.KeyFile, ReadManagedRouterToken(original))
	if err != nil {
		return RouterBinding{}, err
	}
	options := RouterOptions{Port: port, CatalogPath: paths.Catalog, RouterToken: routerToken}
	matched, err := ManagedRouterConfigMatches(original, options)
	if err != nil {
		return RouterBinding{}, err
	}
	updated := !matched
	if updated {
		configured, err := RewriteManagedRouterConfig(original, options)
		if err != nil {
			return RouterBinding{}, err
		}
		if !strings.HasSuffix(configured, "\n") {
			configured += "\n"
		}
		if err := atomicWrite(paths.Config, configured, 0o600); err != nil {
			return RouterBinding{}, err
		}
	}
	return RouterBinding{RouterToken: routerToken, Updated: updated}, nil
}
