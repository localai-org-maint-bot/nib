package chat

import "github.com/mudler/cogito"

// askUserArgs is the JSON-schema shape of the ask_user tool's parameters.
type askUserArgs struct {
	Question    string   `json:"question" jsonschema:"the question to ask the user"`
	Options     []string `json:"options,omitempty" jsonschema:"2-6 concise choices the user picks from instead of typing. Provide these whenever the answer set is known or bounded; omit only for genuinely open-ended questions."`
	MultiSelect bool     `json:"multi_select,omitempty" jsonschema:"when true the user may pick several options (checkbox); when false or omitted they pick exactly one (radio). Only meaningful with options."`
}

// askUserTool is a cogito tool that asks the user a question and returns the
// answer. It satisfies cogito.Tool[map[string]any].
type askUserTool struct {
	ask func(AskRequest) string
}

// Run parses the tool arguments, asks the user, and returns the answer string.
func (a *askUserTool) Run(args map[string]any) (string, any, error) {
	q, _ := args["question"].(string)
	var opts []string
	switch v := args["options"].(type) {
	case []any:
		for _, o := range v {
			if s, ok := o.(string); ok {
				opts = append(opts, s)
			}
		}
	case []string:
		opts = v
	}
	multi, _ := args["multi_select"].(bool)
	if a.ask == nil {
		return "", nil, nil
	}
	return a.ask(AskRequest{Question: q, Options: opts, MultiSelect: multi}), nil, nil
}

// askUserToolDefinition builds the cogito tool definition for ask_user.
func askUserToolDefinition(ask func(AskRequest) string) cogito.ToolDefinitionInterface {
	return cogito.NewToolDefinition[map[string]any](
		&askUserTool{ask: ask},
		askUserArgs{},
		"ask_user",
		"Ask the user a clarifying question and wait for their answer. Prefer offering `options` (2-6 concise choices) so the user can pick from a list instead of typing — do this whenever the question has a known or likely set of answers. Omit `options` only for genuinely open-ended questions. Set `multi_select` to true when several options may be chosen at once (checkbox) rather than exactly one (radio). Exhaust code, configs, docs, and history before asking; use this only for information only the user can provide.",
	)
}
