package codexconfig

import (
	"strings"
	"testing"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

func routerToken() string {
	return strings.Repeat("A", 43)
}

func TestManagedRootBlock(t *testing.T) {
	token := routerToken()
	tests := []struct {
		name       string
		content    string
		wantBlock  bool
		wantStart  int
		wantEnd    int
		wantValues map[string]string
	}{
		{
			name:      "marker followed by both root keys",
			content:   "model = \"gpt-5.6-sol\"\n" + constants.ManagedMarker + "\nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\nmodel_catalog_json = \"/tmp/models.json\"\n[features]\n",
			wantBlock: true,
			wantStart: 1,
			wantEnd:   4,
			wantValues: map[string]string{
				"openai_base_url":    "http://127.0.0.1:10110/" + token + "/v1",
				"model_catalog_json": "/tmp/models.json",
			},
		},
		{
			name:      "marker followed by a single root key",
			content:   constants.ManagedMarker + "\nmodel_catalog_json = \"/tmp/models.json\"\n",
			wantBlock: true,
			wantStart: 0,
			wantEnd:   2,
			wantValues: map[string]string{
				"model_catalog_json": "/tmp/models.json",
			},
		},
		{
			name:      "marker followed by a blank line is not a block",
			content:   constants.ManagedMarker + "\n\nopenai_base_url = \"http://127.0.0.1/v1\"\n",
			wantBlock: false,
		},
		{
			name:      "second marker wins when the first is not followed by root keys",
			content:   constants.ManagedMarker + "\ntheme = \"light\"\n" + constants.ManagedMarker + "\nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\n",
			wantBlock: true,
			wantStart: 2,
			wantEnd:   4,
			wantValues: map[string]string{
				"openai_base_url": "http://127.0.0.1:10110/" + token + "/v1",
			},
		},
		{
			name:      "CRLF is normalized before matching",
			content:   constants.ManagedMarker + "\r\nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\r\n",
			wantBlock: true,
			wantStart: 0,
			wantEnd:   2,
			wantValues: map[string]string{
				"openai_base_url": "http://127.0.0.1:10110/" + token + "/v1",
			},
		},
		{
			name:      "duplicate keys keep the last value",
			content:   constants.ManagedMarker + "\nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\nopenai_base_url = \"http://127.0.0.1:10110/" + strings.Repeat("B", 43) + "/v1\"\n",
			wantBlock: true,
			wantStart: 0,
			wantEnd:   3,
			wantValues: map[string]string{
				"openai_base_url": "http://127.0.0.1:10110/" + strings.Repeat("B", 43) + "/v1",
			},
		},
		{
			name:      "unquoted values are not parsed",
			content:   constants.ManagedMarker + "\nopenai_base_url = http://127.0.0.1:10110/" + token + "/v1\n",
			wantBlock: true,
			wantStart: 0,
			wantEnd:   2,
			wantValues: map[string]string{
				"openai_base_url": "",
			},
		},
		{
			name:      "marker line may carry surrounding whitespace",
			content:   "  " + constants.ManagedMarker + "  \nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\n",
			wantBlock: true,
			wantStart: 0,
			wantEnd:   2,
			wantValues: map[string]string{
				"openai_base_url": "http://127.0.0.1:10110/" + token + "/v1",
			},
		},
		{
			name:      "marker must match exactly",
			content:   constants.ManagedMarker + " extra\nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\n",
			wantBlock: false,
		},
		{
			name:      "root keys without a marker are ignored",
			content:   "openai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\nmodel_catalog_json = \"/tmp/models.json\"\n",
			wantBlock: false,
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			block := managedRootBlock(test.content)
			if !test.wantBlock {
				if block != nil {
					t.Fatalf("managedRootBlock() = %+v, want nil", block)
				}
				return
			}
			if block == nil {
				t.Fatal("managedRootBlock() = nil, want a block")
			}
			if block.start != test.wantStart || block.end != test.wantEnd {
				t.Errorf("managedRootBlock() start/end = %d/%d, want %d/%d", block.start, block.end, test.wantStart, test.wantEnd)
			}
			for key, want := range test.wantValues {
				if got := block.values[key]; got != want {
					t.Errorf("managedRootBlock() values[%q] = %q, want %q", key, got, want)
				}
			}
		})
	}
}

