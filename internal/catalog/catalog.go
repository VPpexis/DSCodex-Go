// Package catalog merges the DSCodex DeepSeek entries into the Codex model
// catalog that Codex loads from model_catalog_json.
//
// It is the Go port of upstream src/catalog.mjs (v1.1.0). The DeepSeek entry is
// cloned from the gpt-5.6-sol template (falling back to the first native
// entry), with every field DeepSeek's Responses API needs pinned explicitly.
package catalog

import (
	"errors"
	"strings"

	"github.com/VPpexis/dscodex-go/internal/constants"
)

// Reasoning levels advertised for every DeepSeek model.
var (
	highReasoningLevel = map[string]any{
		"effort":      "high",
		"description": "DeepSeek thinking mode",
	}
	maxReasoningLevel = map[string]any{
		"effort":      "max",
		"description": "Maximum DeepSeek thinking depth",
	}
)

// clone deep-copies a JSON-derived value so pinned fields never mutate the
// caller's cache.
func clone(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		result := make(map[string]any, len(typed))
		for key, item := range typed {
			result[key] = clone(item)
		}
		return result
	case []any:
		result := make([]any, len(typed))
		for index, item := range typed {
			result[index] = clone(item)
		}
		return result
	default:
		return value
	}
}

func cloneEntry(entry map[string]any) map[string]any {
	if cloned, ok := clone(entry).(map[string]any); ok {
		return cloned
	}
	return map[string]any{}
}

// replaceIdentity rewrites the GPT-5 identity strings Codex ships in its
// instruction templates to name the DeepSeek product instead.
func replaceIdentity(value any, productName string) any {
	text, ok := value.(string)
	if !ok {
		return value
	}
	replacement := "You are Codex, powered by " + productName + "."
	text = strings.ReplaceAll(text, "You are Codex, an agent based on GPT-5.", replacement)
	text = strings.ReplaceAll(text, "You are Codex, based on GPT-5.", replacement)
	return text
}

// backfillNativeEntry supplies the one required field older Codex caches may
// omit. A single unparseable entry breaks the whole catalog, so native entries
// are repaired rather than rejected.
func backfillNativeEntry(model map[string]any) map[string]any {
	entry := cloneEntry(model)
	if _, present := entry["supports_reasoning_summaries"]; !present {
		entry["supports_reasoning_summaries"] = false
	}
	return entry
}

// buildDeepSeekCatalogEntry clones the native template and pins every field the
// DeepSeek Responses API requires.
func buildDeepSeekCatalogEntry(template map[string]any, model constants.Model) map[string]any {
	entry := cloneEntry(template)
	entry["slug"] = model.PickerSlug
	entry["display_name"] = model.DisplayName
	entry["description"] = model.ProductName + " via the native Responses API."
	entry["default_reasoning_level"] = "max"
	entry["supported_reasoning_levels"] = []any{clone(highReasoningLevel), clone(maxReasoningLevel)}
	entry["priority"] = 0
	entry["visibility"] = "list"
	entry["supported_in_api"] = true
	entry["prefer_websockets"] = false
	entry["support_verbosity"] = true
	entry["default_verbosity"] = "low"
	entry["apply_patch_tool_type"] = "freeform"
	entry["web_search_tool_type"] = "text"
	// Declaring the image modality opens the desktop view_image gate; the router
	// rewrites those images into GPT-generated descriptions before DeepSeek sees them.
	entry["input_modalities"] = []any{"text", "image"}
	entry["supports_image_detail_original"] = false
	// DeepSeek's Responses API rejects a turn that replays more than one tool call
	// ("The reasoning_text in the thinking mode must be passed back to the API."),
	// which wedges every later request in the session. Never ask for parallel calls.
	entry["supports_parallel_tool_calls"] = false
	entry["supports_search_tool"] = true
	entry["tool_mode"] = nil
	entry["multi_agent_version"] = "v2"
	entry["use_responses_lite"] = false
	entry["include_skills_usage_instructions"] = false
	entry["context_window"] = 1_048_576
	entry["max_context_window"] = 1_048_576
	entry["effective_context_window_percent"] = 95
	entry["auto_compact_token_limit"] = nil
	entry["default_reasoning_summary"] = "none"
	entry["supports_reasoning_summaries"] = false
	entry["minimal_client_version"] = "0.144.0"
	entry["availability_nux"] = nil
	entry["upgrade"] = nil
	entry["experimental_supported_tools"] = []any{}
	if _, present := entry["base_instructions"]; present {
		entry["base_instructions"] = replaceIdentity(entry["base_instructions"], model.ProductName)
	}
	if messages, ok := entry["model_messages"].(map[string]any); ok {
		if template, ok := messages["instructions_template"].(string); ok && template != "" {
			messages["instructions_template"] = replaceIdentity(template, model.ProductName)
		}
	}

	delete(entry, "additional_speed_tiers")
	delete(entry, "service_tiers")
	delete(entry, "default_service_tier")
	return entry
}

// Build merges the DeepSeek entries into the Codex model cache.
func Build(cache map[string]any) (map[string]any, error) {
	models, ok := cache["models"].([]any)
	if !ok || len(models) == 0 {
		return nil, errors.New("Codex models_cache.json has no model templates; open Codex once, then retry")
	}
	deepSeekSlugs := make(map[string]bool, len(constants.DeepSeekModels))
	for _, model := range constants.DeepSeekModels {
		deepSeekSlugs[model.PickerSlug] = true
	}
	nativeModels := make([]map[string]any, 0, len(models))
	for _, raw := range models {
		model, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		slug, _ := model["slug"].(string)
		if deepSeekSlugs[slug] {
			continue
		}
		nativeModels = append(nativeModels, model)
	}
	if len(nativeModels) == 0 {
		return nil, errors.New("Codex models_cache.json has no native model template; open Codex once, then retry")
	}
	template := nativeModels[0]
	for _, model := range nativeModels {
		if slug, _ := model["slug"].(string); slug == "gpt-5.6-sol" {
			template = model
			break
		}
	}
	merged := make([]any, 0, len(constants.DeepSeekModels)+len(nativeModels))
	for _, model := range constants.DeepSeekModels {
		merged = append(merged, buildDeepSeekCatalogEntry(template, model))
	}
	for _, model := range nativeModels {
		merged = append(merged, backfillNativeEntry(model))
	}
	return map[string]any{"models": merged}, nil
}
