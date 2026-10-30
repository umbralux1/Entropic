// Package scoring measures a set of source files: it analyses each one and
// rolls Unit values up from function to file to Module to repository.
package scoring

import (
	"bytes"
	"errors"
	"fmt"
	"path"
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/duplication"
	"github.com/umbralux1/entropic/internal/scatter"
)

// SourceFile is one file to measure. Path is slash-separated and relative to
// the repository root. Test files are measured in a group of their own.
type SourceFile struct {
	Path     string
	Source   []byte
	Language analysis.Language
	Test     bool
}

// Report is the result of a measurement. Its own Group is production code;
// Tests is the same for test files, so they never affect production Signals.
type Report struct {
	SchemaVersion int `json:"schemaVersion"`
	// Baseline is nil for a snapshot.
	Baseline *Baseline `json:"baseline"`
	// SkippedAtBaseline are the files left out of the Baseline because they
	// failed to parse there; empty for a snapshot.
	SkippedAtBaseline []string `json:"skippedAtBaseline"`
	Group
	Tests Group `json:"tests"`
	// ChurnWindowDays is the history window Hotspot Churn was counted over.
	ChurnWindowDays int `json:"churnWindowDays"`
	// Hotspots rank production file Units by entropy times Churn.
	Hotspots []Hotspot `json:"hotspots"`
	// Scatter is the Scatter of the change since the Baseline in production
	// code; nil for a snapshot.
	Scatter *scatter.Scatter `json:"scatter,omitempty"`
	// Violations are the thresholds the change exceeded; none for a snapshot.
	Violations []Violation `json:"violations"`

	// unparsable are the files skipped because they failed to parse.
	unparsable []string
	// shapes are the fragment shapes Duplication compared, per file path.
	shapes map[string]map[uint64]bool
}

// Group is the measurement of one group of files, rolled up to its own
// repository Unit.
type Group struct {
	// EntropyScore is the weighted Signal total per 100 non-blank lines.
	EntropyScore Figure            `json:"entropyScore"`
	Signals      map[Signal]Figure `json:"signals"`
	Units        []Unit            `json:"units"`
	// CloneGroups are the structural clones found by Duplication.
	CloneGroups []duplication.CloneGroup `json:"cloneGroups"`
}

// ModuleOf returns the Module a file belongs to: its directory, "." for the
// repository root.
func ModuleOf(file string) string {
	return path.Dir(file)
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
	Kind      analysis.UnitKind `json:"kind"`
	Name      string            `json:"name"`
	Path      string            `json:"path,omitempty"`
	Module    string            `json:"module,omitempty"`
	StartLine int               `json:"startLine,omitempty"`
	EndLine   int               `json:"endLine,omitempty"`
	Lines     int               `json:"lines"`
	// Values are the Unit's value per Signal, written to JSON as one field
	// per Signal.
	Values Values     `json:"-"`
	Status Status     `json:"status,omitempty"`
	Delta  *UnitDelta `json:"delta,omitempty"`
	// Informational values are reported on functions but feed no Signal,
	// Entropy Score or Delta, and do not roll up.
	Informational *Informational `json:"informational,omitempty"`
}

// Informational holds a function Unit's informational values.
type Informational struct {
	Cyclomatic int `json:"cyclomatic"`
}

const schemaVersion = 1

// Options configure a measurement.
type Options struct {
	// Weights weigh each Signal in the Entropy Score.
	Weights map[Signal]float64
	// DuplicateMinLines is the smallest clone reported, in lines holding code.
	DuplicateMinLines int
	// SkipUnparsable leaves out files that fail to parse, as Compare then
	// reports, instead of failing the measurement.
	SkipUnparsable bool
	// Baseline, when measuring against one, is the Baseline's measurement:
	// a clone location already there is its group's original.
	Baseline *Report
}

// Measure analyses files, splitting production code from tests.
func Measure(files []SourceFile, opts Options) (Report, error) {
	r := Report{SchemaVersion: schemaVersion, SkippedAtBaseline: []string{}, shapes: map[string]map[uint64]bool{}}
	var production, tests []analysedFile
	for _, f := range files {
		a, err := analyse(f, opts.DuplicateMinLines)
		var syntax *analysis.SyntaxError
		if opts.SkipUnparsable && errors.As(err, &syntax) {
			r.unparsable = append(r.unparsable, f.Path)
			continue
		}
		if err != nil {
			return Report{}, fmt.Errorf("%s: %w", f.Path, err)
		}
		r.shapes[f.Path] = map[uint64]bool{}
		for _, fr := range a.fragments {
			r.shapes[f.Path][fr.Shape] = true
		}
		if f.Test {
			tests = append(tests, a)
		} else {
			production = append(production, a)
		}
	}
	var existed duplication.Existed
	if b := opts.Baseline; b != nil {
		existed = func(path string, shape uint64) bool { return b.shapes[path][shape] }
	}
	r.Group = measureGroup(production, opts.Weights, existed)
	r.Tests = measureGroup(tests, opts.Weights, existed)
	return r, nil
}

