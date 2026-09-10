//go:build windows

package keystore

import (
	"encoding/base64"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestDPAPIRoundTrip(t *testing.T) {
	values := []string{"sk-test-dpapi", "clé-Ω-ünïcode", strings.Repeat("x", 4096)}
	for _, value := range values {
		protected, err := dpapiProtect(value)
		if err != nil {
			t.Fatalf("dpapiProtect(%q) error = %v", value, err)
		}
		if _, err := base64.StdEncoding.DecodeString(protected); err != nil {
			t.Errorf("dpapiProtect(%q) = %q, not valid base64: %v", value, protected, err)
		}
		plain, err := dpapiUnprotect(protected)
		if err != nil {
			t.Fatalf("dpapiUnprotect() error = %v", err)
		}
		if plain != value {
			t.Errorf("DPAPI round trip = %q, want %q", plain, value)
		}
	}
}

func TestDPAPIProtectRandomizes(t *testing.T) {
	first, err := dpapiProtect("sk-test-random")
	if err != nil {
		t.Fatal(err)
	}
	second, err := dpapiProtect("sk-test-random")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("dpapiProtect returned identical ciphertext twice")
	}
}

func TestDPAPIUnprotectRejectsInvalidInput(t *testing.T) {
	inputs := []string{
		"",
		"not-base64!!",
		base64.StdEncoding.EncodeToString([]byte("garbage")),
	}
	for _, input := range inputs {
		if _, err := dpapiUnprotect(input); err == nil {
			t.Errorf("dpapiUnprotect(%q) error = nil, want error", input)
		}
	}
}

func TestDPAPIInteroperatesWithUpstreamPowerShell(t *testing.T) {
	powershell := filepath.Join(os.Getenv("SystemRoot"), "System32", "WindowsPowerShell", "v1.0", "powershell.exe")
	if _, err := os.Stat(powershell); err != nil {
		t.Skipf("Windows PowerShell not available: %v", err)
	}
	const value = "sk-test-interop"

	upstreamBlob := runPowerShellDPAPI(t, powershell, "protect", value)
	plain, err := dpapiUnprotect(upstreamBlob)
	if err != nil {
		t.Fatalf("dpapiUnprotect(upstream blob) error = %v", err)
	}
	if plain != value {
		t.Errorf("dpapiUnprotect(upstream blob) = %q, want %q", plain, value)
	}

	ourBlob, err := dpapiProtect(value)
	if err != nil {
		t.Fatal(err)
	}
	if decoded := runPowerShellDPAPI(t, powershell, "unprotect", ourBlob); decoded != value {
		t.Errorf("upstream unprotect(our blob) = %q, want %q", decoded, value)
	}
}

func runPowerShellDPAPI(t *testing.T, powershell, operation, input string) string {
	t.Helper()
	method := "Protect"
	encoded := base64.StdEncoding.EncodeToString([]byte(input))
	if operation == "unprotect" {
		method = "Unprotect"
		encoded = input
	}
	script := "$inputText = [Console]::In.ReadToEnd().Trim(); " +
		"$bytes = [Convert]::FromBase64String($inputText); " +
		"Add-Type -AssemblyName System.Security; " +
		"$result = [System.Security.Cryptography.ProtectedData]::" + method +
		"($bytes, $null, [System.Security.Cryptography.DataProtectionScope]::CurrentUser); " +
		"[Console]::Out.Write([Convert]::ToBase64String($result))"
	cmd := exec.Command(powershell, "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Stdin = strings.NewReader(encoded + "\n")
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("powershell %s failed: %v", operation, err)
	}
	result := strings.TrimSpace(string(out))
	if operation == "unprotect" {
		decoded, err := base64.StdEncoding.DecodeString(result)
		if err != nil {
			t.Fatalf("powershell unprotect returned invalid base64: %v", err)
		}
		return string(decoded)
	}
	return result
}
