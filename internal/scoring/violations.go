package scoring

import "github.com/umbralux1/entropic/internal/analysis"

// Threshold names a threshold: one per Signal, plus the Entropy Score.
type Threshold string

const (
	ComplexityThreshold   Threshold = "complexity"
	EntropyScoreThreshold Threshold = "entropyScore"
	ScatterThreshold      Threshold = "scatter"
)

// Violation is a threshold a change exceeded. Value is the worst Delta
// measured against Limit; Units are the Units responsible, worst first.
type Violation struct {
	Threshold Threshold `json:"threshold"`
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
