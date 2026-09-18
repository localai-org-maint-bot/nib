package codeindex

import (
	"github.com/msuozzo/bonsai"
	bonsaibash "github.com/msuozzo/bonsai/bonsai-bash"
)

type bashExtractor struct{}

func init() {
	register("bash", []string{".sh", ".bash"}, bonsaibash.NewParser, &bashExtractor{})
}

func (e *bashExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsaibash.KindFunctionDefinition:
			entries = append(entries, e.extractFunction(child, src))
		case bonsaibash.KindVariableAssignment, bonsaibash.KindVariableAssignments:
			entries = append(entries, e.extractVar(child, src))
		}
	}
	return entries
}

func (e *bashExtractor) extractFunction(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaibash.FieldName)
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    name + "()",
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *bashExtractor) extractVar(node *bonsai.Node, src []byte) Entry {
	return Entry{
		Section:   SectionVar,
		Detail:    compactText(node, src),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}
