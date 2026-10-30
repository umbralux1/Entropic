package scoring

import (
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Hotspot is a file Unit whose entropy is paid for repeatedly: it has both
// Signal values and Churn.
type Hotspot struct {
	UnitRef
	Churn int `json:"churn"`
	// WeightedTotal is the file's Signal values weighted as in the Entropy
	// Score, before normalising by size.
	WeightedTotal float64 `json:"weightedTotal"`
	// Score is WeightedTotal times Churn.
	Score float64 `json:"score"`
}

// WithHotspots returns r with its file Units that have both a weighted
// Signal total and Churn listed as Hotspots, highest score first. churn maps
// file paths to their Churn over a window of windowDays. Churn is not a
// Signal: it does not feed the Entropy Score.
func (r Report) WithHotspots(churn map[string]int, windowDays int) Report {
	r.ChurnWindowDays = windowDays
	r.Hotspots = []Hotspot{}
	for _, u := range r.Units {
		if u.Kind != analysis.File || u.Status == Removed {
			continue
		}
		total := u.Values.Weighted(r.weights)
		if c := churn[u.Path]; c > 0 && total > 0 {
			r.Hotspots = append(r.Hotspots, Hotspot{UnitRef: u.Ref(), Churn: c, WeightedTotal: total, Score: total * float64(c)})
		}
	}
	sort.SliceStable(r.Hotspots, func(i, j int) bool {
		a, b := r.Hotspots[i], r.Hotspots[j]
		if a.Score != b.Score {
			return a.Score > b.Score
		}
		return a.Path < b.Path
	})
	return r
}
