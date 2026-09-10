package keystore

import (
	"errors"
	"os"
	"strings"
	"testing"
)

func TestRouterTokenIsStableAndSurvivesKeyDeletion(t *testing.T) {
	keyFile := newKeyFile(t)
	first, err := EnsureRouterToken(keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	if !routerTokenPattern.MatchString(first) {
		t.Fatalf("EnsureRouterToken() = %q, want a 43-character base64url token", first)
	}
	again, err := EnsureRouterToken(keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	if again != first {
		t.Errorf("EnsureRouterToken() changed from %q to %q", first, again)
	}
	if got := ReadRouterToken(keyFile); got != first {
		t.Errorf("ReadRouterToken() = %q, want %q", got, first)
	}
	if err := WriteStoredKey(keyFile, "sk-test-token"); err != nil {
		t.Fatal(err)
	}
	if err := DeleteStoredKey(keyFile); err != nil {
		t.Fatal(err)
	}
	if got := ReadRouterToken(keyFile); got != first {
		t.Errorf("token lost after key deletion: %q", got)
	}
}

func TestCreateRouterTokenIsRandom(t *testing.T) {
	first, err := CreateRouterToken()
	if err != nil {
		t.Fatal(err)
	}
	second, err := CreateRouterToken()
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Error("CreateRouterToken returned the same token twice")
	}
	if len(first) != 43 || !routerTokenPattern.MatchString(first) {
		t.Errorf("CreateRouterToken() = %q, want a 43-character base64url token", first)
	}
}

func TestWriteRouterTokenRejectsInvalidTokens(t *testing.T) {
	keyFile := newKeyFile(t)
	valid := strings.Repeat("A", 43)
	invalid := []string{
		"",
		"short",
		strings.Repeat("A", 42),
		strings.Repeat("A", 44),
		valid[:42] + "!",
		valid + "\n",
	}
	for _, token := range invalid {
		err := WriteRouterToken(keyFile, token)
		if err == nil || err.Error() != "Invalid DSCodex router token" {
			t.Errorf("WriteRouterToken(%q) error = %v, want %q", token, err, "Invalid DSCodex router token")
		}
	}
	if _, err := os.Stat(keyFile); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("invalid token wrote the config file (stat error = %v)", err)
	}
}

func TestWriteReadRouterTokenRoundTrip(t *testing.T) {
	keyFile := newKeyFile(t)
	token := strings.Repeat("a", 42) + "-"
	if err := WriteRouterToken(keyFile, token); err != nil {
		t.Fatal(err)
	}
	if got := ReadRouterToken(keyFile); got != token {
		t.Errorf("ReadRouterToken() = %q, want %q", got, token)
	}
}

func TestReadRouterTokenRejectsMalformedStoredValues(t *testing.T) {
	tests := []struct {
		name    string
		content string
	}{
		{"missing", `{}`},
		{"short", `{"router_token":"short"}`},
		{"long", `{"router_token":"` + strings.Repeat("A", 44) + `"}`},
		{"invalid characters", `{"router_token":"` + strings.Repeat("A", 42) + `!"}`},
		{"non-string", `{"router_token":42}`},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			keyFile := newKeyFile(t)
			writeConfigFile(t, keyFile, test.content)
			if got := ReadRouterToken(keyFile); got != "" {
				t.Errorf("ReadRouterToken() = %q, want empty", got)
			}
		})
	}
}

func TestEnsureRouterTokenPrefersValidPreferredToken(t *testing.T) {
	keyFile := newKeyFile(t)
	preferred := strings.Repeat("B", 43)
	got, err := EnsureRouterToken(keyFile, preferred)
	if err != nil {
		t.Fatal(err)
	}
	if got != preferred {
		t.Errorf("EnsureRouterToken() = %q, want %q", got, preferred)
	}
	if stored := ReadRouterToken(keyFile); stored != preferred {
		t.Errorf("ReadRouterToken() = %q, want %q", stored, preferred)
	}
}

func TestEnsureRouterTokenIgnoresInvalidPreferredToken(t *testing.T) {
	keyFile := newKeyFile(t)
	got, err := EnsureRouterToken(keyFile, "not-a-token")
	if err != nil {
		t.Fatal(err)
	}
	if !routerTokenPattern.MatchString(got) {
		t.Errorf("EnsureRouterToken() = %q, want a generated token", got)
	}
}

func TestEnsureRouterTokenKeepsExistingToken(t *testing.T) {
	keyFile := newKeyFile(t)
	first, err := EnsureRouterToken(keyFile, "")
	if err != nil {
		t.Fatal(err)
	}
	preferred := strings.Repeat("C", 43)
	got, err := EnsureRouterToken(keyFile, preferred)
	if err != nil {
		t.Fatal(err)
	}
	if got != first {
		t.Errorf("EnsureRouterToken() = %q, want the existing %q", got, first)
	}
}

func TestEnsureRouterTokenRejectsCorruptConfig(t *testing.T) {
	keyFile := newKeyFile(t)
	writeConfigFile(t, keyFile, "not json")
	_, err := EnsureRouterToken(keyFile, "")
	if err == nil || !strings.Contains(err.Error(), "Could not read or parse DSCodex config at") {
		t.Errorf("EnsureRouterToken(corrupt) error = %v, want the upstream message", err)
	}
}
