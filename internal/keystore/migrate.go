package keystore

import "strings"

// MigrateLegacyStoredKey re-encrypts a legacy plaintext key on Windows and
// reports whether it did. On other platforms it is a no-op.
func MigrateLegacyStoredKey(keyFile string) (bool, error) {
	if !dpapiAvailable {
		return false, nil
	}
	config, err := ReadRouterConfig(keyFile, false)
	if err != nil {
		return false, nil
	}
	if encodingBlocksMigration(config[fieldKeyEncoding]) {
		return false, nil
	}
	legacy, ok := config[fieldKey].(string)
	if !ok || strings.TrimSpace(legacy) == "" {
		return false, nil
	}
	if err := WriteStoredKey(keyFile, legacy); err != nil {
		return false, err
	}
	return true, nil
}

// encodingBlocksMigration reports whether a stored key_encoding value stops
// legacy migration. Missing, null, false, 0, and "" do not, and neither does
// "plain"; "dpapi" and any other non-empty value do. This mirrors the
// upstream truthiness checks.
func encodingBlocksMigration(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed == encodingDPAPI || (typed != "" && typed != encodingPlain)
	case bool:
		return typed
	case float64:
		return typed != 0
	default:
		return true
	}
}
