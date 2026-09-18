package codex

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

func TestListModelsSkipsHidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/codex/models" || r.URL.Query().Get("client_version") == "" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("request = %s %v", r.URL, r.Header)
		}
		_, _ = w.Write([]byte(`{"models":[
			{"slug":"gpt-5-codex","visibility":"list"},
			{"slug":"gpt-5","visibility":"list"},
			{"slug":"codex-internal","visibility":"hide"}
		]}`))
	}))
	defer srv.Close()
	old := modelsBaseURL
	modelsBaseURL = srv.URL
	defer func() { modelsBaseURL = old }()

	ids, err := New(Config{Token: "tok"}).ListModels(context.Background())
	if err != nil || !reflect.DeepEqual(ids, []string{"gpt-5-codex", "gpt-5"}) {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}
