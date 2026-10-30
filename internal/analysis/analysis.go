// Package analysis turns source code into Units with their Signal values.
package analysis

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// UnitKind is the kind of code a Unit covers.
type UnitKind string

const Function UnitKind = "function"

// Unit is a piece of code a Signal is measured on. Lines are 1-based.
type Unit struct {
	Kind       UnitKind
	Name       string
	StartLine  int
	EndLine    int
	Complexity int
}

// Language describes how to read one programming language: its grammar and
// which syntax nodes matter to each Signal.
type Language struct {
	Name    string
	Grammar *sitter.Language
	// Extensions are the file extensions, with leading dot, written in this language.
	Extensions []string
	// Functions are node kinds that form a function Unit; their "name" field names it.
	Functions map[string]bool
	// Branches are node kinds that break linear flow: each adds 1 plus the
	// current nesting depth to Complexity, and nests what is inside it.
	Branches map[string]bool
}

// Analyze parses source in the given language and returns its function Units
// in source order.
func Analyze(source []byte, lang Language) ([]Unit, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang.Grammar); err != nil {
		return nil, fmt.Errorf("set %s grammar: %w", lang.Name, err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()

	var units []Unit
	var visit func(n *sitter.Node)
	visit = func(n *sitter.Node) {
		if lang.Functions[n.Kind()] {
			name := ""
			if id := n.ChildByFieldName("name"); id != nil {
				name = id.Utf8Text(source)
			}
			units = append(units, Unit{
				Kind:       Function,
				Name:       name,
				StartLine:  int(n.StartPosition().Row) + 1,
				EndLine:    int(n.EndPosition().Row) + 1,
				Complexity: complexity(n, lang, 0),
			})
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			visit(n.Child(i))
		}
	}
	visit(tree.RootNode())
	return units, nil
}

func complexity(n *sitter.Node, lang Language, nesting int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if lang.Branches[child.Kind()] {
			total += 1 + nesting + complexity(child, lang, nesting+1)
		} else {
			total += complexity(child, lang, nesting)
		}
	}
	return total
}
