// Package policy evaluates thresholds against the Delta of a change and
// names the Units responsible for each violation.
package policy

import (
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/scatter"
	"github.com/umbralux1/entropic/internal/scoring"
	"github.com/umbralux1/entropic/internal/signal"
)

// unitSignal is a Signal measured on Units, judged on the kind of Unit that
// best owns it.
type unitSignal struct {
	signal    signal.Signal
	kind      analysis.UnitKind
	threshold signal.Threshold
}

// unitSignals judges Complexity on functions, and Duplication on files,
// which also count cloned lines outside functions.
var unitSignals = []unitSignal{
	{signal.Complexity, analysis.Function, signal.ComplexityThreshold},
	{signal.Duplication, analysis.File, signal.DuplicationThreshold},
}

// Evaluate returns the thresholds r's production code exceeds, given the
// limit of each. A snapshot has no Delta, so it exceeds none.
func Evaluate(r scoring.Report, limit func(signal.Threshold) float64) []scoring.Violation {
	violations := []scoring.Violation{}
	if r.Baseline == nil {
		return violations
	}
	for _, s := range unitSignals {
		if v, ok := s.evaluate(r.Units, limit(s.threshold)); ok {
			violations = append(violations, v)
		}
	}
	if v, ok := scatterOverBudget(r.Scatter, limit(signal.ScatterThreshold)); ok {
		violations = append(violations, v)
	}
	if v, ok := entropyScore(r.Group, limit(signal.EntropyScoreThreshold)); ok {
		violations = append(violations, v)
	}
	return violations
}

// evaluate is violated by every Unit of s's kind gaining more than limit.
func (s unitSignal) evaluate(units []scoring.Unit, limit float64) (scoring.Violation, bool) {
	over := s.gainers(units, limit)
	if len(over) == 0 {
		return scoring.Violation{}, false
	}
	return scoring.Violation{
		Threshold: s.threshold.Name,
		Limit:     limit,
		Value:     float64(over[0].Delta.Values[s.signal]),
		Units:     refs(over),
	}, true
}

// gainers returns the Units of s's kind that gained more than limit in s,
// largest gain first.
func (s unitSignal) gainers(units []scoring.Unit, limit float64) []scoring.Unit {
	var out []scoring.Unit
	for _, u := range units {
		if u.Kind == s.kind && float64(u.Delta.Values[s.signal]) > limit {
			out = append(out, u)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Delta.Values[s.signal] > out[j].Delta.Values[s.signal]
	})
	return out
}

// scatterOverBudget is violated when a change touches more than limit
// Modules beyond its Scatter Budget. Every Module it touches is responsible.
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
		units[i] = scoring.UnitRef{Kind: analysis.Module, Name: m, Location: analysis.Location{Path: m}}
	}
	return scoring.Violation{Threshold: signal.ScatterThreshold.Name, Limit: limit, Value: excess, Units: units}, true
}

// entropyScore is violated when the Entropy Score rises by more than limit.
// The Units that gained in any Signal are responsible; if none did, the rise
// comes from code removed around them, so the changed files are.
func entropyScore(g scoring.Group, limit float64) (scoring.Violation, bool) {
	rise := *g.EntropyScore.Delta
	if rise <= limit {
		return scoring.Violation{}, false
	}
	var responsible []scoring.Unit
	for _, s := range unitSignals {
		responsible = append(responsible, s.gainers(g.Units, 0)...)
	}
	if len(responsible) == 0 {
		for _, u := range g.Units {
			if u.Kind == analysis.File && u.Status != scoring.Unchanged {
				responsible = append(responsible, u)
			}
		}
	}
	return scoring.Violation{
		Threshold: signal.EntropyScoreThreshold.Name,
		Limit:     limit,
		Value:     rise,
		Units:     refs(responsible),
	}, true
}

// refs identifies units in order.
func refs(units []scoring.Unit) []scoring.UnitRef {
	out := make([]scoring.UnitRef, len(units))
	for i, u := range units {
		out[i] = u.Ref()
	}
	return out
}
