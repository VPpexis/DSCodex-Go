//go:build windows

package keystore

import (
	"strings"
	"testing"
)

func TestMigrateLegacyStoredKeyBlocksOnEncoding(t *testing.T) {
	tests := []struct {
		name     string
		encoding string
	}{
		{"dpapi", `"dpapi"`},
		{"other string", `"base64"`},
		{"number", `42`},
		{"boolean", `true`},
		{"object", `{"nested":true}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keyFile := newKeyFile(t)
			writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy","key_encoding":`+test.encoding+`}`)
			migrated, err := MigrateLegacyStoredKey(keyFile)
			if err != nil {
				t.Fatalf("MigrateLegacyStoredKey() error = %v", err)
			}
			if migrated {
				t.Error("MigrateLegacyStoredKey() = true, want false")
			}
			if !strings.Contains(readConfigFile(t, keyFile), "sk-test-legacy") {
				t.Error("key changed despite a blocking encoding")
			}
		})
	}
}

func TestMigrateLegacyStoredKeyPlainEncodingMigrates(t *testing.T) {
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy","key_encoding":"plain"}`)
	migrated, err := MigrateLegacyStoredKey(keyFile)
	if err != nil {
		t.Fatalf("MigrateLegacyStoredKey() error = %v", err)
	}
	if !migrated {
		t.Error("MigrateLegacyStoredKey() = false, want true")
	}
	if got := ReadStoredKey(keyFile); got != "sk-test-legacy" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-test-legacy")
	}
	if strings.Contains(readConfigFile(t, keyFile), "sk-test-legacy") {
		t.Error("legacy plaintext key still present after migration")
	}
}
