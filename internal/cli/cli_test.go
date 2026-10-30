package cli_test

import (
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

	if len(r.Units) != 1 || r.Units[0].Name != "A" {
		t.Errorf("want only unit A, got %+v", r.Units)
	}
}

func TestSnapshotReportsGoFunctionComplexityAsJSON(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", `package calc

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
`).
		Commit("add calc")

	r := repo.Run("--base", "none", "--format", "json").Report(0)

	if r.SchemaVersion != 1 {
		t.Errorf("schemaVersion: want 1, got %d", r.SchemaVersion)
	}
	want := []testrepo.ReportUnit{
		{Kind: "function", Name: "Sign", Path: "calc/calc.go", StartLine: 3, EndLine: 8, Complexity: 1},
		{Kind: "function", Name: "SumPositive", Path: "calc/calc.go", StartLine: 10, EndLine: 18, Complexity: 3},
	}
	if len(r.Units) != len(want) {
		t.Fatalf("want %d units, got %d: %+v", len(want), len(r.Units), r.Units)
	}
	for i, w := range want {
		if r.Units[i] != w {
			t.Errorf("unit %d: want %+v, got %+v", i, w, r.Units[i])
		}
	}
}
