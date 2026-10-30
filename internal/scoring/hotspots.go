package scoring

import (
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Hotspot is a file Unit whose entropy is paid for repeatedly: it has both
// entropy and Churn. Score is entropy times Churn.
type Hotspot struct {
	Path    string  `json:"path"`
	Churn   int     `json:"churn"`
	Entropy float64 `json:"entropy"`
	Score   float64 `json:"score"`
}

// WithHotspots returns r with its file Units that have both entropy and
// Churn listed as Hotspots, highest score first. churn maps file paths to
// their Churn over a window of windowDays. Churn is not a Signal: it does not
// feed the Entropy Score.
func (r Report) WithHotspots(churn map[string]int, windowDays int) Report {
	r.ChurnWindowDays = windowDays
	r.Hotspots = []Hotspot{}
	for _, u := range r.Units {
		if u.Kind != analysis.File || u.Status == Removed {
			continue
		}
		e := entropy(u)
		if c := churn[u.Path]; c > 0 && e > 0 {
			r.Hotspots = append(r.Hotspots, Hotspot{Path: u.Path, Churn: c, Entropy: e, Score: e * float64(c)})
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

// entropy is a Unit's weighted Signal total, weighted as in the Entropy Score.
func entropy(u Unit) float64 {
	return weights[Complexity] * float64(u.Complexity)
}
