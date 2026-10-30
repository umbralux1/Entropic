package cli_test

import (
	"fmt"
	"strings"
	"testing"

	"github.com/umbralux1/entropic/internal/testrepo"
)

func TestFrequentlyChangedComplexFileRanksAboveUnchangedComplexFile(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/cold.go", calcGo).
		Write("calc/hot.go", hotGo(0)).
		CommitDaysAgo(200, "add cold and hot").
		Write("calc/hot.go", hotGo(1)).CommitDaysAgo(20, "tweak hot").
		Write("calc/hot.go", hotGo(2)).CommitDaysAgo(10, "tweak hot again")

	r := repo.Run("--base", "none").Report(0)

	if len(r.Hotspots) == 0 || r.Hotspots[0].Path != "calc/hot.go" {
		t.Fatalf("want calc/hot.go as the top Hotspot, got %+v", r.Hotspots)
	}
	// hot.go: Complexity 4 (as calcGo) plus Duplication 15 weighed 0.1: Sign
	// (6 lines) and SumPositive (9) are clones of cold.go's. Changed in 2
	// commits inside the window.
	if h := r.Hotspots[0]; h.Churn != 2 || h.WeightedTotal != 5.5 || h.Score != 11 {
		t.Errorf("calc/hot.go: want churn 2, weighted total 5.5, score 11; got %+v", h)
	}
	// hot.go has 20 lines: calcGo's 18, a blank and the constant.
	if h := r.Hotspots[0]; h.Kind != "file" || h.Name != "calc/hot.go" || h.StartLine != 1 || h.EndLine != 20 {
		t.Errorf("calc/hot.go: want the Hotspot to reference file Unit calc/hot.go, lines 1-20; got %+v", h)
	}
}

func TestChurnWindowDefaultsTo90Days(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/hot.go", hotGo(0)).CommitDaysAgo(120, "add hot").
		Write("calc/hot.go", hotGo(1)).CommitDaysAgo(95, "outside the window").
		Write("calc/hot.go", hotGo(2)).CommitDaysAgo(85, "inside the window")

	r := repo.Run("--base", "none").Report(0)

	if r.ChurnWindow != 90 {
		t.Errorf("churnWindowDays: want 90, got %d", r.ChurnWindow)
	}
	if len(r.Hotspots) != 1 || r.Hotspots[0].Churn != 1 {
		t.Errorf("want calc/hot.go with churn 1 (only the commit 85 days ago), got %+v", r.Hotspots)
	}
}

func TestConfiguredChurnWindowLimitsHotspots(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/stale.go", calcGo).
		Write("calc/hot.go", hotGo(0)).CommitDaysAgo(30, "add stale and hot").
		Write("calc/hot.go", hotGo(1)).CommitDaysAgo(3, "tweak hot").
		Write("entropic.toml", "churn_window_days = 7\n")

	r := repo.Run("--base", "none").Report(0)

	if r.ChurnWindow != 7 {
		t.Errorf("churnWindowDays: want 7, got %d", r.ChurnWindow)
	}
	// stale.go changed only 30 days ago, outside the 7-day window.
	if len(r.Hotspots) != 1 || r.Hotspots[0].Path != "calc/hot.go" || r.Hotspots[0].Churn != 1 {
		t.Errorf("want only calc/hot.go with churn 1, got %+v", r.Hotspots)
	}
}

func TestNoHistoryInWindowMeansNoHotspots(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).CommitDaysAgo(200, "add calc")

	res := repo.Run("--base", "none")
	r := res.Report(0)

	if !strings.Contains(res.Stdout, `"hotspots": []`) || len(r.Hotspots) != 0 {
		t.Errorf("want an empty hotspots list, got:\n%s", res.Stdout)
	}
}

func TestRenamedFileKeepsChurnFromBeforeTheRename(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/old.go", hotGo(0)).CommitDaysAgo(30, "add old").
		Write("calc/old.go", hotGo(1)).CommitDaysAgo(20, "tweak old").
		Move("calc/old.go", "calc/new.go").CommitDaysAgo(10, "rename old to new")

	r := repo.Run("--base", "none").Report(0)

	// Three commits in the window touched this file: add, tweak, rename.
	if len(r.Hotspots) != 1 || r.Hotspots[0].Path != "calc/new.go" || r.Hotspots[0].Churn != 3 {
		t.Errorf("want calc/new.go with churn 3, got %+v", r.Hotspots)
	}
}

func TestDeletedFileIsNotAHotspot(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/gone.go", hotGo(0)).
		Write("calc/calc.go", calcGo).CommitDaysAgo(30, "add gone and calc").
		Write("calc/gone.go", hotGo(1)).CommitDaysAgo(20, "tweak gone").
		Remove("calc/gone.go").CommitDaysAgo(10, "delete gone")

	r := repo.Run("--base", "none").Report(0)

	if len(r.Hotspots) != 1 || r.Hotspots[0].Path != "calc/calc.go" {
		t.Errorf("want only calc/calc.go, got %+v", r.Hotspots)
	}
}

func TestChurnDoesNotFeedEntropyScore(t *testing.T) {
	repo := testrepo.New(t).Write("calc/calc.go", calcGo).CommitDaysAgo(30, "add calc")
	for i := range 3 {
		repo.Write("calc/calc.go", "// draft\n"+calcGo).CommitDaysAgo(20-i, "draft").
			Write("calc/calc.go", calcGo).CommitDaysAgo(19-i, "undo draft")
	}

	r := repo.Run("--base", "none").Report(0)

	if len(r.Hotspots) != 1 || r.Hotspots[0].Churn != 7 {
		t.Fatalf("want calc/calc.go with churn 7, got %+v", r.Hotspots)
	}
	// As in TestSignalSummaryMatchesRepositoryUnit: Complexity 4 per 16 lines = 25.
	if _, ok := r.Signals["churn"]; r.EntropyScore.Value != 25 || ok {
		t.Errorf("want Entropy Score 25 from the complexity Signal alone, no churn Signal; got %v from %v",
			r.EntropyScore.Value, r.Signals)
	}
}

func TestHotspotsAreListedAgainstABaseline(t *testing.T) {
	repo := testrepo.New(t).
		Write("calc/hot.go", hotGo(0)).CommitDaysAgo(20, "add hot").
		Write("calc/hot.go", hotGo(1)).CommitDaysAgo(10, "tweak hot").
		Write("calc/hot.go", hotGo(2)) // uncommitted: not history yet

	r := repo.Run().Report(0)

	if r.Baseline == nil || len(r.Hotspots) != 1 || r.Hotspots[0].Churn != 2 {
		t.Errorf("want a Baseline and calc/hot.go with churn 2, got %+v, %+v", r.Baseline, r.Hotspots)
	}
}

// hotGo is calcGo with a trailing constant, so each version is a real change.
func hotGo(version int) string {
	return fmt.Sprintf("%s\nconst version = %d\n", calcGo, version)
}
