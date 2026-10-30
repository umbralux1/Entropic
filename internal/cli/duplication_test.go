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
		{Path: "calc/calc.go", StartLine: 10, EndLine: 18, Original: true},
		{Path: "stats/stats.go", StartLine: 3, EndLine: 11},
	}}}
	if !reflect.DeepEqual(r.CloneGroups, want) {
		t.Errorf("clone groups: want %+v, got %+v", want, r.CloneGroups)
	}
}

func TestCopiedPythonFunctionWithRenamedIdentifiersIsAClone(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.py", `def sign(x):
    if x > 0:
        return 1
    return 0


def sum_positive(xs):
    total = 0
    for x in xs:
        if x > 0:
            total += x
    return total
`). // sum_positive: lines 7-12
		Write("stats/stats.py", `def count_above(values):
    n = 10
    for v in values:
        if v > 3:
            n += v
    return n
`). // sum_positive renamed, literals changed: lines 1-6
		Commit("copy sum_positive")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	want := []testrepo.ReportCloneGroup{{Locations: []testrepo.ReportLocation{
		{Path: "calc/calc.py", StartLine: 7, EndLine: 12, Original: true},
		{Path: "stats/stats.py", StartLine: 1, EndLine: 6},
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
		{Path: "calc/calc.go", StartLine: 10, EndLine: 18, Original: true},
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

	// Duplication counts the non-blank lines inside additional copies only:
	// SumPositive was there at the Baseline, so it is the original and adds
	// nothing; CountAbove adds its 9 lines.
	if d := r.Signals["duplication"]; d.Value != 9 || d.Delta != 9 {
		t.Errorf("duplication Signal: want 9 (+9), got %+v", d)
	}
	cases := []struct {
		kind, name, status string
		duplication        int
		delta              testrepo.ReportDelta
	}{
		{"function", "SumPositive", "unchanged", 0, testrepo.ReportDelta{}},
		{"function", "Sign", "unchanged", 0, testrepo.ReportDelta{}},
		{"function", "CountAbove", "added", 9, testrepo.ReportDelta{Duplication: 9, Complexity: 3, Lines: 9}},
		{"file", "calc/calc.go", "unchanged", 0, testrepo.ReportDelta{}},
		{"module", "stats", "added", 9, testrepo.ReportDelta{Duplication: 9, Complexity: 3, Lines: 10}},
		{"repository", "", "changed", 9, testrepo.ReportDelta{Duplication: 9, Complexity: 3, Lines: 10}},
	}
	for _, c := range cases {
		u := r.Unit(t, c.kind, c.name)
		if u.Status != c.status || u.Duplication != c.duplication || u.Delta != c.delta {
			t.Errorf("%s %q: want %s, duplication %d, Delta %+v; got %+v",
				c.kind, c.name, c.status, c.duplication, c.delta, u)
		}
	}
}

// twelveLines is a 12-line function, every line non-blank, in package z.
const twelveLines = `package z

func Tally(xs []int) (int, int) {
	pos, neg := 0, 0
	_ = xs
	for _, x := range xs {
		if x > 0 {
			pos++
		} else {
			neg++
		}
	}
	return pos, neg
}
`

func TestCopyingAFunctionCountsOnlyTheCopy(t *testing.T) {
	// The copy sorts before the original, which still keeps that role
	// because only the original was there at the Baseline.
	repo := testrepo.New(t).Write("z/z.go", twelveLines).Commit("add z").
		// A trailing constant keeps the whole file from being one clone.
		Write("a/a.go", "package a"+twelveLines[len("package z"):]+"\nconst A = 1\n")

	r := repo.Run("--format", "json").Report(0)

	if d := r.Signals["duplication"]; d.Value != 12 || d.Delta != 12 {
		t.Errorf("duplication Signal: want 12 (+12), got %+v", d)
	}
	if f := r.Unit(t, "file", "a/a.go"); f.Duplication != 12 || f.Delta.Duplication != 12 {
		t.Errorf("a/a.go: want the copy's 12 duplicated lines, got %+v", f)
	}
	if f := r.Unit(t, "file", "z/z.go"); f.Status != "unchanged" || f.Duplication != 0 {
		t.Errorf("z/z.go: the untouched original should stay unchanged with no Duplication, got %+v", f)
	}
	want := []testrepo.ReportCloneGroup{{Locations: []testrepo.ReportLocation{
		{Path: "a/a.go", StartLine: 3, EndLine: 14},
		{Path: "z/z.go", StartLine: 3, EndLine: 14, Original: true},
	}}}
	if !reflect.DeepEqual(r.CloneGroups, want) {
		t.Errorf("clone groups: want %+v, got %+v", want, r.CloneGroups)
	}
}

func TestInASnapshotTheFirstLocationIsTheOriginal(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo). // SumPositive: lines 10-18, first by path
		Write("stats/stats.go", countAbove).
		Commit("copy SumPositive")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if f := r.Unit(t, "file", "calc/calc.go"); f.Duplication != 0 {
		t.Errorf("calc/calc.go holds the original: want no Duplication, got %+v", f)
	}
	if f := r.Unit(t, "file", "stats/stats.go"); f.Duplication != 9 {
		t.Errorf("stats/stats.go holds the copy: want Duplication 9, got %+v", f)
	}
	if locs := r.CloneGroups[0].Locations; !locs[0].Original || locs[1].Original {
		t.Errorf("want calc/calc.go marked as the original, got %+v", locs)
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
	// below the default minimum of 6.
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

func TestDuplicateMinimumSizeComesFromConfig(t *testing.T) {
	// SumPositive and its copy CountAbove span 9 lines: below a minimum of 10.
	repo := testrepo.New(t).
		Write("entropic.toml", "duplicate_min_lines = 10\n").
		Write("calc/calc.go", calcGo).
		Write("stats/stats.go", countAbove).
		Commit("copy SumPositive")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if len(r.CloneGroups) != 0 || r.Signals["duplication"].Value != 0 {
		t.Errorf("want no clones under a minimum of 10 lines, got %+v and %+v", r.CloneGroups, r.Signals)
	}
}
