package cli_test

import (
	"math"
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

func TestSnapshotIgnoresFilesDeletedFromWorkingTree(t *testing.T) {
	repo := testrepo.New(t).
		Write("a.go", "package p\n\nfunc A() {}\n").
		Write("b.go", "package p\n\nfunc B() {}\n").
		Commit("add a and b").
		Remove("b.go")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if fns := r.UnitsOfKind("function"); len(fns) != 1 || fns[0].Name != "A" {
		t.Errorf("want only function A, got %+v", fns)
	}
}

func TestUnitValuesRollUpToFileModuleAndRepository(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo). // Sign 1 + SumPositive 3; 16 non-blank lines
		Write("calc/abs.go", `package calc

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
`). // Abs 1; 7 non-blank lines
		Write("main.go", `package main

func main() {
	for {
	}
}
`). // main 1; 5 non-blank lines
		Commit("add code")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	cases := []struct {
		kind, name       string
		complexity, size int
	}{
		{"file", "calc/abs.go", 1, 7},
		{"file", "calc/calc.go", 4, 16},
		{"file", "main.go", 1, 5},
		{"module", "calc", 5, 23},
		{"module", ".", 1, 5},
		{"repository", "", 6, 28},
	}
	for _, c := range cases {
		u := r.Unit(t, c.kind, c.name)
		if u.Complexity != c.complexity || u.Lines != c.size {
			t.Errorf("%s %q: want complexity %d, lines %d; got complexity %d, lines %d",
				c.kind, c.name, c.complexity, c.size, u.Complexity, u.Lines)
		}
	}
	if f := r.Unit(t, "file", "calc/abs.go"); f.Module != "calc" || f.StartLine != 1 || f.EndLine != 8 {
		t.Errorf("calc/abs.go: want module calc, lines 1-8; got %+v", f)
	}
}

func TestSignalSummaryMatchesRepositoryUnit(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if got := r.Signals["complexity"].Value; got != 4 {
		t.Errorf("complexity Signal: want 4, got %v", got)
	}
	// Entropy Score is a density: Complexity 4 per 16 non-blank lines = 25 per 100 lines.
	if r.EntropyScore.Value != 25 {
		t.Errorf("entropyScore: want 25, got %v", r.EntropyScore.Value)
	}
}

func TestEntropyScoreIsNotRounded(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Write("calc/zero.go", `package calc

func Zero() int {
	z := 0
	return z
}
`).Commit("add calc")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	// Complexity 4 per 16+5 = 21 non-blank lines = 19.047619... per 100 lines;
	// a rounded score would hide small Deltas in large repositories.
	if want := 400.0 / 21; math.Abs(r.EntropyScore.Value-want) > 1e-9 {
		t.Errorf("entropyScore: want %v, got %v", want, r.EntropyScore.Value)
	}
}

// Entropy Score is a density, so adding code with Complexity 0 lowers it,
// while adding code denser than the repository average raises it.
func TestAddingFlatCodeDoesNotRaiseEntropyScore(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	before := repo.Run("--base", "none", "--format", "json").Report(0).EntropyScore.Value

	repo.Write("calc/names.go", `package calc

func Name() string {
	return "calc"
}

func Version() string {
	return "1.0"
}
`).Commit("add flat functions")
	after := repo.Run("--base", "none", "--format", "json").Report(0).EntropyScore.Value

	if after > before {
		t.Errorf("entropyScore rose from %v to %v after adding flat code", before, after)
	}
}

func TestAddingNestedCodeRaisesEntropyScore(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	before := repo.Run("--base", "none", "--format", "json").Report(0).EntropyScore.Value

	repo.Write("calc/grid.go", `package calc

func CountPositive(grid [][]int) int {
	n := 0
	for _, row := range grid {
		for _, x := range row {
			if x > 0 {
				if x%2 == 0 {
					n++
				}
			}
		}
	}
	return n
}
`).Commit("add nested function") // complexity 1+2+3+4 = 10 in 14 lines
	after := repo.Run("--base", "none", "--format", "json").Report(0).EntropyScore.Value

	if after <= before {
		t.Errorf("entropyScore did not rise (%v -> %v) after adding nested code", before, after)
	}
}

const calcGo = `package calc

func Sign(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}

func SumPositive(xs []int) int {
	total := 0
	for _, x := range xs {
		if x > 0 {
			total += x
		}
	}
	return total
}
`

func TestSnapshotReportsGoFunctionComplexityAsJSON(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Commit("add calc")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if r.SchemaVersion != 2 {
		t.Errorf("schemaVersion: want 2, got %d", r.SchemaVersion)
	}
	want := []testrepo.ReportUnit{
		{Kind: "function", Name: "Sign", Path: "calc/calc.go", Module: "calc", StartLine: 3, EndLine: 8, Lines: 6, Complexity: 1,
			Informational: testrepo.ReportInformational{Cyclomatic: 2}}, // 1 + if
		{Kind: "function", Name: "SumPositive", Path: "calc/calc.go", Module: "calc", StartLine: 10, EndLine: 18, Lines: 9, Complexity: 3,
			Informational: testrepo.ReportInformational{Cyclomatic: 3}}, // 1 + for + if
	}
	fns := r.UnitsOfKind("function")
	if len(fns) != len(want) {
		t.Fatalf("want %d functions, got %d: %+v", len(want), len(fns), fns)
	}
	for i, w := range want {
		if fns[i] != w {
			t.Errorf("function %d: want %+v, got %+v", i, w, fns[i])
		}
	}
}

func TestCyclomaticComplexityIsInformationalOnly(t *testing.T) {
	repo := testrepo.New(t).Write("words.go", `package words

func Words(n int) string {
	switch n { // Complexity +1; cyclomatic +1 per case
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
`).Commit("add words")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if w := r.Unit(t, "function", "Words"); w.Complexity != 1 || w.Informational.Cyclomatic != 4 {
		t.Errorf("Words: want Complexity 1 and informational cyclomatic 4, got %+v", w)
	}
	if c := r.Signals["complexity"].Value; c != 1 {
		t.Errorf("complexity Signal: want 1, unaffected by cyclomatic; got %v", c)
	}
	for _, u := range r.Units {
		if u.Kind != "function" && u.Informational != (testrepo.ReportInformational{}) {
			t.Errorf("informational values belong to functions only, not rolled up: %+v", u)
		}
	}
}
