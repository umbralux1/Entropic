package cli_test

import (
	"reflect"
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

// countAbove is SumPositive from calcGo copied with every identifier renamed
// and the literal 0 in the condition changed to 10. Lines 3-11, all non-blank.
const countAbove = `package stats

func CountAbove(values []int) int {
	n := 0
	for _, v := range values {
		if v > 10 {
			n += v
		}
	}
	return n
}
`

func TestCopiedFunctionWithRenamedIdentifiersIsAClone(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo). // SumPositive: lines 10-18
		Write("stats/stats.go", countAbove).
		Commit("copy SumPositive")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	want := []testrepo.ReportCloneGroup{{Locations: []testrepo.ReportLocation{
		{Path: "calc/calc.go", StartLine: 10, EndLine: 18},
		{Path: "stats/stats.go", StartLine: 3, EndLine: 11},
	}}}
	if !reflect.DeepEqual(r.CloneGroups, want) {
		t.Errorf("clone groups: want %+v, got %+v", want, r.CloneGroups)
	}
}

func TestCloneGroupListsEveryLocation(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo). // SumPositive: lines 10-18
		Write("stats/stats.go", countAbove).
		Write("stats/more.go", "package stats\n\n// Doubled sums twice.\n"+countAbove[len("package stats\n"):]+
			"\nfunc Zero() int { return 0 }\n"). // so that the two stats files differ
		Commit("copy SumPositive twice")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	want := []testrepo.ReportCloneGroup{{Locations: []testrepo.ReportLocation{
		{Path: "calc/calc.go", StartLine: 10, EndLine: 18},
		{Path: "stats/more.go", StartLine: 5, EndLine: 13}, // a comment and a blank line above
		{Path: "stats/stats.go", StartLine: 3, EndLine: 11},
	}}}
	if !reflect.DeepEqual(r.CloneGroups, want) {
		t.Errorf("clone groups: want %+v, got %+v", want, r.CloneGroups)
	}
}

func TestDuplicateInUncommittedEditShowsPositiveDuplicationDelta(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc").
		Write("stats/stats.go", countAbove)

	r := repo.Run("--format", "json").Report(0)

	// Duplication counts the non-blank lines inside clones, at every
	// location: SumPositive 9 + CountAbove 9.
	if d := r.Signals["duplication"]; d.Value != 18 || d.Delta != 18 {
		t.Errorf("duplication Signal: want 18 (+18), got %+v", d)
	}
	cases := []struct {
		kind, name, status string
		duplication        int
		delta              testrepo.ReportDelta
	}{
		{"function", "SumPositive", "changed", 9, testrepo.ReportDelta{Duplication: 9}},
		{"function", "Sign", "unchanged", 0, testrepo.ReportDelta{}},
		{"function", "CountAbove", "added", 9, testrepo.ReportDelta{Duplication: 9, Complexity: 3, Lines: 9}},
		{"file", "calc/calc.go", "changed", 9, testrepo.ReportDelta{Duplication: 9}},
		{"module", "stats", "added", 9, testrepo.ReportDelta{Duplication: 9, Complexity: 3, Lines: 10}},
		{"repository", "", "changed", 18, testrepo.ReportDelta{Duplication: 18, Complexity: 3, Lines: 10}},
	}
	for _, c := range cases {
		u := r.Unit(t, c.kind, c.name)
		if u.Status != c.status || u.Duplication != c.duplication || u.Delta != c.delta {
			t.Errorf("%s %q: want %s, duplication %d, Delta %+v; got %+v",
				c.kind, c.name, c.status, c.duplication, c.delta, u)
		}
	}
}

func TestImportBlocksAreNotClones(t *testing.T) {
	// Both files import four packages: the same shape once the paths are
	// abstracted, over 6 lines, but importing is not duplicated logic.
	repo := testrepo.New(t).
		Write("a/a.go", "package a\n\nimport (\n\t\"fmt\"\n\t\"os\"\n\t\"sort\"\n\t\"strings\"\n)\n").
		Write("b/b.go", "package b\n\nimport (\n\t\"bytes\"\n\t\"io\"\n\t\"path\"\n\t\"time\"\n)\n").
		Commit("add imports")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if len(r.CloneGroups) != 0 {
		t.Errorf("want no clone groups, got %+v", r.CloneGroups)
	}
}

func TestClonesBelowMinimumSizeAreNotReported(t *testing.T) {
	// Inc and Dec are structural clones, but each spans only 4 lines,
	// below the default minimum of 5.
	repo := testrepo.New(t).Write("small.go", `package small

func Inc(x int) int {
	y := x + 1
	return y
}

func Dec(a int) int {
	b := a + 2
	return b
}
`).Commit("add small")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if len(r.CloneGroups) != 0 {
		t.Errorf("want no clone groups, got %+v", r.CloneGroups)
	}
}
