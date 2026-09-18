package bedrock

import (
	"context"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func fakeControlPlane(t *testing.T, check func(*http.Request)) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		check(r)
		switch r.URL.Path {
		case "/inference-profiles":
			_, _ = w.Write([]byte(`{"inferenceProfileSummaries":[
				{"inferenceProfileId":"us.anthropic.claude-sonnet-4-v1:0","status":"ACTIVE"}
			]}`))
		case "/foundation-models":
			if r.URL.Query().Get("byOutputModality") != "TEXT" {
				t.Errorf("query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(`{"modelSummaries":[
				{"modelId":"amazon.nova-pro-v1:0","responseStreamingSupported":true,"inferenceTypesSupported":["ON_DEMAND"],"modelLifecycle":{"status":"ACTIVE"}},
				{"modelId":"old.model-v1","responseStreamingSupported":true,"inferenceTypesSupported":["ON_DEMAND"],"modelLifecycle":{"status":"LEGACY"}},
				{"modelId":"no.stream-v1","responseStreamingSupported":false,"inferenceTypesSupported":["ON_DEMAND"]}
			]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	old := controlPlaneURL
	controlPlaneURL = func(string) string { return srv.URL }
	t.Cleanup(func() { controlPlaneURL = old })
	return srv
}

func TestListModelsProfilesThenFoundationModels(t *testing.T) {
	fakeControlPlane(t, func(r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer brk" {
			t.Errorf("auth = %q", r.Header.Get("Authorization"))
		}
	})
	ids, err := New(Config{Region: "us-east-1", BearerToken: "brk"}).ListModels(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"us.anthropic.claude-sonnet-4-v1:0", "amazon.nova-pro-v1:0"}; !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids = %v, want %v", ids, want)
	}
}

func TestListModelsSigV4(t *testing.T) {
	t.Setenv("AWS_ACCESS_KEY_ID", "AKIDEXAMPLE")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "secret")
	t.Setenv("AWS_SESSION_TOKEN", "")
	fakeControlPlane(t, func(r *http.Request) {
		a := r.Header.Get("Authorization")
		if !strings.HasPrefix(a, "AWS4-HMAC-SHA256 Credential=AKIDEXAMPLE/") || !strings.Contains(a, "/eu-west-1/bedrock/aws4_request") {
			t.Errorf("sigv4 auth = %q", a)
		}
	})
	if _, err := New(Config{Region: "eu-west-1"}).ListModels(context.Background()); err != nil {
		t.Fatal(err)
	}
}
