package codeindex

import (
	"regexp"
	"strings"

	"github.com/msuozzo/bonsai"
	bonsaipy "github.com/msuozzo/bonsai/bonsai-python"
)

type pyExtractor struct{}

func init() {
	register("python", []string{".py", ".pyi"}, bonsaipy.NewParser, &pyExtractor{})
}

var allCapsRe = regexp.MustCompile(`^[A-Z][A-Z0-9_]*$`)

func (e *pyExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsaipy.KindImportStatement, bonsaipy.KindImportFromStatement:
			entries = append(entries, e.extractImport(child, src))
		case bonsaipy.KindClassDefinition:
			entries = append(entries, e.extractClass(child, src))
		case bonsaipy.KindFunctionDefinition:
			entries = append(entries, e.extractFunction(child, src))
		case bonsaipy.KindDecoratedDefinition:
			// Unwrap decorated definitions: the actual function/class is a child.
			for _, c := range child.Children {
				switch c.Kind {
				case bonsaipy.KindFunctionDefinition:
					entries = append(entries, e.extractFunction(c, src))
				case bonsaipy.KindClassDefinition:
					entries = append(entries, e.extractClass(c, src))
				}
			}
		case bonsaipy.KindExpressionStatement:
			// Module-level constant: ALL_CAPS = value
			if entry, ok := e.extractConstant(child, src); ok {
				entries = append(entries, entry)
			}
		}
	}
	return entries
}

func (e *pyExtractor) extractImport(node *bonsai.Node, src []byte) Entry {
	return Entry{
		Section:   SectionImport,
		Detail:    compactText(node, src),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *pyExtractor) extractClass(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaipy.FieldName)
	superclasses := textByField(node, src, bonsaipy.FieldSuperclasses)
	detail := name
	if superclasses != "" {
		detail += superclasses
	}

	var methods []string
	body := node.ChildByField(bonsaipy.FieldBody)
	if body != nil {
		for _, c := range body.Children {
			switch c.Kind {
			case bonsaipy.KindFunctionDefinition:
				mName := textByField(c, src, bonsaipy.FieldName)
				params := textByField(c, src, bonsaipy.FieldParameters)
				retType := textByField(c, src, bonsaipy.FieldReturnType)
				sig := mName + params
				if retType != "" {
					sig += " -> " + retType
				}
				methods = append(methods, sig)
			}
		}
	}
	return Entry{
		Section:   SectionClass,
		Name:      name,
		Detail:    detail,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    methods,
	}
}

func (e *pyExtractor) extractFunction(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaipy.FieldName)
	params := textByField(node, src, bonsaipy.FieldParameters)
	retType := textByField(node, src, bonsaipy.FieldReturnType)
	sig := name + params
	if retType != "" {
		sig += " -> " + retType
	}
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    sig,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *pyExtractor) extractConstant(node *bonsai.Node, src []byte) (Entry, bool) {
	// ExpressionStatement containing an Assignment where LHS is ALL_CAPS
	for _, c := range node.Children {
		if c.Kind == bonsaipy.KindAssignment {
			left := c.ChildByField(bonsaipy.FieldLeft)
			if left != nil {
				name := strings.TrimSpace(string(left.Text(src)))
				if allCapsRe.MatchString(name) {
					return Entry{
						Section:   SectionConst,
						Detail:    compactText(c, src),
						StartLine: int(node.StartPoint.Row) + 1,
						EndLine:   int(node.EndPoint.Row) + 1,
					}, true
				}
			}
		}
	}
	return Entry{}, false
}
