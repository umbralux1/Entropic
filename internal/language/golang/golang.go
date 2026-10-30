// Package golang is the language adapter for Go.
package golang

import (
	"regexp"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Language tells analysis how to read Go.
var Language = analysis.Language{
	Name:       "go",
	Grammar:    sitter.NewLanguage(tsgo.Language()),
	Extensions: []string{".go"},
	Functions: map[string]bool{
		"function_declaration": true,
		"method_declaration":   true,
	},
	UnitName:   unitName,
	Complexity: complexity,
	Cyclomatic: cyclomatic,
	VendorDirs: []string{"vendor"},
	TestPaths:  []string{"*_test.go"},
	// The convention from "go help generate".
	Generated: regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`),
}

// unitName names a function by its identifier and a method by its receiver
// type and identifier, so that same-named methods on different types stay
// distinct: func (*T) M is "T.M".
func unitName(fn *sitter.Node, source []byte) string {
	name := fn.ChildByFieldName("name").Utf8Text(source)
	if t := receiverType(fn); t != nil {
		return t.Utf8Text(source) + "." + name
	}
	return name
}

// receiverType returns the type identifier of a method's receiver, without
// pointer or type arguments, or nil for a plain function.
func receiverType(fn *sitter.Node) *sitter.Node {
	receiver := fn.ChildByFieldName("receiver")
	if receiver == nil || receiver.NamedChildCount() == 0 {
		return nil
	}
	t := receiver.NamedChild(0).ChildByFieldName("type")
	for t != nil {
		switch t.Kind() {
		case "pointer_type":
			t = t.NamedChild(0)
		case "generic_type":
			t = t.ChildByFieldName("type")
		case "parenthesized_type":
			t = t.NamedChild(0)
		default:
			return t
		}
	}
	return nil
}
