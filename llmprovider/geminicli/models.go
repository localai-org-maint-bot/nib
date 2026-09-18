package geminicli

import (
	"context"

	"github.com/mudler/nib/llmprovider/google"
)

// ListModels offers google.KnownChatModels: Cloud Code Assist has no endpoint
// that enumerates the models the Gemini CLI can use. It makes no request.
func (l *LLM) ListModels(context.Context) ([]string, error) {
	return google.KnownChatModels(), nil
}

// ModelListIsPartial: the built-in list is a suggestion.
func (l *LLM) ModelListIsPartial() bool { return true }
