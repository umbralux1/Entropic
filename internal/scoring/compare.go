package scoring

import (
	"fmt"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Status says how a Unit differs from its counterpart at the Baseline.
type Status string

const (
	Added     Status = "added"
	Removed   Status = "removed"
	Changed   Status = "changed"
	Unchanged Status = "unchanged"
)

// UnitDelta is a Unit's change in value against the Baseline.
type UnitDelta struct {
	Lines int `json:"lines"`
	// Values are the change per Signal, written to JSON as one field per Signal.
	Values Values `json:"-"`
}

// Compare returns current with every Unit marked and given its Delta against
// the same Unit in baseline. Units only in baseline are appended as Removed.
// Files baseline skipped as unparsable are listed in SkippedAtBaseline.
func Compare(current, baseline Report) Report {
	out := current
	out.SkippedAtBaseline = append([]string{}, baseline.unparsable...)
	out.Group = compareGroup(current.Group, baseline.Group)
	out.Tests = compareGroup(current.Tests, baseline.Tests)
	return out
}

func compareGroup(current, baseline Group) Group {
	before := map[string]Unit{}
	baseKeys := identities(baseline.Units)
	for i, key := range baseKeys {
		before[key] = baseline.Units[i]
	}

	out := current
	out.EntropyScore = withDelta(current.EntropyScore, baseline.EntropyScore)
	out.Signals = map[Signal]Figure{}
	for s, f := range current.Signals {
		out.Signals[s] = withDelta(f, baseline.Signals[s])
	}
	out.Units = nil
	for i, key := range identities(current.Units) {
		u := current.Units[i]
		b, existed := before[key]
		delete(before, key)
		u.Delta = &UnitDelta{Lines: u.Lines - b.Lines, Values: u.Values.minus(b.Values)}
		switch {
		case !existed:
			u.Status = Added
		case u.Delta.Lines != 0 || !u.Delta.Values.isZero():
			u.Status = Changed
		default:
			u.Status = Unchanged
		}
		out.Units = append(out.Units, u)
	}
	for _, key := range baseKeys {
		b, gone := before[key]
		if !gone {
			continue
		}
		u := b
		u.Lines, u.Values = 0, Values{}
		u.Status = Removed
		u.Delta = &UnitDelta{Lines: -b.Lines, Values: Values{}.minus(b.Values)}
		out.Units = append(out.Units, u)
	}
	return out
}

func withDelta(now, before Figure) Figure {
	d := now.Value - before.Value
	return Figure{Value: now.Value, Delta: &d}
}

// identities returns, per Unit, a key that identifies it across revisions:
// kind, path and name, plus an occurrence count for same-named Units.
func identities(units []Unit) []string {
	seen := map[string]int{}
	keys := make([]string, len(units))
	for i, u := range units {
		id := fmt.Sprintf("%s|%s|%s", u.Kind, u.Path, u.Name)
		if u.Kind == analysis.Repository {
			id = string(analysis.Repository)
		}
		keys[i] = fmt.Sprintf("%s#%d", id, seen[id])
		seen[id]++
	}
	return keys
}
