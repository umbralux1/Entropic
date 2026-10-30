// Package python is the language adapter for Python.
package python

import (
	sitter "github.com/tree-sitter/go-tree-sitter"
	tspython "github.com/tree-sitter/tree-sitter-python/bindings/go"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Language tells analysis how to read Python.
var Language = analysis.Language{
	Name:       "python",
	Grammar:    sitter.NewLanguage(tspython.Language()),
	Extensions: []string{".py"},
	Functions: map[string]bool{
		"function_definition": true,
	},
	Classes: map[string]bool{
		"class_definition": true,
	},
	UnitName:   unitName,
	Complexity: complexity,
	Cyclomatic: cyclomatic,
}

// unitName qualifies a definition's name with the classes it is in, so that
// same-named methods in different classes stay distinct: Outer.Inner.method.
func unitName(def *sitter.Node, source []byte) string {
	name := def.ChildByFieldName("name").Utf8Text(source)
	for p := def.Parent(); p != nil; p = p.Parent() {
		if p.Kind() == "class_definition" {
			name = p.ChildByFieldName("name").Utf8Text(source) + "." + name
		}
	}
	return name
}
