package cli_test

import (
	"math"
	"strings"
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

func TestChangeConfinedToOneModuleHasScatterOne(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc").
		Write("calc/calc.go", signNested) // 3 lines added, all in Module calc

	r := repo.Run("--format", "json").Report(0)

	if r.Scatter == nil {
		t.Fatalf("want Scatter against the Baseline, got none")
	}
	if r.Scatter.Modules != 1 || r.Scatter.ChangedLines != 3 {
		t.Errorf("Scatter: want 1 Module, 3 changed lines; got %+v", *r.Scatter)
	}
}

// Provisional Scatter Budget: 1 + log2(1 + changed lines / 10), so a change
// of 10 lines may touch 2 Modules, 30 lines 3, 70 lines 4, 150 lines 5.

func TestSmallChangeAcrossManyModulesExceedsScatterBudget(t *testing.T) {
	repo := testrepo.New(t)
	for _, m := range []string{"a", "b", "c", "d"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc F() {}\n")
	}
	repo.Commit("add four Modules")
	for _, m := range []string{"a", "b", "c", "d"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc G() {}\n") // 1 line deleted, 1 added
	}

	r := repo.Run("--format", "json").Report(1) // over budget: a violation

	// 4 Modules, 8 changed lines: budget 1 + log2(1.8) = 1.8479...
	if s := r.Scatter; s == nil || s.Modules != 4 || s.ChangedLines != 8 ||
		math.Abs(s.Budget-1.8479969065549504) > 1e-9 || !s.OverBudget {
		t.Errorf("Scatter: want 4 Modules over a budget of 1.848 for 8 lines; got %+v", s)
	}
}

func TestScatterReportsFilesTouchedAndShannonEntropyAsInformational(t *testing.T) {
	repo := testrepo.New(t).
		Write("a/x.go", "package a\n\nfunc X() {}\n").
		Write("a/y.go", "package a\n\nfunc Y() {}\n\nfunc W() {}\n").
		Write("b/z.go", "package b\n\nfunc Z() {}\n").
		Commit("start").
		Write("a/x.go", "package a\n\nfunc X2() {}\n").                 // 2 changed lines
		Write("a/y.go", "package a\n\nfunc Y2() {}\n\nfunc W2() {}\n"). // 4 changed lines
		Write("b/z.go", "package b\n\nfunc Z2() {}\n")                  // 2 changed lines

	r := repo.Run("--format", "json").Report(1) // 2 Modules over a budget of 1.85

	// Changed lines per Module: a 6, b 2 of 8, so p = 3/4, 1/4.
	// H = -(3/4 log2 3/4 + 1/4 log2 1/4) = 0.8113 bits; 2^H = 1.7548 Modules.
	s := r.Scatter
	if s == nil {
		t.Fatalf("want Scatter, got none")
	}
	info := s.Informational
	if info.Files != 3 || math.Abs(info.Entropy-0.8112781244591328) > 1e-9 ||
		math.Abs(info.EffectiveModules-1.7547653506033232) > 1e-9 {
		t.Errorf("informational: want 3 files, entropy 0.8113, 2^H 1.7548; got %+v", info)
	}
}

func TestSnapshotHasNoScatter(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).Commit("add calc").Write("calc/calc.go", signNested)

	res := repo.Run("--base", "none", "--format", "json")

	if r := res.Report(0); r.Scatter != nil || strings.Contains(res.Stdout, `"scatter"`) {
		t.Errorf("snapshot should carry no Scatter:\n%s", res.Stdout)
	}
}

