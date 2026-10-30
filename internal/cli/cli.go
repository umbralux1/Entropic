// Package cli is the entropic command line: it wires flags to the analysis
// pipeline and maps outcomes to exit codes.
package cli

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/umbralux1/entropic/internal/config"
	"github.com/umbralux1/entropic/internal/language"
	"github.com/umbralux1/entropic/internal/policy"
	"github.com/umbralux1/entropic/internal/reporters"
	"github.com/umbralux1/entropic/internal/revisions"
	"github.com/umbralux1/entropic/internal/scatter"
	"github.com/umbralux1/entropic/internal/scoring"
)

// Exit codes.
const (
	ExitOK                = 0
	ExitThresholdExceeded = 1
	ExitError             = 2
)

// renderers maps each --format value to its reporter.
var renderers = map[string]func(io.Writer, scoring.Report) error{
	"json": reporters.JSON,
	"text": reporters.Text,
}

// Run executes entropic with args in the repository at dir and returns the
// process exit code.
func Run(args []string, dir string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("entropic", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", revisions.Auto,
		"Baseline revision to compare against, or none for a snapshot (default: HEAD if the working tree is dirty, else merge-base with main)")
	format := flags.String("format", "json", "output format: json or text")
	if err := flags.Parse(args); err != nil {
		return ExitError
	}
	render, ok := renderers[*format]
	if !ok {
		fmt.Fprintf(stderr, "entropic: unknown --format %s; use json or text\n", *format)
		return ExitError
	}

	cfg, err := config.Load(dir)
	if err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	r, err := measure(dir, *base, cfg)
	if err == nil {
		r, err = withHotspots(dir, r, cfg.ChurnWindowDays, signalWeights(cfg))
	}
	if err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	r.Violations = policy.Evaluate(r, cfg.Thresholds)
	if err := render(stdout, r); err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	if len(r.Violations) > 0 {
		return ExitThresholdExceeded
	}
	return ExitOK
}

// withHotspots counts Churn over the last windowDays and ranks r's Hotspots.
func withHotspots(dir string, r scoring.Report, windowDays int, weights map[scoring.Signal]float64) (scoring.Report, error) {
	churn, err := revisions.Churn(dir, time.Now().AddDate(0, 0, -windowDays))
	if err != nil {
		return scoring.Report{}, err
	}
	return r.WithHotspots(churn, windowDays, weights), nil
}

// measure measures the working tree and, unless base is none, compares it
// against the Baseline. Both are measured with the current config, so the
// Delta reflects the code alone.
func measure(dir, base string, cfg config.Config) (scoring.Report, error) {
	weights := signalWeights(cfg)
	inScope := func(path string) bool {
		lang, ok := language.ForPath(path)
		return ok && !lang.Vendored(path) && !cfg.Excluded(path)
	}
	baseline, err := revisions.Resolve(dir, base)
	if err != nil {
		return scoring.Report{}, err
	}
	nowFiles, err := revisions.WorkingTree(dir, inScope)
	if err != nil {
		return scoring.Report{}, err
	}
	current, err := scoring.Measure(sourceFiles(nowFiles), weights, cfg.DuplicateMinLines)
	if err != nil || baseline == nil {
		return current, err
	}

	baseFiles, err := revisions.AtCommit(dir, baseline.Commit, inScope)
	if err != nil {
		return scoring.Report{}, err
	}
	before, err := scoring.Measure(sourceFiles(baseFiles), weights, cfg.DuplicateMinLines)
	if err != nil {
		return scoring.Report{}, fmt.Errorf("at Baseline %s: %w", baseline.Ref, err)
	}
	r := scoring.Compare(current, before)
	r.Baseline = &scoring.Baseline{Ref: baseline.Ref, Commit: baseline.Commit}

	changed, err := revisions.ChangedLines(dir, baseline.Commit, inScope)
	if err != nil {
		return scoring.Report{}, err
	}
	s := scatter.Measure(changes(changed, nowFiles, baseFiles))
	r.Scatter = &s
	return r, nil
}

// changes turns changed lines per file into Scatter's input, leaving out
// test files, like every production Signal, and files that are generated
// now or were at the Baseline.
func changes(changed map[string]int, current, baseline []revisions.File) []scatter.Change {
	generated := map[string]bool{}
	for _, f := range append(append([]revisions.File(nil), current...), baseline...) {
		lang, _ := language.ForPath(f.Path)
		if lang.IsGenerated(f.Source) {
			generated[f.Path] = true
		}
	}
	out := make([]scatter.Change, 0, len(changed))
	for path, lines := range changed {
		if lang, _ := language.ForPath(path); generated[path] || lang.IsTest(path) {
			continue
		}
		out = append(out, scatter.Change{Path: path, Module: scoring.ModuleOf(path), Lines: lines})
	}
	return out
}

// signalWeights is cfg's Signal weights keyed by Signal.
func signalWeights(cfg config.Config) map[scoring.Signal]float64 {
	weights := map[scoring.Signal]float64{}
	for name, w := range cfg.Weights {
		weights[scoring.Signal(name)] = w
	}
	return weights
}

// sourceFiles pairs each file with its language, leaving out generated code.
func sourceFiles(files []revisions.File) []scoring.SourceFile {
	var out []scoring.SourceFile
	for _, f := range files {
		lang, _ := language.ForPath(f.Path)
		if lang.IsGenerated(f.Source) {
			continue
		}
		out = append(out, scoring.SourceFile{Path: f.Path, Source: f.Source, Language: lang, Test: lang.IsTest(f.Path)})
	}
	return out
}
