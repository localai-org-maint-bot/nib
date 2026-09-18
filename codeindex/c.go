package codeindex

import (
	"github.com/msuozzo/bonsai"
	bonsaic "github.com/msuozzo/bonsai/bonsai-c"
)

type cExtractor struct{}

func init() {
	register("c", []string{".c", ".h"}, bonsaic.NewParser, &cExtractor{})
}

func (e *cExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		e.extractOne(child, src, &entries)
	}
	return entries
}

func (e *cExtractor) extractOne(node *bonsai.Node, src []byte, entries *[]Entry) {
	switch node.Kind {
	case bonsaic.KindPreprocInclude:
		path := textByField(node, src, bonsaic.FieldPath)
		*entries = append(*entries, Entry{
			Section:   SectionImport,
			Detail:    "#include " + path,
			StartLine: int(node.StartPoint.Row) + 1,
			EndLine:   int(node.StartPoint.Row) + 1,
		})
	case bonsaic.KindPreprocDef:
		*entries = append(*entries, Entry{
			Section:   SectionConst,
			Detail:    compactText(node, src),
			StartLine: int(node.StartPoint.Row) + 1,
			EndLine:   int(node.StartPoint.Row) + 1,
		})
	case bonsaic.KindPreprocFunctionDef:
		*entries = append(*entries, Entry{
			Section:   SectionMacro,
			Detail:    compactText(node, src),
			StartLine: int(node.StartPoint.Row) + 1,
			EndLine:   int(node.StartPoint.Row) + 1,
		})
	case bonsaic.KindFunctionDefinition:
		*entries = append(*entries, e.extractFunction(node, src))
	case bonsaic.KindTypeDefinition:
		*entries = append(*entries, e.extractTypeDef(node, src))
	case bonsaic.KindStructSpecifier:
		*entries = append(*entries, e.extractStruct(node, src, "struct"))
	case bonsaic.KindUnionSpecifier:
		*entries = append(*entries, e.extractStruct(node, src, "union"))
	case bonsaic.KindEnumSpecifier:
		*entries = append(*entries, e.extractEnum(node, src))
	case bonsaic.KindPreprocIfdef, bonsaic.KindPreprocIf, bonsaic.KindLinkageSpecification:
		// Recurse into preprocessor conditionals and extern "C" blocks.
		for _, c := range node.Children {
			e.extractOne(c, src, entries)
		}
	}
}

func (e *cExtractor) extractFunction(node *bonsai.Node, src []byte) Entry {
	// The function name is buried in the declarator tree.
	name := cFuncName(node, src)
	detail := name + cFuncParams(node, src)
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    detail,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

// cFuncName unwraps function_declarator and pointer_declarator to find the identifier.
func cFuncName(node *bonsai.Node, src []byte) string {
	decl := node.ChildByField(bonsaic.FieldDeclarator)
	if decl == nil {
		for _, c := range node.Children {
			if c.Kind == bonsaic.KindFunctionDeclarator {
				decl = c
				break
			}
		}
	}
	for decl != nil {
		switch decl.Kind {
		case bonsaic.KindFunctionDeclarator:
			inner := decl.ChildByField(bonsaic.FieldDeclarator)
			if inner != nil {
				decl = inner
				continue
			}
		case bonsaic.KindPointerDeclarator:
			inner := decl.ChildByField(bonsaic.FieldDeclarator)
			if inner != nil {
				decl = inner
				continue
			}
		case bonsaic.KindIdentifier:
			return string(decl.Text(src))
		case bonsaic.KindFieldIdentifier:
			return string(decl.Text(src))
		case bonsaic.KindParenthesizedDeclarator:
			for _, c := range decl.Children {
				if c.Kind == bonsaic.KindFunctionDeclarator || c.Kind == bonsaic.KindPointerDeclarator {
					decl = c
					break
				}
			}
			continue
		}
		break
	}
	if decl != nil {
		return compactText(decl, src)
	}
	return compactText(node, src)
}

// cFuncParams extracts the parameter list from a function definition.
func cFuncParams(node *bonsai.Node, src []byte) string {
	decl := node.ChildByField(bonsaic.FieldDeclarator)
	if decl == nil {
		for _, c := range node.Children {
			if c.Kind == bonsaic.KindFunctionDeclarator {
				decl = c
				break
			}
		}
	}
	for decl != nil {
		if decl.Kind == bonsaic.KindPointerDeclarator {
			inner := decl.ChildByField(bonsaic.FieldDeclarator)
			if inner != nil {
				decl = inner
				continue
			}
		}
		if decl.Kind == bonsaic.KindParenthesizedDeclarator {
			for _, c := range decl.Children {
				if c.Kind == bonsaic.KindFunctionDeclarator {
					decl = c
					break
				}
			}
			continue
		}
		if decl.Kind == bonsaic.KindFunctionDeclarator {
			params := decl.ChildByField(bonsaic.FieldParameters)
			if params != nil {
				return compactText(params, src)
			}
		}
		break
	}
	return "()"
}

func (e *cExtractor) extractTypeDef(node *bonsai.Node, src []byte) Entry {
	name := ""
	if decl := node.ChildByField(bonsaic.FieldDeclarator); decl != nil {
		name = cDeclaratorName(decl, src)
	}
	if name == "" {
		name = textByField(node, src, bonsaic.FieldName)
	}
	if name == "" {
		name = compactText(node, src)
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

// cDeclaratorName unwraps declarator nodes (function, pointer,
// parenthesized) to find the identifier name.
func cDeclaratorName(decl *bonsai.Node, src []byte) string {
	for decl != nil {
		switch decl.Kind {
		case bonsaic.KindFunctionDeclarator:
			if inner := decl.ChildByField(bonsaic.FieldDeclarator); inner != nil {
				decl = inner
				continue
			}
		case bonsaic.KindPointerDeclarator:
			if inner := decl.ChildByField(bonsaic.FieldDeclarator); inner != nil {
				decl = inner
				continue
			}
		case bonsaic.KindParenthesizedDeclarator:
			found := false
			for _, c := range decl.Children {
				if c.Kind == bonsaic.KindFunctionDeclarator || c.Kind == bonsaic.KindPointerDeclarator || c.Kind == bonsaic.KindIdentifier {
					decl = c
					found = true
					break
				}
			}
			if found {
				continue
			}
		case bonsaic.KindIdentifier, bonsaic.KindFieldIdentifier:
			return string(decl.Text(src))
		}
		break
	}
	if decl != nil {
		return compactText(decl, src)
	}
	return ""
}

func (e *cExtractor) extractStruct(node *bonsai.Node, src []byte, kind string) Entry {
	name := textByField(node, src, bonsaic.FieldName)
	if name == "" {
		name = kind
	}
	var fields []string
	for fl := range node.Find(bonsaic.KindFieldDeclarationList) {
		for fd := range fl.Find(bonsaic.KindFieldDeclaration) {
			fields = append(fields, compactText(fd, src))
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " " + kind,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    fields,
	}
}

func (e *cExtractor) extractEnum(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaic.FieldName)
	if name == "" {
		name = "enum"
	}
	var variants []string
	for el := range node.Find(bonsaic.KindEnumeratorList) {
		for en := range el.Find(bonsaic.KindEnumerator) {
			variants = append(variants, compactText(en, src))
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " enum",
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    variants,
	}
}
