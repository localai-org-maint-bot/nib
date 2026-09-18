package azureresponses

import (
	"context"

	"github.com/mudler/nib/llmprovider/registry"
)

// ListModels offers the model names mapped in AZURE_OPENAI_DEPLOYMENT_NAME_MAP.
// An Azure resource's deployments can only be enumerated through the Azure
// management plane (subscription, resource group, Entra ID token), which an
// API key cannot reach, and the data-plane /models lists base models rather
// than the deployment names requests need. Without a map the picker asks for
// the deployment name instead.
func (l *LLM) ListModels(context.Context) ([]string, error) {
	pairs := deploymentMap()
	if len(pairs) == 0 {
		return nil, registry.ErrNoModelList
	}
	ids := make([]string, len(pairs))
	for i, kv := range pairs {
		ids[i] = kv[0]
	}
	return ids, nil
}

// ModelListIsPartial: the deployment map need not name every deployment.
func (l *LLM) ModelListIsPartial() bool { return true }
