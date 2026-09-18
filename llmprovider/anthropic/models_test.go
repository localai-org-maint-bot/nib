package anthropic

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListModelsPaginatesWithAPIKey(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.URL.Path != "/v1/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-api-key") != "sk-ant" || r.Header.Get("anthropic-version") == "" || r.Header.Get("Authorization") != "" {
			t.Errorf("headers = %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("after_id") == "" {
			_, _ = w.Write([]byte(`{"data":[{"id":"claude-a"},{"id":"claude-b"}],"has_more":true,"last_id":"claude-b"}`))
			return
		}
		if got := r.URL.Query().Get("after_id"); got != "claude-b" {
			t.Errorf("after_id = %q", got)
		}
		_, _ = w.Write([]byte(`{"data":[{"id":"claude-c"}],"has_more":false,"last_id":"claude-c"}`))
	}))
	defer srv.Close()

	ids, err := New(Config{BaseURL: srv.URL, APIKey: "sk-ant"}).ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"claude-a", "claude-b", "claude-c"}; !reflect.DeepEqual(ids, want) || calls != 2 {
		t.Fatalf("ids = %v after %d calls, want %v in 2", ids, calls, want)
	}
}

func TestListModelsOAuthAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("anthropic-beta") == "" || r.Header.Get("x-api-key") != "" {
			t.Errorf("oauth headers = %v", r.Header)
		}
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"authentication_error","message":"invalid x-api-key"}}`))
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, Token: "tok", IsOAuth: true}).ListModels(context.Background())
	if err == nil || !strings.Contains(err.Error(), "invalid x-api-key") {
		t.Fatalf("err = %v, want the API's message", err)
	}
}
