// Package cli is the entropic command line: it wires flags to the analysis
// pipeline and maps outcomes to exit codes.
package cli

import (
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/umbralux1/entropic/internal/language"
	"github.com/umbralux1/entropic/internal/reporters"
	"github.com/umbralux1/entropic/internal/revisions"
	"github.com/umbralux1/entropic/internal/scoring"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 2
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

	r, err := measure(dir, *base)
	if err == nil {
		r, err = withHotspots(dir, r, defaultChurnWindowDays)
	}
	if err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	if err := render(stdout, r); err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	return ExitOK
}

// defaultChurnWindowDays is how far back Churn looks when nothing else says.
// Ticket #8 makes it configurable; pass the configured value to withHotspots.
const defaultChurnWindowDays = 90

// withHotspots counts Churn over the last windowDays and ranks r's Hotspots.
func withHotspots(dir string, r scoring.Report, windowDays int) (scoring.Report, error) {
	churn, err := revisions.Churn(dir, time.Now().AddDate(0, 0, -windowDays))
	if err != nil {
		return scoring.Report{}, err
	}
	return r.WithHotspots(churn, windowDays), nil
}

// measure measures the working tree and, unless base is none, compares it
// against the Baseline.
func measure(dir, base string) (scoring.Report, error) {
	baseline, err := revisions.Resolve(dir, base)
	if err != nil {
		return scoring.Report{}, err
	}
	files, err := revisions.WorkingTree(dir, isSource)
	if err != nil {
		return scoring.Report{}, err
	}
	current, err := scoring.Measure(sourceFiles(files))
	if err != nil || baseline == nil {
		return current, err
	}

	files, err = revisions.AtCommit(dir, baseline.Commit, isSource)
	if err != nil {
		return scoring.Report{}, err
	}
	before, err := scoring.Measure(sourceFiles(files))
	if err != nil {
		return scoring.Report{}, fmt.Errorf("at Baseline %s: %w", baseline.Ref, err)
	}
	r := scoring.Compare(current, before)
	r.Baseline = &scoring.Baseline{Ref: baseline.Ref, Commit: baseline.Commit}
	return r, nil
}

func isSource(path string) bool {
	_, ok := language.ForPath(path)
	return ok
}

func sourceFiles(files []revisions.File) []scoring.SourceFile {
	out := make([]scoring.SourceFile, len(files))
	for i, f := range files {
		lang, _ := language.ForPath(f.Path)
		out[i] = scoring.SourceFile{Path: f.Path, Source: f.Source, Language: lang}
	}
	return out
}
