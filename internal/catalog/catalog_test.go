package catalog

import (
	"encoding/json"
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

func template() map[string]any {
	return map[string]any{
		"slug":                       "gpt-5.6-sol",
		"display_name":               "GPT-5.6-Sol",
		"description":                "native",
		"default_reasoning_level":    "low",
		"supported_reasoning_levels": []any{map[string]any{"effort": "low", "description": "fast"}},
		"shell_type":                 "shell_command",
		"visibility":                 "list",
		"supported_in_api":           true,
		"priority":                   1,
		"base_instructions":          "You are Codex, an agent based on GPT-5.",
		"model_messages": map[string]any{
			"instructions_template": "You are Codex, an agent based on GPT-5.",
		},
	}
}

func modelsOf(t *testing.T, built map[string]any) []map[string]any {
	t.Helper()
	raw, ok := built["models"].([]any)
	if !ok {
		t.Fatalf("built catalog models = %T, want a slice", built["models"])
	}
	models := make([]map[string]any, 0, len(raw))
	for _, item := range raw {
		model, ok := item.(map[string]any)
		if !ok {
			t.Fatalf("built catalog entry = %T, want an object", item)
		}
		models = append(models, model)
	}
	return models
}

func entryWithSlug(t *testing.T, built map[string]any, slug string) map[string]any {
	t.Helper()
	for _, model := range modelsOf(t, built) {
		if model["slug"] == slug {
			return model
		}
	}
	t.Fatalf("catalog has no entry with slug %q", slug)
	return nil
}

func TestBuildCatalogBackfillsRequiredFields(t *testing.T) {
	stale := template()
	delete(stale, "supports_reasoning_summaries")
	built, err := Build(map[string]any{"models": []any{stale}})
	if err != nil {
		t.Fatal(err)
	}
	native := entryWithSlug(t, built, "gpt-5.6-sol")
	if native["supports_reasoning_summaries"] != false {
		t.Errorf("native supports_reasoning_summaries = %v, want false", native["supports_reasoning_summaries"])
	}
	deepseek := entryWithSlug(t, built, "deepseek/deepseek-v4-flash")
	if deepseek["supports_reasoning_summaries"] != false {
		t.Errorf("deepseek supports_reasoning_summaries = %v, want false", deepseek["supports_reasoning_summaries"])
	}
}

func TestBuildCatalogAddsFlashAndProEntries(t *testing.T) {
	built, err := Build(map[string]any{"models": []any{template()}})
	if err != nil {
		t.Fatal(err)
	}
	expected := []struct {
		slug        string
		displayName string
		productName string
	}{
		{"deepseek/deepseek-v4-flash", "DeepSeek V4 Flash", "DeepSeek V4 Flash"},
		{"deepseek/deepseek-v4-pro", "DeepSeek V4 Pro", "DeepSeek V4 Pro"},
	}
	for _, want := range expected {
		model := entryWithSlug(t, built, want.slug)
		if model["display_name"] != want.displayName {
			t.Errorf("%s display_name = %v, want %q", want.slug, model["display_name"], want.displayName)
		}
		if model["description"] != want.productName+" via the native Responses API." {
			t.Errorf("%s description = %v", want.slug, model["description"])
		}
		if model["default_reasoning_level"] != "max" {
			t.Errorf("%s default_reasoning_level = %v, want max", want.slug, model["default_reasoning_level"])
		}
		levels, _ := model["supported_reasoning_levels"].([]any)
		if len(levels) != 2 {
			t.Fatalf("%s supported_reasoning_levels = %v, want two levels", want.slug, model["supported_reasoning_levels"])
		}
		efforts := make([]string, 0, len(levels))
		for _, level := range levels {
			efforts = append(efforts, level.(map[string]any)["effort"].(string))
		}
		if !reflect.DeepEqual(efforts, []string{"high", "max"}) {
			t.Errorf("%s efforts = %v, want [high max]", want.slug, efforts)
		}
		if model["base_instructions"] != "You are Codex, powered by "+want.productName+"." {
			t.Errorf("%s base_instructions = %v", want.slug, model["base_instructions"])
		}
		if !reflect.DeepEqual(model["input_modalities"], []any{"text", "image"}) {
			t.Errorf("%s input_modalities = %v", want.slug, model["input_modalities"])
		}
		if model["prefer_websockets"] != false {
			t.Errorf("%s prefer_websockets = %v, want false", want.slug, model["prefer_websockets"])
		}
	}
}

func TestBuildCatalogReplacesBothIdentityStrings(t *testing.T) {
	source := template()
	source["base_instructions"] = "You are Codex, based on GPT-5."
	built, err := Build(map[string]any{"models": []any{source}})
	if err != nil {
		t.Fatal(err)
	}
	model := entryWithSlug(t, built, "deepseek/deepseek-v4-flash")
	if model["base_instructions"] != "You are Codex, powered by DeepSeek V4 Flash." {
		t.Errorf("base_instructions = %v", model["base_instructions"])
	}
	messages, _ := model["model_messages"].(map[string]any)
	if messages["instructions_template"] != "You are Codex, powered by DeepSeek V4 Flash." {
		t.Errorf("instructions_template = %v", messages["instructions_template"])
	}
}

func TestBuildCatalogDeletesSpeedTierFields(t *testing.T) {
	source := template()
	source["additional_speed_tiers"] = []any{"fast"}
	source["service_tiers"] = []any{"default"}
	source["default_service_tier"] = "default"
	built, err := Build(map[string]any{"models": []any{source}})
	if err != nil {
		t.Fatal(err)
	}
	for _, slug := range []string{"deepseek/deepseek-v4-flash", "deepseek/deepseek-v4-pro"} {
		model := entryWithSlug(t, built, slug)
		for _, field := range []string{"additional_speed_tiers", "service_tiers", "default_service_tier"} {
			if _, present := model[field]; present {
				t.Errorf("%s kept %s", slug, field)
			}
		}
	}
	native := entryWithSlug(t, built, "gpt-5.6-sol")
	if native["default_service_tier"] != "default" {
		t.Errorf("native default_service_tier = %v, want default", native["default_service_tier"])
	}
}

func TestBuildCatalogFallsBackToFirstNativeTemplate(t *testing.T) {
	fallback := template()
	fallback["slug"] = "gpt-5.7-preview"
	fallback["custom_marker"] = "fallback"
	built, err := Build(map[string]any{"models": []any{fallback}})
	if err != nil {
		t.Fatal(err)
	}
	model := entryWithSlug(t, built, "deepseek/deepseek-v4-flash")
	if model["custom_marker"] != "fallback" {
		t.Errorf("deepseek entry did not clone the fallback template: %v", model["custom_marker"])
	}
}

func TestBuildCatalogIgnoresCachedDeepSeekEntries(t *testing.T) {
	cached := template()
	cached["slug"] = "deepseek/deepseek-v4-flash"
	built, err := Build(map[string]any{"models": []any{template(), cached}})
	if err != nil {
		t.Fatal(err)
	}
	slugs := make([]string, 0)
	for _, model := range modelsOf(t, built) {
		slugs = append(slugs, model["slug"].(string))
	}
	want := []string{"deepseek/deepseek-v4-flash", "deepseek/deepseek-v4-pro", "gpt-5.6-sol"}
	if !reflect.DeepEqual(slugs, want) {
		t.Errorf("slugs = %v, want %v", slugs, want)
	}
}

func TestBuildCatalogRejectsEmptyCache(t *testing.T) {
	for name, cache := range map[string]map[string]any{
		"missing models": {},
		"empty models":   {"models": []any{}},
		"wrong type":     {"models": "nope"},
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Build(cache)
			if err == nil || !strings.Contains(err.Error(), "Codex models_cache.json has no model templates; open Codex once, then retry") {
				t.Errorf("Build() error = %v, want the no-model-templates message", err)
			}
		})
	}
}

