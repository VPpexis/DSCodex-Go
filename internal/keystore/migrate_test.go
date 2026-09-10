package keystore

import (
	"runtime"
	"strings"
	"testing"
)

func TestMigratesLegacyWindowsPlaintextKeysBeforeServing(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy"}`)
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

func TestStateWritesMigrateLegacyWindowsPlaintextKeysImmediately(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy-write"}`)
	if err := WriteProxyURL(keyFile, "http://127.0.0.1:10808"); err != nil {
		t.Fatal(err)
	}
	if got := ReadStoredKey(keyFile); got != "sk-test-legacy-write" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-test-legacy-write")
	}
	if strings.Contains(readConfigFile(t, keyFile), "sk-test-legacy-write") {
		t.Error("legacy plaintext key still present after a state-changing write")
	}
}

func TestTokenWritesMigrateLegacyWindowsPlaintextKeys(t *testing.T) {
	if runtime.GOOS != "windows" {
		return
	}
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy-token"}`)
	if err := WriteRouterToken(keyFile, strings.Repeat("D", 43)); err != nil {
		t.Fatal(err)
	}
	if got := ReadStoredKey(keyFile); got != "sk-test-legacy-token" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-test-legacy-token")
	}
	if strings.Contains(readConfigFile(t, keyFile), "sk-test-legacy-token") {
		t.Error("legacy plaintext key still present after a token write")
	}
}

func TestMigrateLegacyStoredKeyIsNoOpOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy"}`)
	migrated, err := MigrateLegacyStoredKey(keyFile)
	if err != nil {
		t.Fatalf("MigrateLegacyStoredKey() error = %v", err)
	}
	if migrated {
		t.Error("MigrateLegacyStoredKey() = true off Windows, want false")
	}
	if !strings.Contains(readConfigFile(t, keyFile), "sk-test-legacy") {
		t.Error("legacy key changed off Windows")
	}
}