func TestReadManagedRouterToken(t *testing.T) {
	token := routerToken()
	block := func(value string) string {
		return constants.ManagedMarker + "\nopenai_base_url = " + quoteToml(value) + "\n"
	}
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{"valid URL", block("http://127.0.0.1:10110/" + token + "/v1"), token},
		{"trailing slash", block("http://127.0.0.1:10110/" + token + "/v1/"), token},
		{"port omitted", block("http://127.0.0.1/" + token + "/v1"), token},
		{"https rejected", block("https://127.0.0.1:10110/" + token + "/v1"), ""},
		{"foreign host rejected", block("http://example.test:10110/" + token + "/v1"), ""},
		{"userinfo rejected", block("http://user@" + constants.Host + ":10110/" + token + "/v1"), ""},
		{"query rejected", block("http://127.0.0.1:10110/" + token + "/v1?x=1"), ""},
		{"fragment rejected", block("http://127.0.0.1:10110/" + token + "/v1#x"), ""},
		{"short token rejected", block("http://127.0.0.1:10110/" + strings.Repeat("A", 42) + "/v1"), ""},
		{"invalid token characters rejected", block("http://127.0.0.1:10110/" + strings.Repeat("A", 42) + "!/v1"), ""},
		{"missing v1 rejected", block("http://127.0.0.1:10110/" + token + "/v2"), ""},
		{"percent-encoded token rejected", block("http://127.0.0.1:10110/%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41%41/v1"), ""},
		{"non-URL value rejected", block("not a url"), ""},
		{"missing block", "model = \"gpt-5.6-sol\"\n", ""},
		{"missing base URL value", constants.ManagedMarker + "\nmodel_catalog_json = \"/tmp/models.json\"\n", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := ReadManagedRouterToken(test.content); got != test.want {
				t.Errorf("ReadManagedRouterToken() = %q, want %q", got, test.want)
			}
		})
	}
}

func TestManagedRouterConfigMatches(t *testing.T) {
	token := routerToken()
	options := RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json", RouterToken: token}
	content := constants.ManagedMarker + "\nopenai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"\nmodel_catalog_json = \"/tmp/models.json\"\n"

	matched, err := ManagedRouterConfigMatches(content, options)
	if err != nil {
		t.Fatal(err)
	}
	if !matched {
		t.Error("ManagedRouterConfigMatches() = false, want true")
	}

	changes := []struct {
		name    string
		options RouterOptions
	}{
		{"different port", RouterOptions{Port: 10111, CatalogPath: "/tmp/models.json", RouterToken: token}},
		{"different catalog", RouterOptions{Port: 10110, CatalogPath: "/tmp/other.json", RouterToken: token}},
		{"different token", RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json", RouterToken: strings.Repeat("B", 43)}},
	}
	for _, test := range changes {
		t.Run(test.name, func(t *testing.T) {
			matched, err := ManagedRouterConfigMatches(content, test.options)
			if err != nil {
				t.Fatal(err)
			}
			if matched {
				t.Error("ManagedRouterConfigMatches() = true, want false")
			}
		})
	}

	if matched, err := ManagedRouterConfigMatches("model = \"gpt-5.6-sol\"\n", options); err != nil || matched {
		t.Errorf("ManagedRouterConfigMatches(no block) = %v, %v; want false, nil", matched, err)
	}

	if _, err := ManagedRouterConfigMatches(content, RouterOptions{Port: 0, CatalogPath: "/tmp/models.json", RouterToken: token}); err == nil {
		t.Error("ManagedRouterConfigMatches(invalid port) = nil error, want an error")
	}
	if _, err := ManagedRouterConfigMatches(content, RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json"}); err == nil {
		t.Error("ManagedRouterConfigMatches(missing token) = nil error, want an error")
	}
}

func TestRewriteManagedRouterConfig(t *testing.T) {
	token := routerToken()
	original := strings.Join([]string{
		"# user comment",
		"personality = \"pragmatic\"",
		constants.ManagedMarker,
		"openai_base_url = \"http://127.0.0.1:10110/" + strings.Repeat("B", 43) + "/v1\"",
		"model_catalog_json = \"/tmp/old.json\"",
		"",
		"[features]",
		"multi_agent = true",
		"",
	}, "\n")

	rewritten, err := RewriteManagedRouterConfig(original, RouterOptions{Port: 20220, CatalogPath: "/tmp/new.json", RouterToken: token})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"# user comment",
		"personality = \"pragmatic\"",
		constants.ManagedMarker,
		"openai_base_url = \"http://127.0.0.1:20220/" + token + "/v1\"",
		"model_catalog_json = \"/tmp/new.json\"",
		"",
		"[features]",
		"multi_agent = true",
		"",
	}, "\n")
	if rewritten != want {
		t.Errorf("RewriteManagedRouterConfig() =\n%s\nwant\n%s", rewritten, want)
	}

	if _, err := RewriteManagedRouterConfig("model = \"gpt-5.6-sol\"\n", RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json", RouterToken: token}); err == nil {
		t.Error("RewriteManagedRouterConfig(no block) = nil error, want an error")
	}
}

