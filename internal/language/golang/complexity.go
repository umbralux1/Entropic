package golang

import sitter "github.com/tree-sitter/go-tree-sitter"

// structures break linear flow: each adds 1 plus the current nesting depth
// to Complexity and nests what is inside it. A switch or select counts once,
// not per case.
var structures = map[string]bool{
	"if_statement":                true,
	"for_statement":               true,
	"expression_switch_statement": true,
	"type_switch_statement":       true,
	"select_statement":            true,
}

// jumps add 1, without a nesting penalty, when they name a label.
var jumps = map[string]bool{
	"break_statement":    true,
	"continue_statement": true,
	"goto_statement":     true,
}

// complexity follows G. Ann Campbell's cognitive complexity (SonarSource, 2017).
func complexity(fn *sitter.Node, source []byte) int {
	total := children(fn, 0)
	if callsItself(fn, fn, source) {
		total++
	}
	return total
}

// callsItself reports whether n, inside function fn, calls fn directly: by
// name for a function, through its receiver for a method.
func callsItself(n, fn *sitter.Node, source []byte) bool {
	if n.Kind() == "call_expression" && isSelf(n.ChildByFieldName("function"), fn, source) {
		return true
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if callsItself(n.NamedChild(i), fn, source) {
			return true
		}
	}
	return false
}

// isSelf reports whether callee names function fn.
func isSelf(callee, fn *sitter.Node, source []byte) bool {
	name := fn.ChildByFieldName("name").Utf8Text(source)
	receiver := receiverName(fn, source)
	switch callee.Kind() {
	case "identifier":
		return receiver == "" && callee.Utf8Text(source) == name
	case "selector_expression":
		operand := callee.ChildByFieldName("operand")
		return receiver != "" && operand.Kind() == "identifier" &&
			operand.Utf8Text(source) == receiver &&
			callee.ChildByFieldName("field").Utf8Text(source) == name
	}
	return false
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

func children(n *sitter.Node, nesting int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		total += score(n.Child(i), nesting)
	}
	return total
}

func score(n *sitter.Node, nesting int) int {
	switch {
	case n.Kind() == "if_statement":
		return 1 + nesting + ifChain(n, nesting)
	case structures[n.Kind()]:
		return 1 + nesting + children(n, nesting+1)
	case n.Kind() == "func_literal":
		return children(n, nesting+1)
	case logicalOperator(n) != "":
		ops, inner := booleanSequence(n, nesting)
		return runs(ops) + inner
	case jumps[n.Kind()] && n.NamedChildCount() > 0:
		return 1 // labelled: goto always is, break and continue may be
	default:
		return children(n, nesting)
	}
}

// ifChain scores an if statement's parts after its own increment: else if
// and else add 1 each without a nesting penalty, and nest what they contain.
func ifChain(n *sitter.Node, nesting int) int {
	total := 0
	alternative := n.ChildByFieldName("alternative")
	for i := uint(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if alternative == nil || child.Id() != alternative.Id() {
			total += score(child, nesting+1)
		}
	}
	switch {
	case alternative == nil:
		return total
	case alternative.Kind() == "if_statement":
		return total + 1 + ifChain(alternative, nesting)
	default:
		return total + 1 + children(alternative, nesting+1)
	}
}

// logicalOperator returns && or || for a boolean binary expression, else "".
func logicalOperator(n *sitter.Node) string {
	if n.Kind() != "binary_expression" {
		return ""
	}
	switch op := n.ChildByFieldName("operator").Kind(); op {
	case "&&", "||":
		return op
	}
	return ""
}

// booleanSequence returns, in source order, the boolean operators of the
// expression n heads (looking through parentheses), and the Complexity of
// its other operands.
func booleanSequence(n *sitter.Node, nesting int) (ops []string, inner int) {
	if n.Kind() == "parenthesized_expression" && n.NamedChildCount() == 1 &&
		logicalOperator(n.NamedChild(0)) != "" {
		return booleanSequence(n.NamedChild(0), nesting)
	}
	op := logicalOperator(n)
	if op == "" {
		return nil, score(n, nesting)
	}
	left, l := booleanSequence(n.ChildByFieldName("left"), nesting)
	right, r := booleanSequence(n.ChildByFieldName("right"), nesting)
	ops = append(append(left, op), right...)
	return ops, l + r
}

// runs counts the runs of like operators: && && || counts 2.
func runs(ops []string) int {
	n := 0
	for i, op := range ops {
		if i == 0 || op != ops[i-1] {
			n++
		}
	}
	return n
}
