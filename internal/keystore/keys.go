package keystore

import (
	"errors"
	"os"
	"strings"
)

// ReadStoredKey returns the DeepSeek API key stored in keyFile, decrypting it
// when it is DPAPI-protected. A missing file, corrupt config, or failed
// decryption resolves to an empty string.
func ReadStoredKey(keyFile string) string {
	config, err := ReadRouterConfig(keyFile, false)
	if err != nil {
		return ""
	}
	stored, ok := config[fieldKey].(string)
	if !ok {
		return ""
	}
	stored = strings.TrimSpace(stored)
	if stored == "" {
		return ""
	}
	// Legacy files written before key_encoding existed store the plaintext.
	if encoding, _ := config[fieldKeyEncoding].(string); encoding == encodingDPAPI {
		plain, err := dpapiUnprotect(stored)
		if err != nil {
			return ""
		}
		return plain
	}
	return stored
}

// WriteStoredKey stores the DeepSeek API key in keyFile, DPAPI-protecting it
// on Windows. The key is trimmed first; an empty key is rejected.
func WriteStoredKey(keyFile, key string) error {
	trimmed := strings.TrimSpace(key)
	if trimmed == "" {
		return errors.New("Empty DeepSeek API key")
	}
	stored := trimmed
	encoding := encodingPlain
	if dpapiAvailable {
		protected, err := dpapiProtect(trimmed)
		if err != nil {
			return err
		}
		stored = protected
		encoding = encodingDPAPI
	}
	config, err := ReadRouterConfig(keyFile, true)
	if err != nil {
		return err
	}
	config[fieldKey] = stored
	config[fieldKeyEncoding] = encoding
	return WriteRouterConfig(keyFile, config)
}

// ReadProxyURL returns the proxy URL stored in keyFile, decrypting it when it
// is DPAPI-protected. Unreadable or undecryptable values resolve to "".
func ReadProxyURL(keyFile string) string {
	config, err := ReadRouterConfig(keyFile, false)
	if err != nil {
		return ""
	}
	value, ok := config[fieldProxy].(string)
	if !ok {
		return ""
	}
	if encoding, _ := config[fieldProxyEncoding].(string); encoding == encodingDPAPI {
		plain, err := dpapiUnprotect(value)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(plain)
	}
	return strings.TrimSpace(value)
}

// WriteProxyURL stores proxyURL in keyFile, DPAPI-protecting it on Windows.
// An empty or whitespace-only URL removes the stored proxy.
func WriteProxyURL(keyFile, proxyURL string) error {
	trimmed := strings.TrimSpace(proxyURL)
	config, err := ReadRouterConfig(keyFile, true)
	if err != nil {
		return err
	}
	if trimmed != "" {
		stored := trimmed
		encoding := encodingPlain
		if dpapiAvailable {
			protected, err := dpapiProtect(trimmed)
			if err != nil {
				return err
			}
			stored = protected
			encoding = encodingDPAPI
		}
		config[fieldProxy] = stored
		config[fieldProxyEncoding] = encoding
	} else {
		delete(config, fieldProxy)
		delete(config, fieldProxyEncoding)
	}
	return WriteRouterConfig(keyFile, config)
}

// DeleteStoredKey removes the DeepSeek API key and its encoding marker from
// keyFile, deleting the file when no other state remains.
func DeleteStoredKey(keyFile string) error {
	config, err := ReadRouterConfig(keyFile, true)
	if err != nil {
		return err
	}
	delete(config, fieldKey)
	delete(config, fieldKeyEncoding)
	if len(config) == 0 {
		if err := os.Remove(keyFile); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		return nil
	}
	return WriteRouterConfig(keyFile, config)
}
