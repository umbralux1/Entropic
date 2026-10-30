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

// UnitKinds lists every kind of Unit, from finest to coarsest.
var UnitKinds = []UnitKind{Function, Class, File, Module, Repository}

// Location is where a piece of code lives: a slash-separated path relative
// to the repository root and a 1-based, inclusive line range. A Module has a
// path but no lines; the repository has neither.
type Location struct {
	Path      string `json:"path,omitempty"`
	StartLine int    `json:"startLine,omitempty"`
	EndLine   int    `json:"endLine,omitempty"`
}

// String is the location as path:start-end, or the path alone without lines.
func (l Location) String() string {
	if l.StartLine == 0 {
		return l.Path
	}
	return fmt.Sprintf("%s:%d-%d", l.Path, l.StartLine, l.EndLine)
}

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
	// Complexity says which nodes count towards a function's Complexity and
	// its informational cyclomatic complexity.
	Complexity ComplexityRules
	// VendorDirs are directory names holding vendored code, not measured.
	VendorDirs []string
	// TestFiles are path.Match patterns for the names of test files.
	TestFiles []string
	// TestDirs are directory names, matched exactly, holding test code.
	TestDirs []string
	// Generated matches source marked as generated, which is not measured.
	Generated *regexp.Regexp
	// Abstracted are node kinds, identifiers and literals, whose text
	// Duplication ignores, so that renamed copies still count as clones.
	Abstracted map[string]bool
	// IgnoredByDuplication are node kinds, such as imports and comments, that
	// Duplication skips: they neither form a clone nor tell two clones apart.
	IgnoredByDuplication map[string]bool
}

// IsTest reports whether a slash-separated path holds test code: its file
// name matches a TestFiles pattern, or a directory above it is a TestDirs entry.
func (l Language) IsTest(p string) bool {
	dirs := strings.Split(p, "/")
	name := dirs[len(dirs)-1]
	for _, pattern := range l.TestFiles {
		if ok, _ := path.Match(pattern, name); ok {
			return true
		}
	}
	for _, d := range dirs[:len(dirs)-1] {
		if slices.Contains(l.TestDirs, d) {
			return true
		}
	}
	return false
}

// IsGenerated reports whether source is marked as generated.
func (l Language) IsGenerated(source []byte) bool {
	return l.Generated != nil && l.Generated.Match(source)
}

// IsVendored reports whether a slash-separated path lies in a vendored directory.
func (l Language) IsVendored(path string) bool {
	dirs := strings.Split(path, "/")
	for _, d := range dirs[:len(dirs)-1] {
		if slices.Contains(l.VendorDirs, d) {
			return true
		}
	}
	return false
}

// SyntaxError is source that does not parse in its language.
type SyntaxError struct {
	// Line is the 1-based line of the first error.
	Line int
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("syntax error at line %d", e.Line)
}

// parse parses source in the given language, failing with a *SyntaxError
// if any of it does not parse. The caller closes the tree.
func parse(source []byte, lang Language) (*sitter.Tree, error) {
	parser := sitter.NewParser()
	defer parser.Close()
	if err := parser.SetLanguage(lang.Grammar); err != nil {
		return nil, fmt.Errorf("set %s grammar: %w", lang.Name, err)
	}
	tree := parser.Parse(source, nil)
	if root := tree.RootNode(); root.HasError() {
		line := int(firstError(root).StartPosition().Row) + 1
		tree.Close()
		return nil, &SyntaxError{Line: line}
	}
	return tree, nil
}

// Analyze parses source in the given language and returns its function and
// class Units in source order.
func Analyze(source []byte, lang Language) ([]Unit, error) {
	tree, err := parse(source, lang)
	if err != nil {
		return nil, err
	}
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
			unit.Complexity = complexity(n, source, lang.Complexity)
			unit.Cyclomatic = cyclomatic(n, lang.Complexity)
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
