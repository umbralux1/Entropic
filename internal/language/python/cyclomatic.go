package python

import sitter "github.com/tree-sitter/go-tree-sitter"

// decisions each add one path to cyclomatic complexity. A match adds one per
// case, not counting a wildcard case.
var decisions = map[string]bool{
	"if_statement":           true,
	"elif_clause":            true,
	"for_statement":          true,
	"while_statement":        true,
	"except_clause":          true,
	"conditional_expression": true,
	"for_in_clause":          true,
	"if_clause":              true,
	"boolean_operator":       true,
}

// cyclomatic is McCabe's cyclomatic complexity: 1 plus each decision.
func cyclomatic(fn *sitter.Node) int {
	return 1 + countDecisions(fn)
}

func countDecisions(n *sitter.Node) int {
	total := 0
	if decisions[n.Kind()] || (n.Kind() == "case_clause" && !isWildcard(n)) {
		total++
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		total += countDecisions(n.NamedChild(i))
	}
	return total
}

// isWildcard reports whether a case clause is case _: with no guard, which
// matches whatever is left, like a switch's default.
func isWildcard(c *sitter.Node) bool {
	if c.ChildByFieldName("guard") != nil || c.NamedChildCount() == 0 {
		return false
	}
	pattern := c.NamedChild(0)
	return pattern.Kind() == "case_pattern" && pattern.NamedChildCount() == 0
}
