package golang

import sitter "github.com/tree-sitter/go-tree-sitter"

// decisions each add one path to cyclomatic complexity. A switch adds one
// per case, not counting default.
var decisions = map[string]bool{
	"if_statement":       true,
	"for_statement":      true,
	"expression_case":    true,
	"type_case":          true,
	"communication_case": true,
}

// cyclomatic is McCabe's cyclomatic complexity: 1 plus each decision and
// each && or ||.
func cyclomatic(fn *sitter.Node) int {
	return 1 + countDecisions(fn)
}

func countDecisions(n *sitter.Node) int {
	total := 0
	if decisions[n.Kind()] || logicalOperator(n) != "" {
		total++
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		total += countDecisions(n.NamedChild(i))
	}
	return total
}
