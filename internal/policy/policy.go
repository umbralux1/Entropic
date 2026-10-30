// Package policy evaluates thresholds against the Delta of a change and
// names the Units responsible for each violation.
package policy

import (
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/config"
	"github.com/umbralux1/entropic/internal/scatter"
	"github.com/umbralux1/entropic/internal/scoring"
)

// Evaluate returns the thresholds r's production code exceeds. A snapshot
// has no Delta, so it exceeds none.
func Evaluate(r scoring.Report, t config.Thresholds) []scoring.Violation {
	violations := []scoring.Violation{}
	if r.Baseline == nil {
		return violations
	}
	if v, ok := complexity(r.Units, t.Complexity); ok {
		violations = append(violations, v)
	}
	if v, ok := scatterOverBudget(r.Scatter, t.Scatter); ok {
		violations = append(violations, v)
	}
	if v, ok := entropyScore(r.Group, t.EntropyScore); ok {
		violations = append(violations, v)
	}
	return violations
}

// complexity is violated by every function gaining more than limit.
func complexity(units []scoring.Unit, limit float64) (scoring.Violation, bool) {
	over := gainers(units, limit)
	if len(over) == 0 {
		return scoring.Violation{}, false
	}
	return scoring.Violation{
		Threshold: scoring.ComplexityThreshold,
		Limit:     limit,
		Value:     float64(over[0].Delta.Values[scoring.Complexity]),
		Units:     refs(over),
	}, true
}

// scatterOverBudget is violated when a change touches more than limit Modules beyond
// its Scatter Budget. Every Module it touches is responsible.
func scatterOverBudget(s *scatter.Scatter, limit float64) (scoring.Violation, bool) {
	if s == nil {
		return scoring.Violation{}, false
	}
	excess := float64(s.Modules) - s.Budget
	if excess <= limit {
		return scoring.Violation{}, false
	}
	units := make([]scoring.UnitRef, len(s.TouchedModules))
	for i, m := range s.TouchedModules {
		units[i] = scoring.UnitRef{Kind: analysis.Module, Name: m, Path: m}
	}
	return scoring.Violation{Threshold: scoring.ScatterThreshold, Limit: limit, Value: excess, Units: units}, true
}

// entropyScore is violated when the Entropy Score rises by more than limit.
// The functions that gained Complexity are responsible; if none did, the
// rise comes from code removed around them, so the changed files are.
func entropyScore(g scoring.Group, limit float64) (scoring.Violation, bool) {
	rise := *g.EntropyScore.Delta
	if rise <= limit {
		return scoring.Violation{}, false
	}
	responsible := gainers(g.Units, 0)
	if len(responsible) == 0 {
		for _, u := range g.Units {
			if u.Kind == analysis.File && u.Status != scoring.Unchanged {
				responsible = append(responsible, u)
			}
		}
	}
	return scoring.Violation{
		Threshold: scoring.EntropyScoreThreshold,
		Limit:     limit,
		Value:     rise,
		Units:     refs(responsible),
	}, true
}

// gainers returns the functions that gained more than limit Complexity,
// largest gain first.
func gainers(units []scoring.Unit, limit float64) []scoring.Unit {
	var out []scoring.Unit
	for _, u := range units {
		if u.Kind == analysis.Function && float64(u.Delta.Values[scoring.Complexity]) > limit {
			out = append(out, u)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Delta.Values[scoring.Complexity] > out[j].Delta.Values[scoring.Complexity]
	})
	return out
}

func refs(units []scoring.Unit) []scoring.UnitRef {
	out := make([]scoring.UnitRef, len(units))
	for i, u := range units {
		out[i] = scoring.UnitRef{Kind: u.Kind, Name: u.Name, Path: u.Path, StartLine: u.StartLine, EndLine: u.EndLine}
	}
	return out
}
