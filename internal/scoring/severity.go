package scoring

import (
	"slices"
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
)

// InSeverityOrder returns r with each group's Units in the order every
// reporter lists them: by kind, finest first; within a kind by weighted
// Delta, largest first, weighing Signals as in the Entropy Score (by
// weighted value for a snapshot); then by weighted value. Ties keep source
// order.
func (r Report) InSeverityOrder() Report {
	r.Units = r.bySeverity(r.Units)
	r.Tests.Units = r.bySeverity(r.Tests.Units)
	return r
}

func (r Report) bySeverity(units []Unit) []Unit {
	out := slices.Clone(units)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if ka, kb := kindRank(a.Kind), kindRank(b.Kind); ka != kb {
			return ka < kb
		}
		if sa, sb := r.severity(a), r.severity(b); sa != sb {
			return sa > sb
		}
		return a.Values.Weighted(r.weights) > b.Values.Weighted(r.weights)
	})
	return out
}

// severity is a Unit's weighted Delta, or its weighted value in a snapshot.
func (r Report) severity(u Unit) float64 {
	if u.Delta == nil {
		return u.Values.Weighted(r.weights)
	}
	return u.Delta.Values.Weighted(r.weights)
}

func kindRank(k analysis.UnitKind) int {
	return slices.Index(analysis.UnitKinds, k)
}
