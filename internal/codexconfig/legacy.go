package codexconfig

import (
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"regexp"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

func assertNoActiveLegacyRouter(paths constants.Paths) error {
	data, err := os.ReadFile(paths.PID)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("Invalid DSCodex pid state at %s; verify no router is running and remove the file before installing", paths.PID)
	}
	state, _ := parsed.(map[string]any)
	if authenticatedPidState(state) {
		return nil
	}
	pid, pidOK := integerField(state, "pid")
	port, portOK := integerField(state, "port")
	if !pidOK || pid < 1 || !portOK || port < 1 || port > 65535 {
		return fmt.Errorf("Untrusted DSCodex pid state at %s; verify no router is running and remove the file before installing", paths.PID)
	}
	if !processAppearsAlive(int(pid)) {
		return nil
	}
	return fmt.Errorf("A router from an older or untrusted DSCodex state is still running (PID %d, port %d); stop it with the previous DSCodex version before installing", int(pid), int(port))
}

func authenticatedPidState(state map[string]any) bool {
	pid, pidOK := integerField(state, "pid")
	port, portOK := integerField(state, "port")
	if !pidOK || pid < 1 || !portOK || port < 1 || port > 65535 {
		return false
	}
	routerToken, _ := state["routerToken"].(string)
	shutdownToken, _ := state["shutdownToken"].(string)
	if !validRouterToken(routerToken) || !validRouterToken(shutdownToken) {
		return false
	}
	instanceID, ok := state["instanceId"].(string)
	if !ok {
		return false
	}
	pattern := regexp.MustCompile(fmt.Sprintf(`^%d-\d+-[0-9a-f]{16}$`, int(pid)))
	return pattern.MatchString(instanceID)
}

func integerField(state map[string]any, key string) (float64, bool) {
	value, ok := state[key].(float64)
	if !ok || math.Trunc(value) != value {
		return 0, false
	}
	return value, true
}
