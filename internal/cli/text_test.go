package cli_test

import (
	"strings"
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

func TestTextSnapshotShowsScoreSignalsAndUnitLocations(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")

	out := repo.Run("--base", "none", "--format", "text").Text(0)

	// calcGo: Complexity 4 over 16 non-blank lines = Entropy Score 25.
	for _, want := range []string{
		"Entropy Score", "25",
		"Complexity", "4",
		"Sign", "calc/calc.go:3-8",
		"SumPositive", "calc/calc.go:10-18",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("text report lacks %q:\n%s", want, out)
		}
	}
}

func TestTextListsFunctionsBeforeTheirClasses(t *testing.T) {
	repo := testrepo.New(t).Write("cart.py", `class Cart:
    def total(self, items):
        return sum(items)
`).Commit("add cart")

	out := repo.Run("--base", "none", "--format", "text").Text(0)

	method, class := strings.Index(out, "function "), strings.Index(out, "class ")
	if method < 0 || class < 0 || class < method {
		t.Errorf("want function Cart.total listed before class Cart:\n%s", out)
	}
}

func TestTextListsUnitsWithLargestDeltaFirst(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc").
		Write("calc/calc.go", signNested). // Sign: Complexity 1 -> 3, Delta +2
		Write("calc/grid.go", `package calc

func Grid(rows [][]int) {
	for range rows {
	}
}
`) // Grid: added, Complexity 1, Delta +1

	out := repo.Run("--base", "HEAD", "--format", "text").Text(0)

	order := []string{"Sign", "Grid", "SumPositive"}
	last := -1
	for _, name := range order {
		i := strings.Index(out, name)
		if i < 0 {
			t.Fatalf("text report lacks %q:\n%s", name, out)
		}
		if i < last {
			t.Errorf("want functions ordered %v by Delta, largest first:\n%s", order, out)
		}
		last = i
	}
	for _, want := range []string{"HEAD", "+2", "+1", "added", "calc/grid.go:3-6"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report lacks %q:\n%s", want, out)
		}
	}
}
