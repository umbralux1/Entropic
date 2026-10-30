// Package scoring measures a set of source files: it analyses each one and
// rolls Unit values up from function to file to Module to repository.
package scoring

import (
	"bytes"
	"fmt"
	"path"
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
)

// SourceFile is one file to measure. Path is slash-separated and relative to
// the repository root.
type SourceFile struct {
	Path     string
	Source   []byte
	Language analysis.Language
}

// Signal names one measurable kind of entropy.
type Signal string

const Complexity Signal = "complexity"

// weights are the provisional Signal weights in the Entropy Score.
var weights = map[Signal]float64{Complexity: 1}

// Report is the result of a measurement.
type Report struct {
	SchemaVersion int `json:"schemaVersion"`
	// Baseline is nil for a snapshot.
	Baseline *Baseline `json:"baseline"`
	// EntropyScore is the weighted Signal total per 100 non-blank lines.
	EntropyScore Figure            `json:"entropyScore"`
	Signals      map[Signal]Figure `json:"signals"`
	Units        []Unit            `json:"units"`
}

// Figure is a repository-wide number and, when there is a Baseline, its Delta.
type Figure struct {
	Value float64  `json:"value"`
	Delta *float64 `json:"delta,omitempty"`
}

// Baseline is the commit a report was compared against, and how it was chosen.
type Baseline struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

// Unit is a measured Unit. Lines counts non-blank lines.
type Unit struct {
	Kind       analysis.UnitKind `json:"kind"`
	Name       string            `json:"name"`
	Path       string            `json:"path,omitempty"`
	Module     string            `json:"module,omitempty"`
	StartLine  int               `json:"startLine,omitempty"`
	EndLine    int               `json:"endLine,omitempty"`
	Lines      int               `json:"lines"`
	Complexity int               `json:"complexity"`
	Status     Status            `json:"status,omitempty"`
	Delta      *UnitDelta        `json:"delta,omitempty"`
}

const schemaVersion = 1

// Measure analyses files and returns their Units: functions, then files,
// then Modules, then the repository.
func Measure(files []SourceFile) (Report, error) {
	var functions, fileUnits []Unit
	modules := map[string]*Unit{}
	repo := Unit{Kind: analysis.Repository}

	for _, f := range files {
		units, err := analysis.Analyze(f.Source, f.Language)
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", f.Path, err)
		}
		module := path.Dir(f.Path)
		lines := bytes.Split(bytes.TrimSuffix(f.Source, []byte("\n")), []byte("\n"))
		file := Unit{Kind: analysis.File, Name: f.Path, Path: f.Path, Module: module,
			StartLine: 1, EndLine: len(lines), Lines: nonBlank(lines)}
		for _, u := range units {
			functions = append(functions, Unit{
				Kind: u.Kind, Name: u.Name, Path: f.Path, Module: module,
				StartLine: u.StartLine, EndLine: u.EndLine,
				Lines:      nonBlank(lines[u.StartLine-1 : u.EndLine]),
				Complexity: u.Complexity,
			})
			file.Complexity += u.Complexity
		}
		fileUnits = append(fileUnits, file)

		m, ok := modules[module]
		if !ok {
			m = &Unit{Kind: analysis.Module, Name: module, Path: module}
			modules[module] = m
		}
		m.add(file)
		repo.add(file)
	}

	units := append(functions, fileUnits...)
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		units = append(units, *modules[name])
	}
	units = append(units, repo)

	signals := map[Signal]Figure{Complexity: {Value: float64(repo.Complexity)}}
	return Report{
		SchemaVersion: schemaVersion,
		EntropyScore:  Figure{Value: entropyScore(signals, repo.Lines)},
		Signals:       signals,
		Units:         units,
	}, nil
}

// entropyScore weights each Signal and normalises by code size, so that
// adding code alone does not raise the score.
func entropyScore(signals map[Signal]Figure, lines int) float64 {
	if lines == 0 {
		return 0
	}
	total := 0.0
	for s, summary := range signals {
		total += weights[s] * summary.Value
	}
	return total * 100 / float64(lines)
}

// add rolls a child Unit's values into u.
func (u *Unit) add(child Unit) {
	u.Lines += child.Lines
	u.Complexity += child.Complexity
}

func nonBlank(lines [][]byte) int {
	n := 0
	for _, l := range lines {
		if len(bytes.TrimSpace(l)) > 0 {
			n++
		}
	}
	return n
}
