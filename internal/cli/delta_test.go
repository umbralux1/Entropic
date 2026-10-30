package cli_test

import (
	"math"
	"strings"
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

// signNested is Sign from calcGo with an extra nested if:
// Complexity 1 -> 3, non-blank lines 6 -> 9.
const signNested = `package calc

func Sign(x int) int {
	if x > 0 {
		if x > 100 {
			return 2
		}
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

func TestDefaultBaseForUncommittedEditIsHEAD(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	head := repo.Rev("HEAD")
	repo.Write("calc/calc.go", signNested)

	r := repo.Run("--format", "json").Report(0)

	if r.Baseline == nil || r.Baseline.Ref != "HEAD" || r.Baseline.Commit != head {
		t.Errorf("baseline: want HEAD at %s, got %+v", head, r.Baseline)
	}
	if d := r.Unit(t, "repository", "").Delta; d != (testrepo.ReportDelta{Complexity: 2, Lines: 3}) {
		t.Errorf("repository Delta: want complexity +2, lines +3; got %+v", d)
	}
}

func TestDefaultBaseForCleanBranchIsMergeBaseWithMain(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	forkPoint := repo.Rev("HEAD")
	repo.Branch("feature").Write("calc/calc.go", signNested).Commit("nest Sign")
	repo.Switch("main").Write("calc/grid.go", `package calc

func Grid(rows [][]int) {
	for range rows {
		for range rows {
		}
	}
}
`).Commit("unrelated work on main")
	repo.Switch("feature")

	r := repo.Run("--format", "json").Report(0)

	if r.Baseline == nil || r.Baseline.Commit != forkPoint {
		t.Errorf("baseline: want merge-base %s, got %+v", forkPoint, r.Baseline)
	}
	if d := r.Unit(t, "repository", "").Delta; d != (testrepo.ReportDelta{Complexity: 2, Lines: 3}) {
		t.Errorf("repository Delta: want only the branch's +2 complexity, +3 lines; got %+v", d)
	}
	for _, u := range r.Units {
		if u.Path == "calc/grid.go" {
			t.Errorf("work merged into main after the fork leaked into the report: %+v", u)
		}
	}
}

func TestUnitsAddedOrRemovedSinceBaselineAreMarked(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Write("calc/abs.go", `package calc

func Abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
`). // Abs: Complexity 1, 6 non-blank lines
		Commit("add calc").
		Remove("calc/abs.go").
		Write("calc/loop.go", `package calc

func Loop() {
	for {
	}
}
`) // Loop: Complexity 1, 4 non-blank lines

	r := repo.Run("--format", "json").Report(0)

	cases := []struct {
		kind, name, status string
		delta              testrepo.ReportDelta
	}{
		{"function", "Abs", "removed", testrepo.ReportDelta{Complexity: -1, Lines: -6}},
		{"file", "calc/abs.go", "removed", testrepo.ReportDelta{Complexity: -1, Lines: -7}}, // package line + Abs
		{"function", "Loop", "added", testrepo.ReportDelta{Complexity: 1, Lines: 4}},
		{"file", "calc/loop.go", "added", testrepo.ReportDelta{Complexity: 1, Lines: 5}}, // package line + Loop
	}
	for _, c := range cases {
		u := r.Unit(t, c.kind, c.name)
		if u.Status != c.status || u.Delta != c.delta {
			t.Errorf("%s %s: want %s with Delta %+v; got %+v", c.kind, c.name, c.status, c.delta, u)
		}
	}
	if abs := r.Unit(t, "function", "Abs"); abs.Path != "calc/abs.go" || abs.StartLine != 3 {
		t.Errorf("removed Abs should keep its Baseline location; got %+v", abs)
	}
}

func TestSnapshotHasNoBaselineOrDelta(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc").Write("calc/calc.go", signNested)

	res := repo.Run("--base", "none", "--format", "json")

	if r := res.Report(0); r.Baseline != nil {
		t.Errorf("baseline: want none, got %+v", r.Baseline)
	}
	if strings.Contains(res.Stdout, `"delta"`) || strings.Contains(res.Stdout, `"status"`) {
		t.Errorf("snapshot should carry no Delta or status:\n%s", res.Stdout)
	}
}

func TestExplicitBaseReportsDeltaPerUnit(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	head := repo.Rev("HEAD")
	repo.Write("calc/calc.go", signNested)

	r := repo.Run("--base", "HEAD", "--format", "json").Report(0)

	if r.Baseline == nil || r.Baseline.Ref != "HEAD" || r.Baseline.Commit != head {
		t.Errorf("baseline: want HEAD at %s, got %+v", head, r.Baseline)
	}
	sign := r.Unit(t, "function", "Sign")
	if sign.Status != "changed" || sign.Delta != (testrepo.ReportDelta{Complexity: 2, Lines: 3}) {
		t.Errorf("Sign: want changed with Delta complexity +2, lines +3; got %+v", sign)
	}
	sum := r.Unit(t, "function", "SumPositive")
	if sum.Status != "unchanged" || sum.Delta != (testrepo.ReportDelta{}) {
		t.Errorf("SumPositive: want unchanged with zero Delta; got %+v", sum)
	}
	if file := r.Unit(t, "file", "calc/calc.go"); file.Delta != (testrepo.ReportDelta{Complexity: 2, Lines: 3}) {
		t.Errorf("calc/calc.go: want Delta complexity +2, lines +3; got %+v", file)
	}
}

func TestEntropyScoreAndSignalsCarryDelta(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc").Write("calc/calc.go", signNested)

	r := repo.Run("--base", "HEAD", "--format", "json").Report(0)

	if c := r.Signals["complexity"]; c.Value != 6 || c.Delta != 2 {
		t.Errorf("complexity Signal: want 6 (+2), got %+v", c)
	}
	// Baseline: 4 per 16 lines = 25. Now: 6 per 19 lines = 31.578947...
	if want := 600.0/19 - 25; math.Abs(r.EntropyScore.Delta-want) > 1e-9 {
		t.Errorf("entropyScore Delta: want %v, got %+v", want, r.EntropyScore)
	}
}

func TestMeasuringABaselineLeavesTheWorkingTreeUntouched(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	repo.Branch("feature").Write("calc/calc.go", signNested).Commit("nest Sign")
	repo.Write("calc/calc.go", calcGo).Write("calc/new.go", "package calc\n\nfunc New() {}\n")
	before := repo.State()

	repo.Run("--base", "main", "--format", "json").Report(0)
	repo.Run("--format", "json").Report(0)

	if after := repo.State(); after != before {
		t.Errorf("working tree changed:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

func TestDefaultBaseInCICheckoutUsesOriginMain(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc")
	forkPoint := repo.Rev("HEAD")
	repo.Branch("feature").Write("calc/calc.go", signNested).Commit("nest Sign")
	ci := repo.CICheckout("feature")

	r := ci.Run("--format", "json").Report(0)

	if r.Baseline == nil || r.Baseline.Commit != forkPoint {
		t.Errorf("baseline: want merge-base with origin/main %s, got %+v", forkPoint, r.Baseline)
	}
}

func TestDefaultBaseInRepoWithoutCommitsIsASnapshot(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo)

	r := repo.Run("--format", "json").Report(0)

	if r.Baseline != nil {
		t.Errorf("baseline: want none before the first commit, got %+v", r.Baseline)
	}
	if got := r.Unit(t, "repository", "").Complexity; got != 4 {
		t.Errorf("repository Complexity: want 4, got %d", got)
	}
}

func TestDeletingOneOfTwoSameNamedMethodsMarksOnlyThatMethodRemoved(t *testing.T) {
	repo := testrepo.New(t).Write("shapes/shapes.go", `package shapes

type A struct{}

func (A) M() int {
	return 1
}

type B struct{}

func (*B) M() int {
	if true {
		return 2
	}
	return 0
}
`).Commit("add shapes").Write("shapes/shapes.go", `package shapes

type A struct{}

type B struct{}

func (*B) M() int {
	if true {
		return 2
	}
	return 0
}
`)

	r := repo.Run("--format", "json").Report(0)

	if a := r.Unit(t, "function", "A.M"); a.Status != "removed" {
		t.Errorf("A.M: want removed, got %+v", a)
	}
	if b := r.Unit(t, "function", "B.M"); b.Status != "unchanged" {
		t.Errorf("B.M: want unchanged, got %+v", b)
	}
	if fns := r.UnitsOfKind("function"); len(fns) != 2 {
		t.Errorf("want exactly A.M and B.M, got %+v", fns)
	}
}
