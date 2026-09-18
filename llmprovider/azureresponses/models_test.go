package azureresponses

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/mudler/nib/llmprovider/registry"
)

func TestListModelsFromDeploymentMap(t *testing.T) {
	t.Setenv("AZURE_OPENAI_DEPLOYMENT_NAME_MAP", "")
	if _, err := New(Config{}).ListModels(context.Background()); !errors.Is(err, registry.ErrNoModelList) {
		t.Fatalf("without a map: err = %v, want ErrNoModelList", err)
	}
	t.Setenv("AZURE_OPENAI_DEPLOYMENT_NAME_MAP", "gpt-4.1=prod-gpt41, o4-mini = reasoning ,bad")
	ids, err := New(Config{}).ListModels(context.Background())
	if err != nil || !reflect.DeepEqual(ids, []string{"gpt-4.1", "o4-mini"}) {
		t.Fatalf("ids = %v, err = %v", ids, err)
	}
	if got := resolveDeploymentName("o4-mini"); got != "reasoning" {
		t.Fatalf("resolveDeploymentName = %q", got)
	}
}
