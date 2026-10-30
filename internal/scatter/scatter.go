// Package scatter measures how many Modules a change touches and judges that
// against a Scatter Budget that grows slowly with the size of the change.
package scatter

import (
	"maps"
	"math"
	"slices"
)

// Change is how many lines changed (added plus deleted) in one file of a
// Module between the Baseline and the current state.
type Change struct {
	Path   string
	Module string
	Lines  int
}

// Scatter is the Scatter of one change.
type Scatter struct {
	// Modules is the number of Modules the change touches: the Scatter Signal.
	Modules int `json:"modules"`
	// TouchedModules names those Modules, sorted.
	TouchedModules []string `json:"touchedModules"`
	// Budget is the Scatter Budget for a change of this size.
	Budget       float64 `json:"budget"`
	OverBudget   bool    `json:"overBudget"`
	ChangedLines int     `json:"changedLines"`
	// Informational values are reported but feed no Signal or Entropy Score.
	Informational Informational `json:"informational"`
}

// Informational holds Scatter's informational values.
type Informational struct {
	// Files is the number of files the change touches.
	Files int `json:"files"`
	// Entropy is the Shannon entropy, in bits, of changed lines across Modules.
	Entropy float64 `json:"entropy"`
	// EffectiveModules is 2^Entropy: how many equally changed Modules would
	// spread the change as evenly. 0 when nothing changed.
	EffectiveModules float64 `json:"effectiveModules"`
}

// Measure returns the Scatter of a change given as its changed files.
func Measure(changes []Change) Scatter {
	perModule := map[string]int{}
	var s Scatter
	for _, c := range changes {
		if c.Lines == 0 {
			continue
		}
		perModule[c.Module] += c.Lines
		s.ChangedLines += c.Lines
		s.Informational.Files++
	}
	s.Modules = len(perModule)
	s.TouchedModules = slices.Sorted(maps.Keys(perModule))
	s.Budget = budget(s.ChangedLines)
	s.OverBudget = float64(s.Modules) > s.Budget
	if s.ChangedLines > 0 {
		s.Informational.Entropy = entropy(perModule, s.TouchedModules, s.ChangedLines)
		s.Informational.EffectiveModules = math.Exp2(s.Informational.Entropy)
	}
	return s
}

// budget is the provisional Scatter Budget: one Module, plus one more each
// time the change roughly doubles past 10 lines. To be tuned on real history.
func budget(changedLines int) float64 {
	return 1 + math.Log2(1+float64(changedLines)/10)
}

// entropy sums in touched (sorted) Module order so the result is identical run to run.
func entropy(perModule map[string]int, touched []string, total int) float64 {
	h := 0.0
	for _, m := range touched {
		p := float64(perModule[m]) / float64(total)
		h -= p * math.Log2(p)
	}
	return h
}
