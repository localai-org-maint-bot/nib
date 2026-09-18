package codeindex

import (
	"strings"

	"github.com/msuozzo/bonsai"
	bonsairuby "github.com/msuozzo/bonsai/bonsai-ruby"
)

type rubyExtractor struct{}

func init() {
	register("ruby", []string{".rb"}, bonsairuby.NewParser, &rubyExtractor{})
}

func (e *rubyExtractor) Extract(root *bonsai.Node, src []byte) []Entry {
	var entries []Entry
	for _, child := range root.Children {
		switch child.Kind {
		case bonsairuby.KindCall:
			// require / require_relative
			method := textByField(child, src, bonsairuby.FieldMethod)
			if method == "require" || method == "require_relative" {
				arg := ""
				if args := child.ChildByField(bonsairuby.FieldArguments); args != nil {
					arg = compactText(args, src)
				}
				entries = append(entries, Entry{
					Section:   SectionImport,
					Detail:    method + " " + arg,
					StartLine: int(child.StartPoint.Row) + 1,
					EndLine:   int(child.EndPoint.Row) + 1,
				})
			}
		case bonsairuby.KindClass:
			entries = append(entries, e.extractClass(child, src))
		case bonsairuby.KindModule:
			entries = append(entries, e.extractModule(child, src))
		case bonsairuby.KindMethod:
			entries = append(entries, e.extractMethod(child, src, ""))
		case bonsairuby.KindSingletonMethod:
			entries = append(entries, e.extractMethod(child, src, "self."))
		case bonsairuby.KindAssignment:
			// Constants: LHS starts with uppercase
			left := child.ChildByField(bonsairuby.FieldLeft)
			if left != nil {
				name := strings.TrimSpace(string(left.Text(src)))
				if name != "" && name[0] >= 'A' && name[0] <= 'Z' {
					entries = append(entries, Entry{
						Section:   SectionConst,
						Detail:    compactText(child, src),
						StartLine: int(child.StartPoint.Row) + 1,
						EndLine:   int(child.EndPoint.Row) + 1,
					})
				}
			}
		}
	}
	return entries
}

func (e *rubyExtractor) extractClass(node *bonsai.Node, src []byte) Entry {
	name := ""
	for _, c := range node.Children {
		if c.Kind == bonsairuby.KindConstant {
			name = string(c.Text(src))
			break
		}
	}
	detail := name
	if sc := node.ChildByField(bonsairuby.FieldSuperclass); sc != nil {
		detail += " < " + compactText(sc, src)
	}
	var methods []string
	if body := node.ChildByField(bonsairuby.FieldBody); body != nil {
		for _, c := range body.Children {
			switch c.Kind {
			case bonsairuby.KindMethod:
				mName := textByField(c, src, bonsairuby.FieldName)
				params := textByField(c, src, bonsairuby.FieldParameters)
				methods = append(methods, mName+params)
			case bonsairuby.KindSingletonMethod:
				mName := textByField(c, src, bonsairuby.FieldName)
				params := textByField(c, src, bonsairuby.FieldParameters)
				methods = append(methods, "self."+mName+params)
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

func (e *rubyExtractor) extractModule(node *bonsai.Node, src []byte) Entry {
	name := ""
	for _, c := range node.Children {
		if c.Kind == bonsairuby.KindConstant {
			name = string(c.Text(src))
			break
		}
	}
	var methods []string
	if body := node.ChildByField(bonsairuby.FieldBody); body != nil {
		for _, c := range body.Children {
			switch c.Kind {
			case bonsairuby.KindMethod:
				mName := textByField(c, src, bonsairuby.FieldName)
				params := textByField(c, src, bonsairuby.FieldParameters)
				methods = append(methods, mName+params)
			case bonsairuby.KindSingletonMethod:
				mName := textByField(c, src, bonsairuby.FieldName)
				params := textByField(c, src, bonsairuby.FieldParameters)
				methods = append(methods, "self."+mName+params)
			}
		}
	}
	return Entry{
		Section:   SectionModule,
		Name:      name,
		Detail:    name,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
		Fields:    methods,
	}
}

func (e *rubyExtractor) extractMethod(node *bonsai.Node, src []byte, prefix string) Entry {
	name := textByField(node, src, bonsairuby.FieldName)
	params := textByField(node, src, bonsairuby.FieldParameters)
	return Entry{
		Section:   SectionFunc,
		Name:      name,
		Detail:    prefix + name + params,
		StartLine: int(node.StartPoint.Row) + 1,
		EndLine:   int(node.EndPoint.Row) + 1,
	}
}
