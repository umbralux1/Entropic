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

// UnitRef identifies a Unit in the report.
type UnitRef struct {
	Kind      analysis.UnitKind `json:"kind"`
	Name      string            `json:"name"`
	Path      string            `json:"path,omitempty"`
	StartLine int               `json:"startLine,omitempty"`
	EndLine   int               `json:"endLine,omitempty"`
}
