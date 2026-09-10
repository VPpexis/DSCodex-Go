package keystore

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"regexp"
)

// routerTokenPattern is the on-disk contract for router tokens: 32 random
// bytes encoded as unpadded base64url, which is exactly 43 characters.
var routerTokenPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43}$`)

// CreateRouterToken returns a fresh 43-character router token: 32 random
// bytes encoded as unpadded base64url.
func CreateRouterToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf), nil
}

// ReadRouterToken returns the stored router token, or "" when it is missing
// or does not match the 43-character base64url contract.
func ReadRouterToken(keyFile string) string {
	config, err := ReadRouterConfig(keyFile, false)
	if err != nil {
		return ""
	}
	value, ok := config[fieldRouterToken].(string)
	if !ok || !routerTokenPattern.MatchString(value) {
		return ""
	}
	return value
}

// WriteRouterToken stores token in keyFile. Tokens must match the
// 43-character base64url contract; anything else is rejected. State-changing
// writes also migrate a legacy plaintext Windows key.
func WriteRouterToken(keyFile, token string) error {
	if !routerTokenPattern.MatchString(token) {
		return errors.New("Invalid DSCodex router token")
	}
	if _, err := MigrateLegacyStoredKey(keyFile); err != nil {
		return err
	}
	config, err := ReadRouterConfig(keyFile, true)
	if err != nil {
		return err
	}
	config[fieldRouterToken] = token
	return WriteRouterConfig(keyFile, config)
}

// EnsureRouterToken returns the stored router token, creating one when the
// stored value is missing or invalid. A valid preferredToken (for example one
// recovered from config.toml) wins over generating a random token.
func EnsureRouterToken(keyFile, preferredToken string) (string, error) {
	config, err := ReadRouterConfig(keyFile, true)
	if err != nil {
		return "", err
	}
	if existing, ok := config[fieldRouterToken].(string); ok && routerTokenPattern.MatchString(existing) {
		if _, err := MigrateLegacyStoredKey(keyFile); err != nil {
			return "", err
		}
		return existing, nil
	}
	token := preferredToken
	if !routerTokenPattern.MatchString(token) {
		token, err = CreateRouterToken()
		if err != nil {
			return "", err
		}
	}
	if err := WriteRouterToken(keyFile, token); err != nil {
		return "", err
	}
	return token, nil
}
