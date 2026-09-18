package codex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

// codexModelsEndpoint lists the models the ChatGPT subscription can use with
// Codex, the same list the Codex CLI's /model picker shows.
const codexModelsEndpoint = "/codex/models"

// modelsBaseURL is codexBaseURL, a variable so tests can point it at a fake.
var modelsBaseURL = codexBaseURL

// ListModels returns the Codex models visible to this ChatGPT account. Entries
// the backend marks hidden are left out, as the Codex CLI does.
func (l *LLM) ListModels(ctx context.Context) ([]string, error) {
	u := modelsBaseURL + codexModelsEndpoint + "?" + url.Values{"client_version": {codexClientVer}}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("codex: create request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+l.config.Token)
	req.Header.Set("originator", "nib")
	req.Header.Set("version", codexClientVer)
	if accountID := extractAccountID(l.config.Token); accountID != "" {
		req.Header.Set("chatgpt-account-id", accountID)
	}

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("codex: list models: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("codex: read models: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var page struct {
		Models []struct {
			Slug       string `json:"slug"`
			ID         string `json:"id"`
			Visibility string `json:"visibility"`
		} `json:"models"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("codex: decode models: %w", err)
	}
	var ids []string
	for _, m := range page.Models {
		id := firstNonEmpty(m.Slug, m.ID)
		if id == "" || m.Visibility == "hide" || m.Visibility == "none" {
			continue
		}
		ids = append(ids, id)
	}
	return ids, nil
}
