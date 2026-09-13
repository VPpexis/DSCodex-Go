package proxycfg

import (
	"strings"
	"testing"
)

func envFrom(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestResolveProxyPrecedenceDSCODEXThenGenericEnvThenStored(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		stored string
		want   string
	}{
		{"DSCODEX https beats generic https", map[string]string{"DSCODEX_HTTPS_PROXY": "http://ds:1", "HTTPS_PROXY": "http://generic:2"}, "http://stored:3", "http://ds:1"},
		{"DSCODEX http beats generic http", map[string]string{"DSCODEX_HTTP_PROXY": "http://ds:1", "HTTP_PROXY": "http://generic:2"}, "http://stored:3", "http://ds:1"},
		{"generic HTTPS_PROXY beats stored", map[string]string{"HTTPS_PROXY": "http://generic:2"}, "http://stored:3", "http://generic:2"},
		{"generic HTTP_PROXY beats stored", map[string]string{"HTTP_PROXY": "http://generic:2"}, "http://stored:3", "http://generic:2"},
		{"lowercase https_proxy beats stored", map[string]string{"https_proxy": "http://lower:4"}, "http://stored:3", "http://lower:4"},
		{"lowercase beats uppercase", map[string]string{"HTTPS_PROXY": "http://upper:5", "https_proxy": "http://lower:4"}, "http://stored:3", "http://lower:4"},
		{"stored is trimmed", map[string]string{}, "  http://stored:3  ", "http://stored:3"},
		{"no source resolves to empty", map[string]string{}, "", ""},
		{"whitespace-only env resolves to empty", map[string]string{"HTTPS_PROXY": "   "}, "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ResolveProxy(envFrom(test.env), test.stored); got != test.want {
				t.Errorf("ResolveProxy(%v, %q) = %q, want %q", test.env, test.stored, got, test.want)
			}
		})
	}
}

func TestProxySourceNamesTheWinningSource(t *testing.T) {
	tests := []struct {
		name   string
		env    map[string]string
		stored string
		want   string
	}{
		{"DSCODEX env", map[string]string{"DSCODEX_HTTPS_PROXY": "x"}, "", "DSCODEX_*_PROXY env"},
		{"generic env", map[string]string{"HTTPS_PROXY": "x"}, "", "HTTP(S)_PROXY env"},
		{"stored value", map[string]string{}, "http://stored", "stored in config.json"},
		{"no source", map[string]string{}, "", ""},
		{"whitespace-only stored", map[string]string{}, "   ", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ProxySource(envFrom(test.env), test.stored); got != test.want {
				t.Errorf("ProxySource(%v, %q) = %q, want %q", test.env, test.stored, got, test.want)
			}
		})
	}
}

func TestValidateAndRedactCredentialedProxyURLs(t *testing.T) {
	credentialed := "https://user:REDACTED@example.test:8443"
	validated, err := ValidateProxyURL(" " + credentialed + " ")
	if err != nil || validated != credentialed {
		t.Errorf("ValidateProxyURL() = %q, %v; want the trimmed URL and no error", validated, err)
	}

	redacted := RedactProxyURL(credentialed)
	if !strings.Contains(redacted, "redacted") {
		t.Errorf("RedactProxyURL() = %q, want a redacted userinfo", redacted)
	}
	if strings.Contains(redacted, "user") || strings.Contains(redacted, "REDACTED") {
		t.Errorf("RedactProxyURL() leaked credentials: %q", redacted)
	}

	query := RedactProxyURL("https://example.test/?token=secret")
	if strings.Contains(query, "secret") || !strings.Contains(query, "redacted") {
		t.Errorf("RedactProxyURL() query = %q, want the query redacted", query)
	}

	fragment := RedactProxyURL("https://example.test/#token-secret")
	if strings.Contains(fragment, "secret") || !strings.Contains(fragment, "redacted") {
		t.Errorf("RedactProxyURL() fragment = %q, want the fragment redacted", fragment)
	}

	if got := RedactProxyURL("not a url"); got != "<invalid proxy URL>" {
		t.Errorf("RedactProxyURL(invalid) = %q, want %q", got, "<invalid proxy URL>")
	}

	for _, value := range []string{"", "not a url", "http://", "file:///tmp/proxy", "ftp://proxy.test"} {
		if _, err := ValidateProxyURL(value); err == nil || !strings.Contains(err.Error(), "Invalid proxy URL") {
			t.Errorf("ValidateProxyURL(%q) error = %v, want the invalid-proxy error", value, err)
		}
	}

	if _, err := ValidateProxyURL("HTTP://proxy.example:8080"); err != nil {
		t.Errorf("ValidateProxyURL(uppercase scheme) error = %v, want nil", err)
	}
}

func TestMergeNoProxyCaseInsensitiveAndDeduplicated(t *testing.T) {
	if got, want := MergeNoProxy("example.com"), "example.com,127.0.0.1,localhost,::1,api.deepseek.com"; got != want {
		t.Errorf("MergeNoProxy(example.com) = %q, want %q", got, want)
	}
	if got, want := MergeNoProxy(""), "127.0.0.1,localhost,::1,api.deepseek.com"; got != want {
		t.Errorf("MergeNoProxy(empty) = %q, want %q", got, want)
	}
	if got, want := MergeNoProxy("  a , ,b "), "a,b,127.0.0.1,localhost,::1,api.deepseek.com"; got != want {
		t.Errorf("MergeNoProxy(trim) = %q, want %q", got, want)
	}

	existing := MergeNoProxy("API.DEEPSEEK.COM")
	if !strings.HasPrefix(existing, "API.DEEPSEEK.COM,") {
		t.Errorf("MergeNoProxy() = %q, want the existing entry first and case preserved", existing)
	}
	count := 0
	for _, part := range strings.Split(strings.ToLower(existing), ",") {
		if part == "api.deepseek.com" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("MergeNoProxy() = %q, want api.deepseek.com exactly once (case-insensitive)", existing)
	}
}
