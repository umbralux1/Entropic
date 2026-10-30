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
`), golang.Language)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}
	if len(units) != 2 {
		t.Fatalf("want 2 units, got %d: %+v", len(units), units)
	}
	want := []analysis.Unit{
		{Kind: analysis.Function, Name: "A", StartLine: 3, EndLine: 3},
		{Kind: analysis.Function, Name: "B", StartLine: 7, EndLine: 8},
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
