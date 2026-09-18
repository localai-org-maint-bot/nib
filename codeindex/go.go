package codeindex

import (
	"strings"

	"github.com/msuozzo/bonsai"
	bonsaigo "github.com/msuozzo/bonsai/bonsai-go"
)

type goExtractor struct{}

func init() {
	register("go", []string{".go"}, bonsaigo.NewParser, &goExtractor{})
}

func (e *goExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsaigo.KindPackageClause:
			entries = append(entries, e.extractPackage(child, src))
		case bonsaigo.KindImportDeclaration:
			if entry, ok := e.extractImport(child, src); ok {
				entries = append(entries, entry)
			}
		case bonsaigo.KindFunctionDeclaration:
			entries = append(entries, e.extractFunction(child, src))
		case bonsaigo.KindMethodDeclaration:
			entries = append(entries, e.extractMethod(child, src))
		case bonsaigo.KindTypeDeclaration:
			for spec := range child.Find(bonsaigo.KindTypeSpec) {
				entries = append(entries, e.extractType(spec, src))
			}
			for spec := range child.Find(bonsaigo.KindTypeAlias) {
				entries = append(entries, e.extractTypeAlias(spec, src))
			}
		case bonsaigo.KindConstDeclaration:
			for spec := range child.Find(bonsaigo.KindConstSpec) {
				entries = append(entries, e.extractConst(spec, src))
			}
		case bonsaigo.KindVarDeclaration:
			for spec := range child.Find(bonsaigo.KindVarSpec) {
				entries = append(entries, e.extractVar(spec, src))
			}
		}
	}
	return entries
}

func (e *goExtractor) extractPackage(node *bonsai.Node, src []byte) Entry {
	// tree-sitter-go doesn't assign a field name to package_identifier
	// inside package_clause, so find it by kind.
	name := ""
	for _, c := range node.Children {
		if c.Kind == bonsaigo.KindPackageIdentifier {
			name = string(c.Text(src))
			break
		}
	}
	return Entry{
		Section:   SectionPackage,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *goExtractor) extractImport(node *bonsai.Node, src []byte) (Entry, bool) {
	var paths []string
	for spec := range node.Find(bonsaigo.KindImportSpec) {
		s := ""
		if name := spec.ChildByField(bonsaigo.FieldName); name != nil {
			s = string(name.Text(src)) + " "
		}
		if p := spec.ChildByField(bonsaigo.FieldPath); p != nil {
			s += string(p.Text(src))
		}
		if s != "" {
			paths = append(paths, s)
		}
	}
	if len(paths) == 0 {
		return Entry{}, false
	}
	return Entry{
		Section:   SectionImport,
		Detail:    strings.Join(paths, ", "),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}, true
}

func (e *goExtractor) extractFunction(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaigo.FieldName)
	params := textByField(node, src, bonsaigo.FieldParameters)
	result := textByField(node, src, bonsaigo.FieldResult)
	sig := name + params
	if result != "" {
		sig += " " + result
	}
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    sig,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *goExtractor) extractMethod(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaigo.FieldName)
	receiver := textByField(node, src, bonsaigo.FieldReceiver)
	params := textByField(node, src, bonsaigo.FieldParameters)
	result := textByField(node, src, bonsaigo.FieldResult)
	sig := receiver + " " + name + params
	if result != "" {
		sig += " " + result
	}
	return Entry{
		Section:   SectionMethod,
		Name:      name,
		Detail:    sig,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *goExtractor) extractType(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaigo.FieldName)
	typeNode := node.ChildByField(bonsaigo.FieldType)
	typeKind := "type"
	var fields []string
	if typeNode != nil {
		switch typeNode.Kind {
		case bonsaigo.KindStructType:
			typeKind = "struct"
			fields = extractStructFields(typeNode, src)
		case bonsaigo.KindInterfaceType:
			typeKind = "interface"
			fields = extractInterfaceMethods(typeNode, src)
		case bonsaigo.KindFunctionType:
			typeKind = "func " + compactText(typeNode, src)
		default:
			typeKind = compactText(typeNode, src)
		}
	}
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " " + typeKind,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    fields,
	}
}

func (e *goExtractor) extractTypeAlias(node *bonsai.Node, src []byte) Entry {
	name := textByField(node, src, bonsaigo.FieldName)
	aliasType := textByField(node, src, bonsaigo.FieldType)
	return Entry{
		Section:   SectionType,
		Name:      name,
		Detail:    name + " = " + aliasType,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *goExtractor) extractConst(node *bonsai.Node, src []byte) Entry {
	return Entry{
		Section:   SectionConst,
		Detail:    compactText(node, src),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func (e *goExtractor) extractVar(node *bonsai.Node, src []byte) Entry {
	return Entry{
		Section:   SectionVar,
		Detail:    compactText(node, src),
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}

func extractStructFields(structNode *bonsai.Node, src []byte) []string {
	var fields []string
	for fd := range structNode.Find(bonsaigo.KindFieldDeclaration) {
		fields = append(fields, compactText(fd, src))
	}
	return fields
}

func extractInterfaceMethods(ifaceNode *bonsai.Node, src []byte) []string {
	var methods []string
	for me := range ifaceNode.Find(bonsaigo.KindMethodElem) {
		name := textByField(me, src, bonsaigo.FieldName)
		params := textByField(me, src, bonsaigo.FieldParameters)
		result := textByField(me, src, bonsaigo.FieldResult)
		sig := name + params
		if result != "" {
			sig += " " + result
		}
		methods = append(methods, sig)
	}
	return methods
}

func textByField(node *bonsai.Node, src []byte, field string) string {
	if n := node.ChildByField(field); n != nil {
		return strings.Join(strings.Fields(string(n.Text(src))), " ")
	}
	return ""
}

func compactText(node *bonsai.Node, src []byte) string {
	return strings.Join(strings.Fields(string(node.Text(src))), " ")
}