// Unparsable returns the files Measure skipped because they failed to parse.
func (r Report) Unparsable() []string {
	return r.unparsable
}

// analyse finds a file's function and class Units and its fragments that
// Duplication compares.
func analyse(f SourceFile, duplicateMinLines int) (analysedFile, error) {
	units, err := analysis.Analyze(f.Source, f.Language)
	if err != nil {
		return analysedFile{}, err
	}
	fragments, err := analysis.Fragments(f.Source, f.Language, duplicateMinLines)
	if err != nil {
		return analysedFile{}, err
	}
	return analysedFile{SourceFile: f, units: units, fragments: fragments,
		lines: bytes.Split(bytes.TrimSuffix(f.Source, []byte("\n")), []byte("\n"))}, nil
}

// measureGroup returns the Units of files: functions, then classes, then
// files, then Modules, then the repository. Clones are sought within the group.
func measureGroup(analysed []analysedFile, weights map[Signal]float64, existed duplication.Existed) Group {
	fragments := map[string][]analysis.Fragment{}
	for _, f := range analysed {
		fragments[f.Path] = f.fragments
	}
	clones := duplication.Find(fragments, existed)
	cloned := clonedLines(clones)

	var functions, classes, fileUnits []Unit
	modules := map[string]*Unit{}
	repo := Unit{Kind: analysis.Repository, Values: Values{}}
	for _, f := range analysed {
		module := ModuleOf(f.Path)
		file := Unit{Kind: analysis.File, Name: f.Path, Path: f.Path, Module: module,
			StartLine: 1, EndLine: len(f.lines), Lines: nonBlank(f.lines),
			Values: Values{Duplication: f.duplicated(cloned[f.Path], 1, len(f.lines))}}
		for _, u := range f.units {
			unit := Unit{
				Kind: u.Kind, Name: u.Name, Path: f.Path, Module: module,
				StartLine: u.StartLine, EndLine: u.EndLine,
				Lines: nonBlank(f.lines[u.StartLine-1 : u.EndLine]),
				Values: Values{
					Complexity:  u.Complexity,
					Duplication: f.duplicated(cloned[f.Path], u.StartLine, u.EndLine),
				},
			}
			// A class already rolls up the functions it encloses, so only
			// functions roll up into the file.
			if u.Kind == analysis.Function {
				unit.Informational = &Informational{Cyclomatic: u.Cyclomatic}
				file.Values[Complexity] += u.Complexity
				functions = append(functions, unit)
			} else {
				classes = append(classes, unit)
			}
		}
		fileUnits = append(fileUnits, file)

		m, ok := modules[module]
		if !ok {
			m = &Unit{Kind: analysis.Module, Name: module, Path: module, Values: Values{}}
			modules[module] = m
		}
		m.add(file)
		repo.add(file)
	}

	units := append(append(functions, classes...), fileUnits...)
	names := make([]string, 0, len(modules))
	for name := range modules {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		units = append(units, *modules[name])
	}
	units = append(units, repo)

	signals := map[Signal]Figure{}
	for _, s := range Signals {
		signals[s] = Figure{Value: float64(repo.Values[s])}
	}
	return Group{
		EntropyScore: Figure{Value: entropyScore(repo, weights)},
		Signals:      signals,
		Units:        units,
		CloneGroups:  clones,
	}
}

// analysedFile is a source file with its function Units and its lines.
type analysedFile struct {
	SourceFile
	units     []analysis.Unit
	fragments []analysis.Fragment
	lines     [][]byte
}

// duplicated counts the non-blank lines from start to end that lie in a clone.
func (f analysedFile) duplicated(cloned map[int]bool, start, end int) int {
	n := 0
	for line := start; line <= end; line++ {
		if cloned[line] && len(bytes.TrimSpace(f.lines[line-1])) > 0 {
			n++
		}
	}
	return n
}

// clonedLines returns, per file path, the lines lying in an additional copy:
// the original of each clone group is not duplication.
func clonedLines(groups []duplication.CloneGroup) map[string]map[int]bool {
	cloned := map[string]map[int]bool{}
	for _, g := range groups {
		for _, l := range g.Locations {
			if l.Original {
				continue
			}
			if cloned[l.Path] == nil {
				cloned[l.Path] = map[int]bool{}
			}
			for line := l.StartLine; line <= l.EndLine; line++ {
				cloned[l.Path][line] = true
			}
		}
	}
	return cloned
}

// entropyScore weights each Signal and normalises by code size, so that
// adding code alone does not raise the score.
func entropyScore(repo Unit, weights map[Signal]float64) float64 {
	if repo.Lines == 0 {
		return 0
	}
	return repo.Values.Weighted(weights) * 100 / float64(repo.Lines)
}

// add rolls a child Unit's values into u.
func (u *Unit) add(child Unit) {
	u.Lines += child.Lines
	u.Values = u.Values.add(child.Values)
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
