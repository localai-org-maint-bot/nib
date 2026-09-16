package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// defaultContextTokens is the last-resort fallback when neither the endpoint
// probe nor the static table can identify the model's context window.
const defaultContextTokens = 128000

// probeContextSize asks the endpoint's /models/capabilities for the given
// model's context_size. Returns 0 on any failure (network, decode, model not
// found, or context_size absent), so the caller can fall through to the next
// strategy.
func probeContextSize(ctx context.Context, baseURL, apiKey, model string) int {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+"/models/capabilities", nil)
	if err != nil {
		return 0
	}
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 0
	}
	var body struct {
		Data []struct {
			ID          string `json:"id"`
			ContextSize int    `json:"context_size"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return 0
	}
	for _, m := range body.Data {
		if m.ID == model && m.ContextSize > 0 {
			return m.ContextSize
		}
	}
	return 0
}

// staticContextSize returns a known context window for commercial models that
// do not expose /models/capabilities. Matching is case-insensitive on the
// model name prefix. Returns 0 when the model is not recognised.
func staticContextSize(model string) int {
	m := strings.ToLower(strings.TrimSpace(model))
	for _, e := range staticContextTable {
		if strings.HasPrefix(m, e.prefix) {
			return e.tokens
		}
	}
	return 0
}

type contextEntry struct {
	prefix string
	tokens int
}

// staticContextTable is ordered longest-prefix-first so that e.g. "gpt-4o"
// matches before "gpt-4".
var staticContextTable = []contextEntry{
	// OpenAI
	{"gpt-4o", 128000},
	{"gpt-4-turbo", 128000},
	{"gpt-4.1", 1047576},
	{"gpt-4", 8192},
	{"gpt-3.5", 16385},
	{"o1", 200000},
	{"o3", 200000},
	{"o4", 200000},

	// Anthropic
	{"claude-3", 200000},
	{"claude-2", 100000},

	// Google
	{"gemini-2", 1048576},
	{"gemini-1.5", 1048576},
}

// detectContextSize resolves the model's context window by trying the endpoint
// probe first, then the static table. Returns 0 when neither source identifies
// the model, leaving the caller to apply its own default.
func detectContextSize(ctx context.Context, baseURL, apiKey, model string) int {
	if v := probeContextSize(ctx, baseURL, apiKey, model); v > 0 {
		return v
	}
	return staticContextSize(model)
}

// probeTimeout bounds the context-size probe. Like ModelListTimeout this runs
// on a path where blocking degrades the UX; a failed probe is silent (the
// existing value is kept), so a short budget is strictly better than waiting.
const probeTimeout = 5 * time.Second
