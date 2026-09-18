package google

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestListModelsFiltersAndPaginates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1beta/models" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.Header.Get("x-goog-api-key") != "AIza" {
			t.Errorf("headers = %v", r.Header)
		}
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("pageToken") == "" {
			_, _ = w.Write([]byte(`{"models":[
				{"name":"models/gemini-2.5-pro","supportedGenerationMethods":["generateContent","countTokens"]},
				{"name":"models/text-embedding-004","supportedGenerationMethods":["embedContent"]}
			],"nextPageToken":"p2"}`))
			return
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"models/gemini-2.5-flash","supportedGenerationMethods":["generateContent"]}]}`))
	}))
	defer srv.Close()

	ids, err := New(Config{BaseURL: srv.URL + "/v1beta", APIKey: "AIza"}).ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"gemini-2.5-pro", "gemini-2.5-flash"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v (chat models only, no models/ prefix)", ids, want)
	}
}

func TestListModelsOAuthAndErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.Header.Get("x-goog-api-key") != "" {
			t.Errorf("oauth headers = %v", r.Header)
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":{"code":403,"status":"PERMISSION_DENIED","message":"API key not valid"}}`))
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL, Token: "tok", IsOAuth: true}).ListModels(context.Background())
	if err == nil || !strings.Contains(err.Error(), "API key not valid") {
		t.Fatalf("err = %v, want the API's message", err)
	}
}
