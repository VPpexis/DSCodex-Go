package keystore

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func newKeyFile(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "dscodex", "config.json")
}

func writeConfigFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func readConfigFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestStoredKeyRoundTripsWithOwnerOnlyPermissions(t *testing.T) {
	keyFile := newKeyFile(t)
	if err := WriteStoredKey(keyFile, "  sk-test-123  "); err != nil {
		t.Fatalf("WriteStoredKey() error = %v", err)
	}
	if got := ReadStoredKey(keyFile); got != "sk-test-123" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-test-123")
	}
	// Windows has no POSIX mode bits; the file is protected by the account
	// ACL and, since the DPAPI change, the ciphertext itself.
	if runtime.GOOS != "windows" {
		info, err := os.Stat(keyFile)
		if err != nil {
			t.Fatal(err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("key file mode = %o, want 600", perm)
		}
	}
}

func TestKeyFileNeverStoresPlaintextOnWindows(t *testing.T) {
	keyFile := newKeyFile(t)
	if err := WriteStoredKey(keyFile, "sk-test-123"); err != nil {
		t.Fatal(err)
	}
	raw := readConfigFile(t, keyFile)
	if runtime.GOOS == "windows" {
		if strings.Contains(raw, "sk-test-123") {
			t.Error("key file contains the plaintext API key on Windows")
		}
	} else if !strings.Contains(raw, "sk-test-123") {
		t.Error("key file does not contain the plaintext API key on POSIX")
	}
	if got := ReadStoredKey(keyFile); got != "sk-test-123" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-test-123")
	}
}

func TestMissingOrCorruptKeyFilesResolveToEmpty(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "config.json")
	if got := ReadStoredKey(keyFile); got != "" {
		t.Errorf("ReadStoredKey(missing) = %q, want empty", got)
	}
	writeConfigFile(t, keyFile, "not json")
	if got := ReadStoredKey(keyFile); got != "" {
		t.Errorf("ReadStoredKey(corrupt) = %q, want empty", got)
	}
	writeConfigFile(t, keyFile, `{"unrelated":true}`)
	if got := ReadStoredKey(keyFile); got != "" {
		t.Errorf("ReadStoredKey(unrelated) = %q, want empty", got)
	}
}

func TestWriteStoredKeyRejectsEmptyKey(t *testing.T) {
	keyFile := newKeyFile(t)
	err := WriteStoredKey(keyFile, "   ")
	if err == nil || err.Error() != "Empty DeepSeek API key" {
		t.Fatalf("WriteStoredKey(blank) error = %v, want %q", err, "Empty DeepSeek API key")
	}
	if _, statErr := os.Stat(keyFile); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("key file exists after rejected write (stat error = %v)", statErr)
	}
}

func TestDeleteStoredKeyRemovesTheFile(t *testing.T) {
	keyFile := newKeyFile(t)
	if err := WriteStoredKey(keyFile, "sk-x"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteStoredKey(keyFile); err != nil {
		t.Fatalf("DeleteStoredKey() error = %v", err)
	}
	if _, err := os.Stat(keyFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("key file still exists after deletion (stat error = %v)", err)
	}
}

func TestProxySettingSurvivesKeyWritesAndKeyDeletion(t *testing.T) {
	keyFile := newKeyFile(t)
	if err := WriteProxyURL(keyFile, "  http://127.0.0.1:10808  "); err != nil {
		t.Fatal(err)
	}
	if got := ReadProxyURL(keyFile); got != "http://127.0.0.1:10808" {
		t.Errorf("ReadProxyURL() = %q, want %q", got, "http://127.0.0.1:10808")
	}
	if runtime.GOOS == "windows" && strings.Contains(readConfigFile(t, keyFile), "http://127.0.0.1:10808") {
		t.Error("proxy file contains the plaintext URL on Windows")
	}

	if err := WriteStoredKey(keyFile, "sk-proxy-test"); err != nil {
		t.Fatal(err)
	}
	if got := ReadProxyURL(keyFile); got != "http://127.0.0.1:10808" {
		t.Errorf("proxy lost after key write: %q", got)
	}
	if got := ReadStoredKey(keyFile); got != "sk-proxy-test" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-proxy-test")
	}

	if err := DeleteStoredKey(keyFile); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(keyFile); err != nil {
		t.Errorf("proxy-only config should survive key deletion: %v", err)
	}
	if got := ReadProxyURL(keyFile); got != "http://127.0.0.1:10808" {
		t.Errorf("proxy lost after key deletion: %q", got)
	}

	if err := WriteProxyURL(keyFile, ""); err != nil {
		t.Fatal(err)
	}
	if got := ReadProxyURL(keyFile); got != "" {
		t.Errorf("ReadProxyURL() = %q, want empty", got)
	}
	if _, err := os.Stat(keyFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("empty config file still exists (stat error = %v)", err)
	}
}

func TestReadStoredKeyReadsLegacyPlaintext(t *testing.T) {
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-legacy-key"}`)
	if got := ReadStoredKey(keyFile); got != "sk-test-legacy-key" {
		t.Errorf("ReadStoredKey() = %q, want %q", got, "sk-test-legacy-key")
	}
}

func TestDPAPIEncodedValuesUnreadableOffWindows(t *testing.T) {
	if runtime.GOOS == "windows" {
		return
	}
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, `{"deepseek_api_key":"sk-test-dpapi-blob","key_encoding":"dpapi","proxy_url":"sk-test-proxy-blob","proxy_encoding":"dpapi"}`)
	if got := ReadStoredKey(keyFile); got != "" {
		t.Errorf("ReadStoredKey(dpapi off Windows) = %q, want empty", got)
	}
	if got := ReadProxyURL(keyFile); got != "" {
		t.Errorf("ReadProxyURL(dpapi off Windows) = %q, want empty", got)
	}
}