func TestRewriteManagedRouterConfigNormalizesCRLF(t *testing.T) {
	token := routerToken()
	original := constants.ManagedMarker + "\r\nopenai_base_url = \"http://127.0.0.1:10110/" + strings.Repeat("B", 43) + "/v1\"\r\nmodel_catalog_json = \"/tmp/old.json\"\r\n"
	rewritten, err := RewriteManagedRouterConfig(original, RouterOptions{Port: 10110, CatalogPath: "/tmp/models.json", RouterToken: token})
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(rewritten, "\r") {
		t.Errorf("RewriteManagedRouterConfig() kept CR bytes: %q", rewritten)
	}
	if got := ReadManagedRouterToken(rewritten); got != token {
		t.Errorf("ReadManagedRouterToken(rewritten) = %q, want %q", got, token)
	}
}

func TestManagedBlockGoldenMessyConfig(t *testing.T) {
	token := routerToken()
	original := strings.Join([]string{
		"# Codex configuration",
		"model = \"gpt-5.6-sol\"",
		"",
		"# DSCodex managed; remove with `dscodex uninstall`",
		"  openai_base_url   =   \"http://127.0.0.1:10110/" + strings.Repeat("B", 43) + "/v1\"",
		"model_catalog_json = \"C:\\\\Users\\\\x\\\\.codex\\\\dscodex-models.json\"",
		"",
		"[features]",
		"multi_agent = true",
		"",
		"[mcp_servers.node_repl.env]",
		"CODEX_HOME = \"C:\\\\Users\\\\x\\\\.codex\"",
		"",
	}, "\n")

	rewritten, err := RewriteManagedRouterConfig(original, RouterOptions{
		Port:        10110,
		CatalogPath: "C:\\Users\\x\\.codex\\dscodex-models.json",
		RouterToken: token,
	})
	if err != nil {
		t.Fatal(err)
	}
	want := strings.Join([]string{
		"# Codex configuration",
		"model = \"gpt-5.6-sol\"",
		"",
		"# DSCodex managed; remove with `dscodex uninstall`",
		"openai_base_url = \"http://127.0.0.1:10110/" + token + "/v1\"",
		"model_catalog_json = \"C:\\\\Users\\\\x\\\\.codex\\\\dscodex-models.json\"",
		"",
		"[features]",
		"multi_agent = true",
		"",
		"[mcp_servers.node_repl.env]",
		"CODEX_HOME = \"C:\\\\Users\\\\x\\\\.codex\"",
		"",
	}, "\n")
	if rewritten != want {
		t.Errorf("RewriteManagedRouterConfig() =\n%s\nwant\n%s", rewritten, want)
	}
	if got := ReadManagedRouterToken(rewritten); got != token {
		t.Errorf("ReadManagedRouterToken(rewritten) = %q, want %q", got, token)
	}
}

func TestQuoteTomlMatchesJSONString(t *testing.T) {
	tests := []struct {
		value string
		want  string
	}{
		{"http://127.0.0.1:10110/" + routerToken() + "/v1", `"http://127.0.0.1:10110/` + routerToken() + `/v1"`},
		{`C:\Users\x\.codex\dscodex-models.json`, `"C:\\Users\\x\\.codex\\dscodex-models.json"`},
		{"/tmp/models.json", `"/tmp/models.json"`},
		{"a&b<c>d", `"a&b<c>d"`},
	}
	for _, test := range tests {
		if got := quoteToml(test.value); got != test.want {
			t.Errorf("quoteToml(%q) = %s, want %s", test.value, got, test.want)
		}
	}
}
