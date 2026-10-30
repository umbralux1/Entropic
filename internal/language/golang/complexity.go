package golang

import (
	sitter "github.com/tree-sitter/go-tree-sitter"

	"github.com/umbralux1/entropic/internal/analysis"
)

// complexityRules are Go's node kinds for Complexity, after G. Ann
// Campbell's cognitive complexity (SonarSource, 2017), and for cyclomatic
// complexity.
var complexityRules = analysis.ComplexityRules{
	// A switch or select counts once, not per case.
	Structures: map[string]bool{
		"if_statement":                true,
		"for_statement":               true,
		"expression_switch_statement": true,
		"type_switch_statement":       true,
		"select_statement":            true,
	},
	// else if and else.
	Branching: map[string]bool{"if_statement": true},
	Closures:  map[string]bool{"func_literal": true},
	// goto always names a label; break and continue may.
	Jumps: map[string]bool{
		"break_statement":    true,
		"continue_statement": true,
		"goto_statement":     true,
	},
	BooleanExpressions: map[string]bool{"binary_expression": true},
	BooleanOperators:   map[string]bool{"&&": true, "||": true},
	Parentheses:        map[string]bool{"parenthesized_expression": true},
	Calls:              map[string]bool{"call_expression": true},
	Identifier:         "identifier",
	MemberAccess:       analysis.MemberAccess{Kind: "selector_expression", Object: "operand", Member: "field"},
	Receiver:           receiverName,
	// A switch adds one path per case, not counting default.
	Decisions: map[string]bool{
		"if_statement":       true,
		"for_statement":      true,
		"expression_case":    true,
		"type_case":          true,
		"communication_case": true,
	},
}

// receiverName is the name a method's receiver is bound to, or "".
func receiverName(fn *sitter.Node, source []byte) string {
	receiver := fn.ChildByFieldName("receiver")
	if receiver == nil || receiver.NamedChildCount() == 0 {
		return ""
	}
	if id := receiver.NamedChild(0).ChildByFieldName("name"); id != nil {
		return id.Utf8Text(source)
	}
	return ""
}
