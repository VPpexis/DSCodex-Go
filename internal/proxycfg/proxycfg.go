// Package proxycfg resolves the loopback router's upstream proxy.
//
// It is the Go port of upstream src/proxy-config.mjs. Node's global fetch
// (undici) ignores HTTP_PROXY / HTTPS_PROXY by default, so upstream resolves a
// proxy explicitly and re-execs itself with --use-env-proxy; the Go router
// supports environment proxies natively, so only the resolution, validation,
// redaction, and NO_PROXY semantics are ported.
package proxycfg

import (
	"errors"
	"net/url"
	"os"
	"strings"
)

// DirectNoProxy lists the hosts that must never be reached through a proxy:
// the loopback router and the DeepSeek API.
var DirectNoProxy = []string{"127.0.0.1", "localhost", "::1", "api.deepseek.com"}

// proxyEnvNames is the resolution order: DSCodex-specific names first, then
// the standard names with the lowercase variants winning.
var proxyEnvNames = []string{
	"DSCODEX_HTTPS_PROXY",
	"DSCODEX_HTTP_PROXY",
	"dscodex_https_proxy",
	"dscodex_http_proxy",
	"https_proxy",
	"HTTPS_PROXY",
	"http_proxy",
	"HTTP_PROXY",
}

func valueFromEnv(lookup func(string) string, name string) string {
	if lookup == nil {
		lookup = os.Getenv
	}
	return strings.TrimSpace(lookup(name))
}

// ResolveProxy returns the proxy URL to use: the first non-empty environment
// variable in proxyEnvNames, or the stored value when the environment has
// none. Every candidate is trimmed.
func ResolveProxy(lookup func(string) string, stored string) string {
	for _, name := range proxyEnvNames {
		if candidate := valueFromEnv(lookup, name); candidate != "" {
			return candidate
		}
	}
	return strings.TrimSpace(stored)
}

// ProxySource names the winning source of ResolveProxy: the DSCODEX_* env
// pair, the standard HTTP(S)_PROXY env names, the stored value, or "".
func ProxySource(lookup func(string) string, stored string) string {
	for _, name := range proxyEnvNames[:4] {
		if valueFromEnv(lookup, name) != "" {
			return "DSCODEX_*_PROXY env"
		}
	}
	for _, name := range proxyEnvNames[4:] {
		if valueFromEnv(lookup, name) != "" {
			return "HTTP(S)_PROXY env"
		}
	}
	if strings.TrimSpace(stored) != "" {
		return "stored in config.json"
	}
	return ""
}

// ValidateProxyURL returns the trimmed proxy URL when it is an http:// or
// https:// URL with a hostname, and the upstream error otherwise.
func ValidateProxyURL(value string) (string, error) {
	trimmed := strings.TrimSpace(value)
	parsed, err := url.Parse(trimmed)
	if err != nil || !isHTTP(parsed) || parsed.Hostname() == "" {
		return "", errors.New("Invalid proxy URL (expected http:// or https:// with a hostname)")
	}
	return trimmed, nil
}

func isHTTP(parsed *url.URL) bool {
	return strings.EqualFold(parsed.Scheme, "http") || strings.EqualFold(parsed.Scheme, "https")
}

// RedactProxyURL hides proxy credentials, the query, and the fragment for CLI
// output. Values that do not parse as an absolute URL become
// "<invalid proxy URL>", mirroring the WHATWG URL constructor upstream.
func RedactProxyURL(value string) string {
	parsed, err := url.Parse(value)
	if err != nil || !parsed.IsAbs() {
		return "<invalid proxy URL>"
	}
	if parsed.User != nil {
		username := parsed.User.Username()
		_, hasPassword := parsed.User.Password()
		switch {
		case username != "" && hasPassword:
			parsed.User = url.UserPassword("redacted", "redacted")
		case username != "":
			parsed.User = url.User("redacted")
		case hasPassword:
			parsed.User = url.UserPassword("", "redacted")
		}
	}
	if parsed.RawQuery != "" {
		parsed.RawQuery = "redacted"
	}
	if parsed.Fragment != "" {
		parsed.Fragment = "redacted"
	}
	return parsed.String()
}

// MergeNoProxy preserves the existing NO_PROXY entries (order and case) and
// appends the DirectNoProxy hosts that are missing, comparing
// case-insensitively.
func MergeNoProxy(existing string) string {
	parts := make([]string, 0, len(DirectNoProxy))
	seen := map[string]bool{}
	for _, part := range strings.Split(existing, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		parts = append(parts, part)
		seen[strings.ToLower(part)] = true
	}
	for _, value := range DirectNoProxy {
		if !seen[strings.ToLower(value)] {
			parts = append(parts, value)
			seen[strings.ToLower(value)] = true
		}
	}
	return strings.Join(parts, ",")
}