func TestScatterEdgeStates(t *testing.T) {
	const f = "package p\n\nfunc F() {}\n" // 3 lines
	cases := []struct {
		name         string
		change       func(*testrepo.Repo)
		modules      int
		changedLines int
		files        int
		exit         int // 1 when over the Scatter Budget
	}{
		{
			name:   "no change touches no Module",
			change: func(r *testrepo.Repo) {},
		},
		{
			name:    "deleted file touches its Module",
			change:  func(r *testrepo.Repo) { r.Remove("b/f.go") }, // 3 lines deleted
			modules: 1, changedLines: 3, files: 1,
		},
		{
			name: "renamed file touches both Modules",
			change: func(r *testrepo.Repo) {
				r.Remove("a/f.go").Write("c/f.go", f) // 3 deleted in a, 3 added in c
			},
			modules: 2, changedLines: 6, files: 2, exit: 1, // budget 1.68
		},
		{
			name: "root Module counts like any other",
			change: func(r *testrepo.Repo) {
				// 1 line deleted and 1 added in each of . and a.
				r.Write("main.go", "package main\n\nfunc main() { _ = 1 }\n").
					Write("a/f.go", "package p\n\nfunc G() {}\n")
			},
			modules: 2, changedLines: 4, files: 2, exit: 1, // budget 1.49
		},
		{
			name:   "non-source files are not part of Scatter",
			change: func(r *testrepo.Repo) { r.Write("docs/notes.md", "one\ntwo\n").Write("b/data.txt", "x\n") },
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := testrepo.New(t).
				Write("main.go", "package main\n\nfunc main() {}\n").
				Write("a/f.go", f).
				Write("b/f.go", f).
				Commit("start")
			c.change(repo)

			r := repo.Run("--base", "HEAD", "--format", "json").Report(c.exit)

			s := r.Scatter
			if s == nil || s.Modules != c.modules || s.ChangedLines != c.changedLines || s.Informational.Files != c.files {
				t.Errorf("Scatter: want %d Modules, %d changed lines, %d files; got %+v",
					c.modules, c.changedLines, c.files, s)
			}
		})
	}
}

func TestScatterLeavesOutExcludedVendoredAndGeneratedFiles(t *testing.T) {
	repo := testrepo.New(t).
		Write("entropic.toml", "exclude = [\"migrations\"]\n").
		Write("calc/calc.go", calcGo).
		Commit("start").
		Write("calc/calc.go", signNested). // 3 lines added in calc: the only counted change
		Write("migrations/m.go", "package migrations\n\nfunc M() {}\n").
		Write("vendor/example.com/lib/lib.go", "package lib\n\nfunc L() {}\n").
		Write("gen/gen.go", "// Code generated by hand; DO NOT EDIT.\n\npackage gen\n\nfunc G() {}\n")

	r := repo.Run("--base", "HEAD", "--format", "json").Report(0)

	if s := r.Scatter; s == nil || s.Modules != 1 || s.ChangedLines != 3 || s.Informational.Files != 1 {
		t.Errorf("Scatter: want only calc (1 Module, 3 lines, 1 file); got %+v", s)
	}
}

func TestScatterOfCommittedRenameAgainstExplicitBase(t *testing.T) {
	repo := testrepo.New(t).Write("a/f.go", "package a\n\nfunc F() {}\n").Commit("start")
	start := repo.Rev("HEAD")
	repo.Remove("a/f.go").Write("b/f.go", "package a\n\nfunc F() {}\n").Commit("move f to b")

	r := repo.Run("--base", start, "--format", "json").Report(1) // over a budget of 1.68

	// Deleted from a (3 lines) and added to b (3 lines).
	if s := r.Scatter; s == nil || s.Modules != 2 || s.ChangedLines != 6 {
		t.Errorf("Scatter: want 2 Modules, 6 changed lines; got %+v", s)
	}
}

// bigFunc is a 60-line Go file in package pkg: 3 header lines, 56
// statements, 1 closing brace.
func bigFunc(pkg string) string {
	return "package " + pkg + "\n\nfunc Big() {\n" + strings.Repeat("\t_ = 0\n", 56) + "}\n"
}

func TestLargeChangeAcrossFewModulesStaysWithinScatterBudget(t *testing.T) {
	repo := testrepo.New(t).Write("main.go", "package main\n\nfunc main() {}\n").Commit("start").
		Write("a/big.go", bigFunc("a")).
		Write("b/big.go", bigFunc("b")).
		Write("c/big.go", bigFunc("c")) // new, untracked files: 3 x 60 added lines

	r := repo.Run("--format", "json").Report(0)

	// 3 Modules, 180 changed lines: budget 1 + log2(19) = 5.2479...
	if s := r.Scatter; s == nil || s.Modules != 3 || s.ChangedLines != 180 ||
		math.Abs(s.Budget-5.247927513443585) > 1e-9 || s.OverBudget {
		t.Errorf("Scatter: want 3 Modules within a budget of 5.248 for 180 lines; got %+v", s)
	}
}
