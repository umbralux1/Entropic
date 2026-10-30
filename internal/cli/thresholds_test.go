package cli_test

import (
	"bytes"
	"math"
	"strings"
	"testing"

	"github.com/umbralux1/entropic/internal/cli"
	"github.com/umbralux1/entropic/internal/testrepo"
)

// signDeep is Sign from calcGo nested three levels deeper:
// Complexity 1 -> 1+2+3+4 = 10, so +9 for Sign.
const signDeep = `package calc

func Sign(x int) int {
	if x > 0 {
		if x > 1 {
			if x > 2 {
				if x > 3 {
					return 4
				}
			}
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

func TestComplexityOverThresholdIsAViolationNamingTheFunctionAndExits1(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Write("entropic.toml", "[thresholds]\ncomplexity = 5\n").
		Commit("add calc").
		Write("calc/calc.go", signDeep)

	r := repo.Run().Report(1)

	v := r.Violation(t, "complexity")
	if v.Limit != 5 || v.Value != 9 {
		t.Errorf("complexity violation: want +9 over a limit of 5; got %+v", v)
	}
	if len(v.Units) != 1 || v.Units[0].Kind != "function" || v.Units[0].Name != "Sign" || v.Units[0].Path != "calc/calc.go" {
		t.Errorf("complexity violation: want function Sign in calc/calc.go responsible; got %+v", v.Units)
	}
}

func TestTextListsViolationsWithTheirUnits(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Write("entropic.toml", "[thresholds]\ncomplexity = 5\nentropy_score = 100\n").
		Commit("add calc").
		Write("calc/calc.go", signDeep)

	out := repo.Run("--format", "text").Text(1)

	for _, want := range []string{"Threshold exceeded", "complexity +9 > 5", "function Sign calc/calc.go:3-15"} {
		if !strings.Contains(out, want) {
			t.Errorf("text report lacks %q:\n%s", want, out)
		}
	}
}

func TestChangeWithinThresholdsExits0WithNoViolations(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Write("entropic.toml", "[thresholds]\ncomplexity = 9\nentropy_score = 100\n").
		Commit("add calc").
		Write("calc/calc.go", signDeep) // Sign +9: exactly at the limit

	r := repo.Run().Report(0)

	if r.Violations == nil || len(r.Violations) != 0 {
		t.Errorf("want an empty violations list, got %+v", r.Violations)
	}
}

func TestScatterOverBudgetIsAViolationNamingTheModulesTouched(t *testing.T) {
	repo := testrepo.New(t)
	for _, m := range []string{"d", "a", "c", "b"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc F() {}\n")
	}
	repo.Commit("add four Modules")
	for _, m := range []string{"d", "a", "c", "b"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc G() {}\n") // 1 line deleted, 1 added
	}

	r := repo.Run().Report(1)

	// 4 Modules for 8 changed lines; budget 1 + log2(1.8) = 1.848, so 2.152
	// Modules over it against a default allowance of 0.
	v := r.Violation(t, "scatter")
	if v.Limit != 0 || math.Abs(v.Value-(4-1.8479969065549504)) > 1e-9 {
		t.Errorf("scatter violation: want 2.152 Modules over budget, limit 0; got %+v", v)
	}
	var names []string
	for _, u := range v.Units {
		if u.Kind != "module" {
			t.Errorf("scatter violation: want Module Units, got %+v", u)
		}
		names = append(names, u.Name)
	}
	if strings.Join(names, ",") != "a,b,c,d" {
		t.Errorf("scatter violation: want Modules a,b,c,d responsible; got %v", names)
	}
}

func TestScatterAllowanceIsConfigurable(t *testing.T) {
	repo := testrepo.New(t)
	for _, m := range []string{"a", "b", "c", "d"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc F() {}\n")
	}
	repo.Write("entropic.toml", "[thresholds]\nscatter = 3\n").Commit("add four Modules")
	for _, m := range []string{"a", "b", "c", "d"} {
		repo.Write(m+"/f.go", "package "+m+"\n\nfunc G() {}\n")
	}

	repo.Run().Report(0) // 2.152 Modules over budget, 3 allowed
}

func TestEntropyScoreRiseOverThresholdNamesFunctionsThatGainedComplexity(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Write("entropic.toml", "[thresholds]\ncomplexity = 100\nentropy_score = 5\n").
		Commit("add calc").
		Write("calc/calc.go", signDeep)

	r := repo.Run().Report(1)

	// Before: Complexity 4 per 16 lines = 25. After: Sign 10 + SumPositive 3
	// = 13 per 23 lines = 56.52..., a rise of 31.52...
	v := r.Violation(t, "entropyScore")
	if v.Limit != 5 || math.Abs(v.Value-(1300.0/23-25)) > 1e-9 {
		t.Errorf("entropyScore violation: want a rise of 31.52 over a limit of 5; got %+v", v)
	}
	if len(v.Units) != 1 || v.Units[0].Name != "Sign" {
		t.Errorf("entropyScore violation: want Sign responsible; got %+v", v.Units)
	}
	if len(r.Violations) != 1 {
		t.Errorf("want only the entropyScore violation, got %+v", r.Violations)
	}
}

func TestSnapshotExceedsNoThresholdEvenWhenStrict(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", signDeep).
		Write("entropic.toml", "[thresholds]\ncomplexity = 0\nentropy_score = 0\n").
		Commit("add calc")

	r := repo.Run("--base", "none").Report(0)

	if len(r.Violations) != 0 {
		t.Errorf("a snapshot has no Delta, so no violations; got %+v", r.Violations)
	}
}

func TestOutsideAGitRepositoryExits2(t *testing.T) {
	var out, errOut bytes.Buffer

	code := cli.Run([]string{"--base", "none"}, t.TempDir(), &out, &errOut)

	if code != 2 || out.Len() != 0 || errOut.Len() == 0 {
		t.Errorf("want exit 2, no report and an error; got %d, stdout %q, stderr %q", code, out.String(), errOut.String())
	}
}

func TestSourceThatFailsToParseExits2NamingTheFile(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/calc.go", calcGo).
		Write("calc/broken.go", "package calc\n\nfunc Broken( {\n").
		Commit("add calc")

	res := repo.Run("--base", "none")

	if res.Code != 2 || res.Stdout != "" || !strings.Contains(res.Stderr, "calc/broken.go") {
		t.Errorf("want exit 2 and an error naming calc/broken.go; got %d, stdout %q, stderr %q", res.Code, res.Stdout, res.Stderr)
	}
}
