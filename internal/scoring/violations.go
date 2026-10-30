package scoring

import "github.com/umbralux1/entropic/internal/analysis"

// Violation is a threshold a change exceeded. Value is the worst Delta
// measured against Limit; Units are the Units responsible, worst first.
type Violation struct {
	// Threshold is the threshold's JSON name (signal.Threshold.Name).
	Threshold string    `json:"threshold"`
	Limit     float64   `json:"limit"`
	Value     float64   `json:"value"`
	Units     []UnitRef `json:"units"`
}

// UnitRef identifies a Unit in the report: its kind, name and Location.
type UnitRef struct {
	Kind analysis.UnitKind `json:"kind"`
	Name string            `json:"name"`
	analysis.Location
}

// Ref identifies u.
func (u Unit) Ref() UnitRef {
	return UnitRef{Kind: u.Kind, Name: u.Name, Location: u.Location}
}
