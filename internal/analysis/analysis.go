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
	Class      UnitKind = "class"
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
	// Classes are node kinds that form a class Unit, whose Complexity rolls
	// up the functions it encloses.
	Classes map[string]bool
	// UnitName names the function or class Unit formed by n.
	UnitName func(n *sitter.Node, source []byte) string
	// Complexity scores the function Unit formed by fn.
	Complexity func(fn *sitter.Node, source []byte) int
	// Cyclomatic counts the independent paths through fn.
	Cyclomatic func(fn *sitter.Node) int
}

// Analyze parses source in the given language and returns its function and
// class Units in source order.
func Analyze(source []byte, lang Language) ([]Unit, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang.Grammar); err != nil {
		return nil, fmt.Errorf("set %s grammar: %w", lang.Name, err)
	}
	tree := parser.Parse(source, nil)
	defer tree.Close()

	var units []Unit
	// visit appends the Units n forms and contains, and returns the
	// Complexity of the functions among them.
	var visit func(n *sitter.Node) int
	visit = func(n *sitter.Node) int {
		unit := Unit{
			StartLine: int(n.StartPosition().Row) + 1,
			EndLine:   int(n.EndPosition().Row) + 1,
		}
		switch {
		case lang.Functions[n.Kind()]:
			unit.Kind = Function
			unit.Name = lang.UnitName(n, source)
			unit.Complexity = lang.Complexity(n, source)
			unit.Cyclomatic = lang.Cyclomatic(n)
			units = append(units, unit)
			return unit.Complexity // a nested function is part of the function it is in
		case lang.Classes[n.Kind()]:
			unit.Kind = Class
			unit.Name = lang.UnitName(n, source)
			i := len(units)
			units = append(units, unit)
			units[i].Complexity = visitChildren(n, visit)
			return units[i].Complexity
		}
		return visitChildren(n, visit)
	}
	visit(tree.RootNode())
	return units, nil
}

func visitChildren(n *sitter.Node, visit func(*sitter.Node) int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		total += visit(n.Child(i))
	}
	return total
}
