package analysis_test

import (
	"testing"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/language/golang"
)

func complexityOf(t *testing.T, source string) map[string]int {
	t.Helper()
	units, err := analysis.Analyze([]byte(source), golang.Language)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	got := map[string]int{}
	for _, u := range units {
		got[u.Name] = u.Complexity
	}
	return got
}

func TestGoFunctionsAreUnits(t *testing.T) {
	units, err := analysis.Analyze([]byte(`package p

func A() {}

type T struct{}

func (T) B() {
}

type G[K any] struct{}

func (g *G[K]) C() {}
`), golang.Language)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(units) != 3 {
		t.Fatalf("want 3 units, got %d: %+v", len(units), units)
	}
	want := []analysis.Unit{
		{Kind: analysis.Function, Name: "A", StartLine: 3, EndLine: 3, Cyclomatic: 1},
		{Kind: analysis.Function, Name: "T.B", StartLine: 7, EndLine: 8, Cyclomatic: 1},
		{Kind: analysis.Function, Name: "G.C", StartLine: 12, EndLine: 12, Cyclomatic: 1},
	}
	for i, w := range want {
		if units[i] != w {
			t.Errorf("unit %d: want %+v, got %+v", i, w, units[i])
		}
	}
}

func TestGoComplexityCountsBranchesAndLoops(t *testing.T) {
	got := complexityOf(t, `package p

func flat(x int) int {
	if x > 0 {
		return 1
	}
	for i := 0; i < x; i++ {
	}
	return 0
}
`)
	if got["flat"] != 2 {
		t.Errorf("flat: want 2, got %d", got["flat"])
	}
}

func TestGoComplexityPenalisesNesting(t *testing.T) {
	got := complexityOf(t, `package p

func nested(xs []int) {
	for _, x := range xs { // +1
		if x > 0 { // +2 (nesting 1)
			for x > 10 { // +3 (nesting 2)
				x--
			}
		}
	}
}
`)
	if got["nested"] != 6 {
		t.Errorf("nested: want 6, got %d", got["nested"])
	}
}

// Campbell, "Cognitive Complexity" (2017), getWords: a switch counts once,
// however many cases it has. Expected 1.
func TestGoComplexitySwitchCountsOnce(t *testing.T) {
	got := complexityOf(t, `package p

func getWords(number int) string {
	switch number { // +1
	case 1:
		return "one"
	case 2:
		return "a couple"
	case 3:
		return "a few"
	default:
		return "lots"
	}
}

func kinds(v any, c chan int) {
	switch v.(type) { // +1
	case int:
	case string:
	}
	select { // +1
	case <-c:
	default:
	}
}
`)
	if got["getWords"] != 1 {
		t.Errorf("getWords: want 1, got %d", got["getWords"])
	}
	if got["kinds"] != 2 {
		t.Errorf("kinds: want 2, got %d", got["kinds"])
	}
}

// else if and else add one each with no nesting penalty, but what they
// contain is nested. Hand count: 5.
func TestGoComplexityElseIfAndElseHaveNoNestingPenalty(t *testing.T) {
	got := complexityOf(t, `package p

func grade(x int) string {
	if x > 90 { // +1
		return "a"
	} else if x > 80 { // +1
		return "b"
	} else { // +1
		for x > 0 { // +2 (nesting 1)
			x--
		}
		return "c"
	}
}
`)
	if got["grade"] != 5 {
		t.Errorf("grade: want 5, got %d", got["grade"])
	}
}

// Campbell, "Cognitive Complexity" (2017), sumOfPrimes: a labelled continue
// adds 1. Expected 7.
func TestGoComplexityLabelledJumpsIncrement(t *testing.T) {
	got := complexityOf(t, `package p

func sumOfPrimes(max int) int {
	total := 0
OUT:
	for i := 1; i <= max; i++ { // +1
		for j := 2; j < i; j++ { // +2
			if i%j == 0 { // +3
				continue OUT // +1
			}
		}
		total += i
	}
	return total
}

func jumps(xs []int) {
	for range xs { // +1
		break // +0: unlabelled
	}
L:
	for range xs { // +1
		break L // +1
	}
	goto L // +1
}
`)
	if got["sumOfPrimes"] != 7 {
		t.Errorf("sumOfPrimes: want 7, got %d", got["sumOfPrimes"])
	}
	if got["jumps"] != 4 {
		t.Errorf("jumps: want 4, got %d", got["jumps"])
	}
}

// Campbell, "Cognitive Complexity" (2017): each sequence of like boolean
// operators adds 1, with no nesting penalty; other binary operators add none.
func TestGoComplexityCountsSequencesOfBooleanOperators(t *testing.T) {
	got := complexityOf(t, `package p

func mixed(a, b, c, d, e, f bool) {
	if a && b && c || d || e && f { // +1 if, +3 for &&, ||, &&
	}
}

func assigned(a, b, c bool, x int) bool {
	ok := a && b && c // +1
	return ok || x > 0 && x < 10 // +2
}
`)
	if got["mixed"] != 4 {
		t.Errorf("mixed: want 4, got %d", got["mixed"])
	}
	if got["assigned"] != 3 {
		t.Errorf("assigned: want 3, got %d", got["assigned"])
	}
}

// Campbell, "Cognitive Complexity" (2017), myMethod2: a closure adds nothing
// itself but nests what is inside it. Expected 2.
func TestGoComplexityClosuresIncreaseNesting(t *testing.T) {
	got := complexityOf(t, `package p

func myMethod2(condition1 bool) {
	r := func() { // +0 (but nesting level is now 1)
		if condition1 { // +2 (nesting 1)
		}
	}
	r()
}
`)
	if got["myMethod2"] != 2 {
		t.Errorf("myMethod2: want 2, got %d", got["myMethod2"])
	}
}

// Campbell, "Cognitive Complexity" (2017): recursion adds 1 for each function
// in the recursion cycle, however many recursive calls it makes.
func TestGoComplexityRecursionIncrements(t *testing.T) {
	got := complexityOf(t, `package p

func fact(n int) int {
	if n <= 1 { // +1
		return 1
	}
	return n * fact(n-1) // +1
}

type T struct{}

func (t *T) walk(n int) {
	if n > 0 { // +1
		t.walk(n - 1) // +1
		t.walk(n - 2) // +0: already in the cycle
	}
}

func delegate() {
	other.delegate() // +0: another package's function
}
`)
	want := map[string]int{"fact": 2, "T.walk": 2, "delegate": 0}
	for name, w := range want {
		if got[name] != w {
			t.Errorf("%s: want %d, got %d", name, w, got[name])
		}
	}
}

// McCabe's cyclomatic complexity: 1 plus each if, loop, non-default case
// and boolean operator.
func TestGoCyclomaticCountsDecisionPoints(t *testing.T) {
	units, err := analysis.Analyze([]byte(`package p

func paths(a, b bool, c chan int, n int) {
	if a && b || !a { // +1 if, +2 operators
	} else if b { // +1
	} else { // +0
	}
	for range c { // +1
	}
	select {
	case <-c: // +1
	case c <- n: // +1
	default: // +0
	}
	f := func() bool { return a || b } // +1
	_ = f
}
`), golang.Language)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if got := units[0].Cyclomatic; got != 9 {
		t.Errorf("paths: want cyclomatic 9, got %d", got)
	}
}
