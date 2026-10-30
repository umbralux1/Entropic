// Package analysis turns source code into Units with their Signal values.
package analysis

import (
	"fmt"

	sitter "github.com/tree-sitter/go-tree-sitter"
)

// UnitKind is the kind of code a Unit covers.
type UnitKind string

// Unit kinds, from finest to coarsest.
const (
	Function   UnitKind = "function"
	File       UnitKind = "file"
	Module     UnitKind = "module"
	Repository UnitKind = "repository"
)

// Unit is a piece of code a Signal is measured on. Lines are 1-based.
type Unit struct {
	Kind       UnitKind
	Name       string
	StartLine  int
	EndLine    int
	Complexity int
	// Cyclomatic is McCabe's cyclomatic complexity: informational only, it
	// feeds no Signal or Entropy Score.
	Cyclomatic int
}

// Language describes how to read one programming language: its grammar and
// which syntax nodes matter to each Signal.
type Language struct {
	Name    string
	Grammar *sitter.Language
	// Extensions are the file extensions, with leading dot, written in this language.
	Extensions []string
	// Functions are node kinds that form a function Unit.
	Functions map[string]bool
	// UnitName names the function Unit formed by fn.
	UnitName func(fn *sitter.Node, source []byte) string
	// Complexity scores the function Unit formed by fn.
	Complexity func(fn *sitter.Node, source []byte) int
	// Cyclomatic counts the independent paths through fn.
	Cyclomatic func(fn *sitter.Node) int
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
			units = append(units, Unit{
				Kind:       Function,
				Name:       lang.UnitName(n, source),
				StartLine:  int(n.StartPosition().Row) + 1,
				EndLine:    int(n.EndPosition().Row) + 1,
				Complexity: lang.Complexity(n, source),
				Cyclomatic: lang.Cyclomatic(n),
			})
		}
		for i := uint(0); i < n.ChildCount(); i++ {
			visit(n.Child(i))
		}
	}
	visit(tree.RootNode())
	return units, nil
}
