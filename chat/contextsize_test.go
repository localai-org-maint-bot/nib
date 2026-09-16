package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestStaticContextSize(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"gpt-4o", 128000},
		{"gpt-4o-2024-08-06", 128000},
		{"gpt-4-turbo", 128000},
		{"gpt-4", 8192},
		{"gpt-4-0613", 8192},
		{"gpt-3.5-turbo", 16385},
		{"o1-preview", 200000},
		{"o3-mini", 200000},
		{"claude-3-5-sonnet-20241022", 200000},
		{"claude-3-opus-20240229", 200000},
		{"claude-2.1", 100000},
		{"gemini-2.0-flash", 1048576},
		{"gemini-1.5-pro", 1048576},
		{"unknown-model", 0},
		{"", 0},
	}
	for _, tc := range tests {
		got := staticContextSize(tc.model)
		if got != tc.want {
			t.Errorf("staticContextSize(%q) = %d, want %d", tc.model, got, tc.want)
		}
	}
}

func TestStaticContextSizeCaseInsensitive(t *testing.T) {
	if got := staticContextSize("GPT-4O"); got != 128000 {
		t.Fatalf("staticContextSize(\"GPT-4O\") = %d, want 128000", got)
	}
	if got := staticContextSize("Claude-3-5-Sonnet"); got != 200000 {
		t.Fatalf("staticContextSize(\"Claude-3-5-Sonnet\") = %d, want 200000", got)
	}
}

func TestProbeContextSize(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models/capabilities" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"object": "list",
			"data": []map[string]any{
				{"id": "model-a", "context_size": 32768},
				{"id": "model-b", "context_size": 8192},
			},
		})
	}))
	defer srv.Close()

	got := probeContextSize(context.Background(), srv.URL, "", "model-a")
	if got != 32768 {
		t.Fatalf("probeContextSize(model-a) = %d, want 32768", got)
	}
	got = probeContextSize(context.Background(), srv.URL, "", "model-b")
	if got != 8192 {
		t.Fatalf("probeContextSize(model-b) = %d, want 8192", got)
	}
	// Model not in the list
	got = probeContextSize(context.Background(), srv.URL, "", "model-c")
	if got != 0 {
		t.Fatalf("probeContextSize(model-c) = %d, want 0", got)
	}
}

func TestProbeContextSizeMissingField(t *testing.T) {
	// A model entry without context_size should yield 0, not a panic.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "no-ctx"},
			},
		})
	}))
	defer srv.Close()

	got := probeContextSize(context.Background(), srv.URL, "", "no-ctx")
	if got != 0 {
		t.Fatalf("probeContextSize(no-ctx) = %d, want 0", got)
	}
}

func TestProbeContextSizeServerDown(t *testing.T) {
	// A non-responsive server returns 0 (best-effort).
	got := probeContextSize(context.Background(), "http://127.0.0.1:1", "", "any")
	if got != 0 {
		t.Fatalf("probeContextSize(unreachable) = %d, want 0", got)
	}
}

func TestProbeContextSizeNon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	got := probeContextSize(context.Background(), srv.URL, "", "any")
	if got != 0 {
		t.Fatalf("probeContextSize(404) = %d, want 0", got)
	}
}

func TestDetectContextSizeProbeWins(t *testing.T) {
	// When the probe returns a value, it should be used even if the static
	// table also recognises the model.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]any{
				{"id": "gpt-4o", "context_size": 99999},
			},
		})
	}))
	defer srv.Close()

	got := detectContextSize(context.Background(), srv.URL, "", "gpt-4o")
	if got != 99999 {
		t.Fatalf("detectContextSize = %d, want 99999 (probe should win over static table)", got)
	}
}

func TestDetectContextSizeFallsBackToStatic(t *testing.T) {
	// When the probe fails (server down), the static table is used.
	got := detectContextSize(context.Background(), "http://127.0.0.1:1", "", "gpt-4o")
	if got != 128000 {
		t.Fatalf("detectContextSize = %d, want 128000 (static fallback)", got)
	}
}

func TestDetectContextSizeUnknownModel(t *testing.T) {
	// Neither probe nor static table recognise the model.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"data": []map[string]any{}})
	}))
	defer srv.Close()

	got := detectContextSize(context.Background(), srv.URL, "", "totally-unknown")
	if got != 0 {
		t.Fatalf("detectContextSize = %d, want 0 (unrecognised)", got)
	}
}
