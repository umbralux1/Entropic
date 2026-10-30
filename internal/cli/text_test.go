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

// flat is a 12-line function without control flow: Complexity 0.
var flat = "\nfunc Flat() {\n" + strings.Repeat("\tx := 1\n", 10) + "}\n"

func TestUnitsAreOrderedByWeightedDeltaInJSONAndText(t *testing.T) {
	repo := testrepo.New(t).Write("z/z.go", "package z\n"+flat).Commit("add z").
		// A copy of Flat: Duplication +12, weighing 12 x 0.1 = 1.2.
		Write("c/c.go", "package c\n"+flat+"\nconst C = 1\n").
		// Grid: Complexity +1, weighing 1.
		Write("a/grid.go", "package a\n\nfunc Grid(rows [][]int) {\n\tfor range rows {\n\t}\n}\n")

	r := repo.Run("--base", "HEAD").Report(0)

	var got []string
	for _, u := range r.UnitsOfKind("function") {
		got = append(got, u.Path+":"+u.Name)
	}
	want := []string{"c/c.go:Flat", "a/grid.go:Grid", "z/z.go:Flat"}
	if strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("JSON functions: want %v by weighted Delta, got %v", want, got)
	}

	out := repo.Run("--base", "HEAD", "--format", "text").Text(0)
	last := -1
	for _, loc := range []string{"c/c.go:3-14", "a/grid.go:3-6", "z/z.go:3-14"} {
		i := strings.Index(out, loc)
		if i < last {
			t.Errorf("text: want functions in the order of the JSON, %v:\n%s", want, out)
		}
		last = i
	}
}

func TestTextShowsScatterAgainstItsBudget(t *testing.T) {
	repo := testrepo.New(t)
	for _, m := range []string{"a", "b", "c", "d"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc F() {}\n")
	}
	repo.Commit("add four Modules")
	for _, m := range []string{"a", "b", "c", "d"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc G() {}\n")
	}

	out := repo.Run("--format", "text").Text(1)

	// 4 Modules for 8 changed lines; budget 1 + log2(1.8) = 1.85.
	for _, want := range []string{"Scatter", "4 Modules", "budget 1.85", "over budget"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report lacks %q:\n%s", want, out)
		}
	}
}
