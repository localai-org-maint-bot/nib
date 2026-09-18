package geminicli

import (
	"context"
	"testing"
)

func TestListModelsOffersKnownModels(t *testing.T) {
	ids, err := New(Config{}).ListModels(context.Background())
	if err != nil || len(ids) == 0 || ids[0] != "gemini-2.5-pro" {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
}
