// Package signal is the registry of Signals and of the thresholds on them:
// their names, default weights and default limits are defined here once,
// and config validation, scoring, policy and reporting all read them.
package signal

// Signal names one measurable kind of entropy, as written in the JSON report
// and in entropic.toml's [weights] table.
type Signal string

const (
	Complexity  Signal = "complexity"
	Duplication Signal = "duplication"
	// Scatter is a Signal of a change, not of code: it is measured between
	// the Baseline and the working tree, not on Units, so it has no weight
	// and is not part of the Entropy Score.
	Scatter Signal = "scatter"
)

// OfCode are the Signals measured on Units, in reporting order. Each is
// weighed into the Entropy Score.
var OfCode = []Signal{Complexity, Duplication}

// DefaultWeights weigh each Signal of code in the Entropy Score. Values are
// provisional until tuned on real repositories.
func DefaultWeights() map[Signal]float64 {
	// A duplicated line weighs a tenth of a Complexity point.
	return map[Signal]float64{Complexity: 1, Duplication: 0.1}
}

// Threshold bounds how much a change may raise one figure; exceeding it is
// a Violation.
type Threshold struct {
	// Name names the threshold in the JSON report: camelCase.
	Name string
	// Key is its key in entropic.toml's [thresholds] table: snake_case.
	Key string
	// DefaultLimit applies when entropic.toml sets none. Provisional until
	// tuned on real repositories.
	DefaultLimit float64
}

// The thresholds Entropic checks.
var (
	// ComplexityThreshold is the most Complexity any one function may gain.
	ComplexityThreshold = Threshold{Name: "complexity", Key: "complexity", DefaultLimit: 10}
	// DuplicationThreshold is the most duplicated lines any one file may gain.
	DuplicationThreshold = Threshold{Name: "duplication", Key: "duplication", DefaultLimit: 20}
	// EntropyScoreThreshold is the most the Entropy Score may rise.
	EntropyScoreThreshold = Threshold{Name: "entropyScore", Key: "entropy_score", DefaultLimit: 10}
	// ScatterThreshold is how many Modules a change may touch beyond its
	// Scatter Budget.
	ScatterThreshold = Threshold{Name: "scatter", Key: "scatter", DefaultLimit: 0}
)

// Thresholds lists every threshold, in the order violations are reported.
var Thresholds = []Threshold{ComplexityThreshold, DuplicationThreshold, ScatterThreshold, EntropyScoreThreshold}
