package google

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
)

// maxModelPages bounds pagination against a server that keeps returning a
// nextPageToken; the real list fits in one or two pages.
const maxModelPages = 20

// ListModels returns the Gemini models that can generate content
// (GET {base}/models), following nextPageToken pagination. Embedding-only and
// other non-chat models are left out, and the "models/" prefix is dropped so
// the IDs are what the chat request expects.
func (l *LLM) ListModels(ctx context.Context) ([]string, error) {
	var ids []string
	token := ""
	for range maxModelPages {
		q := url.Values{"pageSize": {"1000"}}
		if token != "" {
			q.Set("pageToken", token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, l.config.BaseURL+"/models?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("google: create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if l.config.IsOAuth {
			req.Header.Set("Authorization", "Bearer "+l.config.Token)
		} else {
			req.Header.Set("x-goog-api-key", l.config.APIKey)
		}

		resp, err := l.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("google: list models: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("google: read models: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, parseAPIError(resp.StatusCode, body)
		}

		var page struct {
			Models []struct {
				Name    string   `json:"name"`
				Methods []string `json:"supportedGenerationMethods"`
			} `json:"models"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("google: decode models: %w", err)
		}
		for _, m := range page.Models {
			// A model that states its methods and lacks generateContent
			// cannot chat; one that states none is kept rather than guessed.
			if len(m.Methods) > 0 && !slices.Contains(m.Methods, "generateContent") {
				continue
			}
			if id := strings.TrimPrefix(m.Name, "models/"); id != "" {
				ids = append(ids, id)
			}
		}
		if page.NextPageToken == "" {
			break
		}
		token = page.NextPageToken
	}
	return ids, nil
}

// KnownChatModels are Gemini chat models for backends that cannot enumerate
// them: Cloud Code Assist (Gemini CLI) has no listing endpoint, and Vertex's
// publisher-model listing refuses API keys. It is a suggestion for the model
// picker, where a name missing from it can still be typed; the server decides
// whether the account may use it. Returns a fresh copy.
func KnownChatModels() []string {
	return []string{
		"gemini-2.5-pro",
		"gemini-2.5-flash",
		"gemini-2.5-flash-lite",
		"gemini-3-pro-preview",
		"gemini-3-flash-preview",
	}
}
