package llmprovider

import (
	"errors"
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
