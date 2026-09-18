package copilot

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// ListModels returns the chat models this Copilot account can pick
// (GET {endpoint}/models). Copilot also lists embedding models and internal
// ones the editor's picker hides; only the chat models its picker offers are
// kept, unless the API marks none, in which case every chat model is kept.
func (l *LLM) ListModels(ctx context.Context) ([]string, error) {
	endpoint, err := l.ensureEndpoint(ctx)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(endpoint, "/")+"/models", nil)
	if err != nil {
		return nil, fmt.Errorf("copilot: create request: %w", err)
	}
	l.setHeaders(req, false)
	req.Header.Del("Content-Type")

	resp, err := l.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("copilot: list models: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("copilot: read models: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, parseAPIError(resp.StatusCode, body)
	}

	var page struct {
		Data []struct {
			ID           string `json:"id"`
			PickerEnable *bool  `json:"model_picker_enabled"`
			Capabilities struct {
				Type string `json:"type"`
			} `json:"capabilities"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &page); err != nil {
		return nil, fmt.Errorf("copilot: decode models: %w", err)
	}
	var chat, picker []string
	seen := map[string]bool{}
	for _, m := range page.Data {
		if m.ID == "" || seen[m.ID] || (m.Capabilities.Type != "" && m.Capabilities.Type != "chat") {
			continue
		}
		seen[m.ID] = true
		chat = append(chat, m.ID)
		if m.PickerEnable != nil && *m.PickerEnable {
			picker = append(picker, m.ID)
		}
	}
	if len(picker) > 0 {
		return picker, nil
	}
	return chat, nil
}
