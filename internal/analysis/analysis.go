// Package analysis turns source code into Units with their Signal values.
package analysis

import (
	"fmt"
	"path"
	"regexp"
	"slices"
	"strings"

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
	// VendorDirs are directory names holding vendored code, not measured.
	VendorDirs []string
	// TestPaths are path.Match patterns for test code: a file is a test when
	// its name or any directory above it matches one.
	TestPaths []string
	// Generated matches source marked as generated, which is not measured.
	Generated *regexp.Regexp
	// Abstracted are node kinds, identifiers and literals, whose text
	// Duplication ignores, so that renamed copies still count as clones.
	Abstracted map[string]bool
	// Unduplicable are node kinds, such as imports and comments, that
	// Duplication skips: they neither form a clone nor tell two clones apart.
	Unduplicable map[string]bool
}

// IsTest reports whether a slash-separated path holds test code.
func (l Language) IsTest(p string) bool {
	for _, elem := range strings.Split(p, "/") {
		for _, pattern := range l.TestPaths {
			if ok, _ := path.Match(pattern, elem); ok {
				return true
			}
		}
	}
	return false
}

// IsGenerated reports whether source is marked as generated.
func (l Language) IsGenerated(source []byte) bool {
	return l.Generated != nil && l.Generated.Match(source)
}

// Vendored reports whether a slash-separated path lies in a vendored directory.
func (l Language) Vendored(path string) bool {
	dirs := strings.Split(path, "/")
	for _, d := range dirs[:len(dirs)-1] {
		if slices.Contains(l.VendorDirs, d) {
			return true
		}
	}
	return false
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
	if root := tree.RootNode(); root.HasError() {
		return nil, fmt.Errorf("syntax error at line %d", firstError(root).StartPosition().Row+1)
	}

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

// firstError returns the first error or missing node below n, which has an
// error somewhere in its subtree; n itself if none is found.
func firstError(n *sitter.Node) *sitter.Node {
	if n.IsError() || n.IsMissing() {
		return n
	}
	for i := uint(0); i < n.ChildCount(); i++ {
		if c := n.Child(i); c.HasError() || c.IsMissing() {
			return firstError(c)
		}
	}
	return n
}

func visitChildren(n *sitter.Node, visit func(*sitter.Node) int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		total += visit(n.Child(i))
	}
	return total
}
