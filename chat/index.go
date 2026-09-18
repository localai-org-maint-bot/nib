package chat

import (
	"github.com/mudler/cogito"
	"github.com/mudler/nib/codeindex"
)

type indexArgs struct {
	Path string `json:"path" jsonschema:"path to the source file (relative to the workspace or absolute)"`
}

type indexTool struct {
	resolvePath func(string) string
}

func (t *indexTool) Run(args map[string]any) (string, any, error) {
	path, _ := args["path"].(string)
	if path == "" {
		return "index error: 'path' is required", nil, nil
	}
	out, err := codeindex.Index(t.resolvePath(path))
	if err != nil {
		return "index failed: " + err.Error(), nil, nil
	}
	return out, nil, nil
}

func indexToolDefinition(resolvePath func(string) string) cogito.ToolDefinitionInterface {
	return cogito.NewToolDefinition[map[string]any](&indexTool{resolvePath: resolvePath}, indexArgs{},
		"index",
		"Return a compact overview of a source file: imports, type definitions, function signatures, "+
			"and structure with their line numbers surrounded by []. ~70-90% more efficient than reading the full file.\n\n"+
			"Use this FIRST to understand file structure before using read with offset/limit.\n"+
			"Supports source files by extension (currently .go). "+
			"Falls back with an error for unsupported file types.",
	)
}
