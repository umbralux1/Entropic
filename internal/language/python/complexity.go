package python

import sitter "github.com/tree-sitter/go-tree-sitter"

// structures break linear flow: each adds 1 plus the current nesting depth
// to Complexity and nests what is inside it. A match counts once, not per
// case.
var structures = map[string]bool{
	"if_statement":    true,
	"for_statement":   true,
	"while_statement": true,
	"except_clause":   true,
	"match_statement": true,
	// a if c else b
	"conditional_expression": true,
}

// clauses are a comprehension's loops and filters: each adds 1 plus the
// current nesting depth, but they do not nest each other, as they read as
// one flat pipeline.
var clauses = map[string]bool{
	"for_in_clause": true,
	"if_clause":     true,
}

// closures add nothing themselves but nest what is inside them.
var closures = map[string]bool{
	"function_definition": true,
	"lambda":              true,
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
// name for a function, through its first parameter (self or cls) for a
// method.
func callsItself(n, fn *sitter.Node, source []byte) bool {
	if n.Kind() == "call" && isSelf(n.ChildByFieldName("function"), fn, source) {
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
	case "attribute":
		object := callee.ChildByFieldName("object")
		return receiver != "" && object.Kind() == "identifier" &&
			object.Utf8Text(source) == receiver &&
			callee.ChildByFieldName("attribute").Utf8Text(source) == name
	}
	return false
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

func children(n *sitter.Node, nesting int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		total += score(n.Child(i), nesting)
	}
	return total
}

func score(n *sitter.Node, nesting int) int {
	switch {
	case structures[n.Kind()]:
		return 1 + nesting + children(n, nesting+1)
	case isElifOrElse(n):
		// nesting is already the if statement's plus one
		return 1 + children(n, nesting)
	case clauses[n.Kind()]:
		return 1 + nesting + children(n, nesting)
	case closures[n.Kind()]:
		return children(n, nesting+1)
	case n.Kind() == "boolean_operator":
		ops, inner := booleanSequence(n, nesting)
		return runs(ops) + inner
	default:
		return children(n, nesting)
	}
}

// booleanSequence returns, in source order, the and/or operators of the
// expression n heads (looking through parentheses), and the Complexity of
// its other operands.
func booleanSequence(n *sitter.Node, nesting int) (ops []string, inner int) {
	if n.Kind() == "parenthesized_expression" && n.NamedChildCount() == 1 &&
		n.NamedChild(0).Kind() == "boolean_operator" {
		return booleanSequence(n.NamedChild(0), nesting)
	}
	if n.Kind() != "boolean_operator" {
		return nil, score(n, nesting)
	}
	left, l := booleanSequence(n.ChildByFieldName("left"), nesting)
	right, r := booleanSequence(n.ChildByFieldName("right"), nesting)
	ops = append(append(left, n.ChildByFieldName("operator").Kind()), right...)
	return ops, l + r
}

// runs counts the runs of like operators: and and or counts 2.
func runs(ops []string) int {
	n := 0
	for i, op := range ops {
		if i == 0 || op != ops[i-1] {
			n++
		}
	}
	return n
}

// isElifOrElse reports whether n continues an if statement. They add 1
// without a nesting penalty. The else of a loop or try is part of that
// structure and adds nothing.
func isElifOrElse(n *sitter.Node) bool {
	switch n.Kind() {
	case "elif_clause":
		return true
	case "else_clause":
		return n.Parent().Kind() == "if_statement"
	}
	return false
}