func TestBuildCatalogRejectsAllDeepSeekCache(t *testing.T) {
	cached := template()
	cached["slug"] = "deepseek/deepseek-v4-flash"
	_, err := Build(map[string]any{"models": []any{cached}})
	if err == nil || !strings.Contains(err.Error(), "no native model template") {
		t.Errorf("Build() error = %v, want the no-native-template message", err)
	}
}

func TestBuildCatalogMatchesUpstreamGolden(t *testing.T) {
	cacheData, err := os.ReadFile("../../testdata/catalog-cache.json")
	if err != nil {
		t.Fatal(err)
	}
	cache := map[string]any{}
	if err := json.Unmarshal(cacheData, &cache); err != nil {
		t.Fatal(err)
	}
	goldenData, err := os.ReadFile("../../testdata/catalog-golden.json")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{}
	if err := json.Unmarshal(goldenData, &want); err != nil {
		t.Fatal(err)
	}
	built, err := Build(cache)
	if err != nil {
		t.Fatal(err)
	}
	normalized, err := json.Marshal(built)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]any{}
	if err := json.Unmarshal(normalized, &got); err != nil {
		t.Fatal(err)
	}
	// Display names intentionally differ from upstream (plain vs whale); every
	// other field must match the upstream builder structurally.
	stripDisplayNames(t, got)
	stripDisplayNames(t, want)
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Build() does not match the upstream golden fixture")
	}
	if len(constants.DeepSeekModels) == 0 {
		t.Fatal("no DeepSeek models configured")
	}
}

func stripDisplayNames(t *testing.T, catalog map[string]any) {
	t.Helper()
	raw, _ := catalog["models"].([]any)
	for _, item := range raw {
		if model, ok := item.(map[string]any); ok {
			delete(model, "display_name")
		}
	}
}
