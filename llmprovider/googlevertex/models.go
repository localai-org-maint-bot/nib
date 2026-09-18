package googlevertex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/mudler/nib/llmprovider/google"
)

// maxModelPages bounds pagination against a server that keeps returning a
// nextPageToken.
const maxModelPages = 20

// ListModels returns the Gemini models Google publishes on Vertex AI
// (GET {base}/v1beta1/publishers/google/models; the list call only exists in
// v1beta1). The catalogue also carries Imagen, Veo, embedding and other
// non-chat models this adapter cannot drive, so only Gemini chat models are
// kept. OAuth callers bill the listing to their project via
// x-goog-user-project, as the chat call does through its URL.
//
// The listing refuses API keys ("API keys are not supported by this API"), so
// an API-key (express mode) session is offered google.KnownChatModels instead.
func (l *LLM) ListModels(ctx context.Context) ([]string, error) {
	if !l.config.IsOAuth {
		return google.KnownChatModels(), nil
	}
	base := strings.TrimRight(l.config.BaseURL, "/")
	var ids []string
	token := ""
	for range maxModelPages {
		q := url.Values{"pageSize": {"1000"}}
		if token != "" {
			q.Set("pageToken", token)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, base+"/v1beta1/publishers/google/models?"+q.Encode(), nil)
		if err != nil {
			return nil, fmt.Errorf("google-vertex: create request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		if l.config.IsOAuth {
			req.Header.Set("Authorization", "Bearer "+l.config.Token)
			if l.config.Project != "" {
				req.Header.Set("x-goog-user-project", l.config.Project)
			}
		} else {
			req.Header.Set("x-goog-api-key", l.config.APIKey)
		}

		resp, err := l.client.Do(req)
		if err != nil {
			return nil, fmt.Errorf("google-vertex: list models: %w", err)
		}
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("google-vertex: read models: %w", err)
		}
		if resp.StatusCode != http.StatusOK {
			return nil, parseAPIError(resp.StatusCode, body)
		}

		var page struct {
			PublisherModels []struct {
				Name string `json:"name"`
			} `json:"publisherModels"`
			NextPageToken string `json:"nextPageToken"`
		}
		if err := json.Unmarshal(body, &page); err != nil {
			return nil, fmt.Errorf("google-vertex: decode models: %w", err)
		}
		for _, m := range page.PublisherModels {
			id := m.Name[strings.LastIndex(m.Name, "/")+1:]
			if strings.HasPrefix(id, "gemini") && !strings.Contains(id, "embedding") {
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

// ModelListIsPartial: in API-key mode the list is google.KnownChatModels, a
// suggestion; with OAuth it is Vertex's own catalogue.
func (l *LLM) ModelListIsPartial() bool { return !l.config.IsOAuth }
