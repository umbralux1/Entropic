package analysis

import sitter "github.com/tree-sitter/go-tree-sitter"

// ComplexityRules tell the Complexity and cyclomatic walkers which syntax
// nodes of a language count. The walkers are language-agnostic; every node
// kind and field name below comes from the language's adapter.
type ComplexityRules struct {
	// Structures break linear flow: each adds 1 plus the current nesting
	// depth to Complexity and nests what is inside it.
	Structures map[string]bool
	// Branching are structures whose "alternative" children (else if, elif,
	// else) each add 1 without a nesting penalty and nest what they contain.
	// An alternative that is itself Branching (Go's else if) continues the
	// chain at the same nesting.
	Branching map[string]bool
	// Clauses add 1 plus the current nesting depth, but do not nest each
	// other: a comprehension's loops and filters read as one flat pipeline.
	Clauses map[string]bool
	// Closures add nothing themselves but nest what is inside them.
	Closures map[string]bool
	// Jumps add 1, without a nesting penalty, when they name a label.
	Jumps map[string]bool
	// BooleanExpressions are binary expression kinds whose "operator" field
	// may be one of BooleanOperators. Each run of like operators in a
	// sequence adds 1 to Complexity; each operator adds 1 to cyclomatic
	// complexity.
	BooleanExpressions map[string]bool
	BooleanOperators   map[string]bool
	// Parentheses wrap one expression; a boolean sequence is read through them.
	Parentheses map[string]bool
	// Calls are call expression kinds, the callee in their "function" field.
	// A function calling itself adds 1 to Complexity.
	Calls map[string]bool
	// Identifier is the node kind of a plain name.
	Identifier string
	// MemberAccess is how a call through a receiver (recv.method) is written.
	MemberAccess MemberAccess
	// Receiver is the name a method's receiver (or self) is bound to, or ""
	// for a plain function.
	Receiver func(fn *sitter.Node, source []byte) string
	// Decisions each add one path to cyclomatic complexity.
	Decisions map[string]bool
	// IsDefaultCase, if set, reports a Decision that matches whatever is
	// left, like a switch's default, and so adds no path.
	IsDefaultCase func(n *sitter.Node) bool
}

// MemberAccess is the node kind of an object.member expression and the
// field names of its two parts.
type MemberAccess struct {
	Kind, Object, Member string
}

// complexity follows G. Ann Campbell's cognitive complexity (SonarSource,
// 2017) for the function Unit formed by fn.
func complexity(fn *sitter.Node, source []byte, r ComplexityRules) int {
	total := r.children(fn, 0)
	if r.callsItself(fn, fn, source) {
		total++
	}
	return total
}

// cyclomatic is McCabe's cyclomatic complexity of fn: 1 plus each decision
// and each boolean operator.
func cyclomatic(fn *sitter.Node, r ComplexityRules) int {
	return 1 + r.countDecisions(fn)
}

func (r ComplexityRules) children(n *sitter.Node, nesting int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		total += r.score(n.Child(i), nesting)
	}
	return total
}

func (r ComplexityRules) score(n *sitter.Node, nesting int) int {
	kind := n.Kind()
	switch {
	case r.Structures[kind]:
		return 1 + nesting + r.structure(n, nesting)
	case r.Clauses[kind]:
		return 1 + nesting + r.children(n, nesting)
	case r.Closures[kind]:
		return r.children(n, nesting+1)
	case r.isBoolean(n):
		ops, inner := r.booleanSequence(n, nesting)
		return runs(ops) + inner
	case r.Jumps[kind] && n.NamedChildCount() > 0:
		return 1
	default:
		return r.children(n, nesting)
	}
}

// structure scores a structure's parts after its own increment: they are
// nested one deeper, except the alternatives of a Branching structure.
func (r ComplexityRules) structure(n *sitter.Node, nesting int) int {
	total := 0
	for i := uint(0); i < n.ChildCount(); i++ {
		child := n.Child(i)
		if r.Branching[n.Kind()] && n.FieldNameForChild(uint32(i)) == "alternative" {
			total += 1 + r.alternative(child, nesting)
			continue
		}
		total += r.score(child, nesting+1)
	}
	return total
}

// alternative scores an else-branch after its own increment.
func (r ComplexityRules) alternative(n *sitter.Node, nesting int) int {
	if r.Branching[n.Kind()] {
		return r.structure(n, nesting)
	}
	return r.children(n, nesting+1)
}

// isBoolean reports whether n is a binary expression with a boolean operator.
func (r ComplexityRules) isBoolean(n *sitter.Node) bool {
	if !r.BooleanExpressions[n.Kind()] {
		return false
	}
	op := n.ChildByFieldName("operator")
	return op != nil && r.BooleanOperators[op.Kind()]
}

// booleanSequence returns, in source order, the boolean operators of the
// expression n heads (looking through parentheses), and the Complexity of
// its other operands.
func (r ComplexityRules) booleanSequence(n *sitter.Node, nesting int) (ops []string, inner int) {
	if r.Parentheses[n.Kind()] && n.NamedChildCount() == 1 && r.isBoolean(n.NamedChild(0)) {
		return r.booleanSequence(n.NamedChild(0), nesting)
	}
	if !r.isBoolean(n) {
		return nil, r.score(n, nesting)
	}
	left, l := r.booleanSequence(n.ChildByFieldName("left"), nesting)
	right, rr := r.booleanSequence(n.ChildByFieldName("right"), nesting)
	ops = append(append(left, n.ChildByFieldName("operator").Kind()), right...)
	return ops, l + rr
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

// callsItself reports whether n, inside function fn, calls fn directly: by
// name for a plain function, through its receiver for a method.
func (r ComplexityRules) callsItself(n, fn *sitter.Node, source []byte) bool {
	if r.Calls[n.Kind()] && r.isSelf(n.ChildByFieldName("function"), fn, source) {
		return true
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		if r.callsItself(n.NamedChild(i), fn, source) {
			return true
		}
	}
	return false
}

// isSelf reports whether callee names function fn.
func (r ComplexityRules) isSelf(callee, fn *sitter.Node, source []byte) bool {
	if callee == nil {
		return false
	}
	name := fn.ChildByFieldName("name").Utf8Text(source)
	receiver := r.Receiver(fn, source)
	switch callee.Kind() {
	case r.Identifier:
		return receiver == "" && callee.Utf8Text(source) == name
	case r.MemberAccess.Kind:
		object := callee.ChildByFieldName(r.MemberAccess.Object)
		return receiver != "" && object != nil && object.Kind() == r.Identifier &&
			object.Utf8Text(source) == receiver &&
			callee.ChildByFieldName(r.MemberAccess.Member).Utf8Text(source) == name
	}
	return false
}

func (r ComplexityRules) countDecisions(n *sitter.Node) int {
	total := 0
	isDefault := r.IsDefaultCase != nil && r.IsDefaultCase(n)
	if (r.Decisions[n.Kind()] && !isDefault) || r.isBoolean(n) {
		total++
	}
	for i := uint(0); i < n.NamedChildCount(); i++ {
		total += r.countDecisions(n.NamedChild(i))
	}
	return total
}
