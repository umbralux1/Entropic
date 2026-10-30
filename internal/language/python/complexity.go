package python

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/umbralux1/entropic/internal/analysis"
)

// complexityRules are Python's node kinds for Complexity, after G. Ann
// Campbell's cognitive complexity (SonarSource, 2017), and for cyclomatic
// complexity.
var complexityRules = analysis.ComplexityRules{
	// A match counts once, not per case.
	Structures: map[string]bool{
		"if_statement":    true,
		"for_statement":   true,
		"while_statement": true,
		"except_clause":   true,
		"match_statement": true,
		// a if c else b
		"conditional_expression": true,
	},
	// elif and else. The else of a loop or try is part of that structure
	// and adds nothing.
	Branching: map[string]bool{"if_statement": true},
	// A comprehension's loops and filters.
	Clauses: map[string]bool{
		"for_in_clause": true,
		"if_clause":     true,
	},
	Closures: map[string]bool{
		"function_definition": true,
		"lambda":              true,
	},
	BooleanExpressions: map[string]bool{"boolean_operator": true},
	BooleanOperators:   map[string]bool{"and": true, "or": true},
	Parentheses:        map[string]bool{"parenthesized_expression": true},
	Calls:              map[string]bool{"call": true},
	Identifier:         "identifier",
	MemberAccess:       analysis.MemberAccess{Kind: "attribute", Object: "object", Member: "attribute"},
	Receiver:           receiverName,
	// A match adds one path per case, not counting a wildcard case.
	Decisions: map[string]bool{
		"if_statement":           true,
		"elif_clause":            true,
		"for_statement":          true,
		"while_statement":        true,
		"except_clause":          true,
		"conditional_expression": true,
		"for_in_clause":          true,
		"if_clause":              true,
		"case_clause":            true,
	},
	IsDefaultCase: isWildcard,
}

// receiverName is the name of a method's first parameter, usually self or
// cls, or "" for a function outside a class.
func receiverName(fn *sitter.Node, source []byte) string {
	if enclosingClass(fn) == nil {
		return ""
	}
	params := fn.ChildByFieldName("parameters")
	if params.NamedChildCount() == 0 || params.NamedChild(0).Kind() != "identifier" {
		return ""
	}
	return params.NamedChild(0).Utf8Text(source)
}

// enclosingClass returns the class a definition sits directly in, looking
// through decorators, or nil.
func enclosingClass(def *sitter.Node) *sitter.Node {
	p := def.Parent()
	if p != nil && p.Kind() == "decorated_definition" {
		p = p.Parent()
	}
	if p == nil || p.Kind() != "block" {
		return nil
	}
	if class := p.Parent(); class != nil && class.Kind() == "class_definition" {
		return class
	}
	return nil
}

// isWildcard reports whether n is case _: with no guard, which matches
// whatever is left, like a switch's default.
func isWildcard(n *sitter.Node) bool {
	if n.Kind() != "case_clause" || n.ChildByFieldName("guard") != nil || n.NamedChildCount() == 0 {
		return false
	}
	pattern := n.NamedChild(0)
	return pattern.Kind() == "case_pattern" && pattern.NamedChildCount() == 0
}
