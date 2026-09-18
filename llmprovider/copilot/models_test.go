package copilot

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListModelsKeepsPickerChatModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/models" || r.Header.Get("Authorization") != "Bearer ghu" || r.Header.Get("Editor-Version") == "" {
			t.Errorf("request = %s %v", r.URL.Path, r.Header)
		}
		_, _ = w.Write([]byte(`{"data":[
			{"id":"gpt-4.1","model_picker_enabled":true,"capabilities":{"type":"chat"}},
			{"id":"claude-sonnet-4","model_picker_enabled":true,"capabilities":{"type":"chat"}},
			{"id":"gpt-4o-mini-internal","model_picker_enabled":false,"capabilities":{"type":"chat"}},
			{"id":"text-embedding-3-small","capabilities":{"type":"embeddings"}}
		]}`))
	}))
	defer srv.Close()

	l := New(Config{Token: "ghu"})
	l.endpoint, l.resolved = srv.URL, true
	ids, err := l.ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gpt-4.1", "claude-sonnet-4"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestListModelsFallsBackToAllChatModels(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"data":[{"id":"a","capabilities":{"type":"chat"}},{"id":"e","capabilities":{"type":"embeddings"}},{"id":"b"}]}`))
	}))
	defer srv.Close()
	l := New(Config{Token: "t"})
	l.endpoint, l.resolved = srv.URL, true
	ids, err := l.ListModels(context.Background())
	if err != nil || !reflect.DeepEqual(ids, []string{"a", "b"}) {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}
