// Package keystore manages ~/.codex/dscodex/config.json, the router's private
// state file (DeepSeek API key, proxy URL, router token).
//
// It is the Go port of upstream src/keys.mjs. Writes are atomic (temporary
// file plus rename) and the file is created with mode 0600. Its contents are
// secret and must never be logged or included in error messages.
package keystore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// Router config field names; part of the on-disk contract.
const (
	fieldKey           = "deepseek_api_key"
	fieldKeyEncoding   = "key_encoding"
	fieldProxy         = "proxy_url"
	fieldProxyEncoding = "proxy_encoding"
	fieldRouterToken   = "router_token"
)

// Secret encoding markers.
const (
	encodingDPAPI = "dpapi"
	encodingPlain = "plain"
)

// ReadRouterConfig reads the router config file. A missing or unreadable
// file, or one that does not contain a JSON object, resolves to an empty
// config. With strict set, an unreadable file or non-object content returns
// an error that never includes file content.
//
// Error strings match the upstream implementation, including capitalization.
func ReadRouterConfig(keyFile string, strict bool) (map[string]any, error) {
	data, err := os.ReadFile(keyFile)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]any{}, nil
		}
		if strict {
			return nil, fmt.Errorf("Could not read or parse DSCodex config at %s", keyFile)
		}
		return map[string]any{}, nil
	}

	var parsed any
	if err := json.Unmarshal(data, &parsed); err != nil {
		if strict {
			return nil, fmt.Errorf("Could not read or parse DSCodex config at %s", keyFile)
		}
		return map[string]any{}, nil
	}

	config, ok := parsed.(map[string]any)
	if !ok {
		if strict {
			return nil, errors.New("DSCodex config must contain a JSON object")
		}
		return map[string]any{}, nil
	}
	return config, nil
}

// WriteRouterConfig atomically replaces the router config file. An empty
// config removes the file. The parent directory is created with mode 0700
// and the file with mode 0600.
func WriteRouterConfig(keyFile string, config map[string]any) error {
	if config == nil {
		return errors.New("DSCodex config must contain a JSON object")
	}
	if len(config) == 0 {
		if err := os.Remove(keyFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(keyFile), 0o700); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')

	temporary := fmt.Sprintf("%s.dscodex-tmp-%d", keyFile, os.Getpid())
	if err := os.WriteFile(temporary, data, 0o600); err != nil {
		return err
	}
	if err := os.Chmod(temporary, 0o600); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	if err := os.Rename(temporary, keyFile); err != nil {
		_ = os.Remove(temporary)
		return err
	}
	return nil
}
