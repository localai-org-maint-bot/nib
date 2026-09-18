package llmprovider

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/mudler/nib/auth"
	"github.com/mudler/nib/codexapp"
	"github.com/mudler/nib/provider"
	"github.com/mudler/nib/types"
)

func TestNewSelectsProviders(t *testing.T) {
	openaiLLM, err := New(types.ModelProviderConfig{Provider: "openai-compatible", Model: "local", BaseURL: "http://localhost:8080/v1"})
	if err != nil || openaiLLM == nil {
		t.Fatalf("openai-compatible provider: llm=%T err=%v", openaiLLM, err)
	}
	codexLLM, err := New(types.ModelProviderConfig{Provider: "codex", Model: "gpt-test"})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := codexLLM.(*codexapp.LLM); !ok {
		t.Fatalf("codex provider returned %T", codexLLM)
	}
}

func TestNewRejectsUnknownProvider(t *testing.T) {
	if _, err := New(types.ModelProviderConfig{Provider: "mystery"}); err == nil {
		t.Fatal("unknown provider was accepted")
	}
}

func TestModelsEndpointKeepsStoredKeysOffCustomServers(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "env-openai-key")
	store := auth.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
	def, _ := provider.Get("openai")
	if _, err := auth.LoginAPIKey(store, def, "stored-openai-key"); err != nil {
		t.Fatal(err)
	}

	// A local server behind the implicit "openai" provider must never see the
	// OpenAI key stored by /login or exported in the environment.
	base, key, err := ModelsEndpoint(types.ModelProviderConfig{Provider: "", BaseURL: "http://localhost:8080/v1"}, store)
	if err != nil || base != "http://localhost:8080/v1" || key != "" {
		t.Fatalf("custom endpoint: base=%q key=%q err=%v, want localhost with no key", base, key, err)
	}
	_, key, _ = ModelsEndpoint(types.ModelProviderConfig{BaseURL: "http://localhost:8080/v1", APIKey: "local-key"}, store)
	if key != "local-key" {
		t.Fatalf("custom endpoint key = %q, want the configured one", key)
	}

	// The provider's own endpoint gets the stored credential.
	base, key, _ = ModelsEndpoint(types.ModelProviderConfig{Provider: "openai"}, store)
	if base != openAIDefaultBaseURL || key != "stored-openai-key" {
		t.Fatalf("openai endpoint: base=%q key=%q", base, key)
	}
}

func TestModelsEndpointUsesProviderDefaults(t *testing.T) {
	store := auth.NewStore(filepath.Join(t.TempDir(), "credentials.json"))
	def, _ := provider.Get("regolo")
	if _, err := auth.LoginAPIKey(store, def, "regolo-key"); err != nil {
		t.Fatal(err)
	}
	base, key, err := ModelsEndpoint(types.ModelProviderConfig{Provider: "regolo"}, store)
	if err != nil || base != def.BaseURL || key != "regolo-key" {
		t.Fatalf("regolo: base=%q key=%q err=%v", base, key, err)
	}
	if _, _, err := ModelsEndpoint(types.ModelProviderConfig{Provider: "anthropic"}, store); !errors.Is(err, ErrNoModelList) {
		t.Fatalf("anthropic: err=%v, want ErrNoModelList", err)
	}
}

func TestListModelsDispatchesByProtocol(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GOOGLE_API_KEY", "")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/v1/models": // Anthropic Messages API and OpenAI-compatible alike
			if r.Header.Get("x-api-key") == "sk-ant" {
				_, _ = w.Write([]byte(`{"data":[{"id":"claude-x"}],"has_more":false}`))
				return
			}
			_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"oai-x","object":"model"}]}`))
		case "/v1beta/models":
			_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-x","supportedGenerationMethods":["generateContent"]}]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	ctx := context.Background()

	cases := []struct {
		cfg  types.ModelProviderConfig
		want string
	}{
		{types.ModelProviderConfig{Provider: "anthropic", BaseURL: srv.URL, APIKey: "sk-ant"}, "claude-x"},
		{types.ModelProviderConfig{Provider: "google", BaseURL: srv.URL + "/v1beta", APIKey: "AIza"}, "gemini-x"},
		{types.ModelProviderConfig{Provider: "", BaseURL: srv.URL + "/v1"}, "oai-x"},
	}
	for _, c := range cases {
		got, err := ListModels(ctx, c.cfg, nil)
		if err != nil || len(got) != 1 || got[0] != c.want {
			t.Errorf("%s: got %v, %v; want [%s]", c.cfg.Provider, got, err, c.want)
		}
	}

	// A native adapter without credentials reports that, not "no list".
	if _, err := ListModels(ctx, types.ModelProviderConfig{Provider: "anthropic", BaseURL: srv.URL}, nil); err == nil || errors.Is(err, ErrNoModelList) {
		t.Fatalf("anthropic without a key: err = %v, want a credentials error", err)
	}
}

func TestListModelsWithoutListerIsErrNoModelList(t *testing.T) {
	t.Setenv("AZURE_OPENAI_API_KEY", "az-key")
	cfg := types.ModelProviderConfig{Provider: "azure", BaseURL: "https://example.openai.azure.com/openai/v1", Model: "gpt"}
	if _, err := ListModels(context.Background(), cfg, nil); !errors.Is(err, ErrNoModelList) {
		t.Fatalf("azure: err = %v, want ErrNoModelList", err)
	}
}

func TestListModelChoicesFlagsPartialLists(t *testing.T) {
	t.Setenv("AZURE_OPENAI_API_KEY", "az-key")
	t.Setenv("AZURE_OPENAI_DEPLOYMENT_NAME_MAP", "gpt-4.1=prod")
	cfg := types.ModelProviderConfig{Provider: "azure", BaseURL: "https://example.openai.azure.com/openai/v1", Model: "gpt-4.1"}
	ids, partial, err := ListModelChoices(context.Background(), cfg, nil)
	if err != nil || !partial || len(ids) != 1 || ids[0] != "gpt-4.1" {
		t.Fatalf("azure map: ids=%v partial=%v err=%v", ids, partial, err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"object":"list","data":[{"id":"m","object":"model"}]}`))
	}))
	defer srv.Close()
	if _, partial, _ := ListModelChoices(context.Background(), types.ModelProviderConfig{BaseURL: srv.URL}, nil); partial {
		t.Fatal("an endpoint's own /models list is complete, not partial")
	}
}
