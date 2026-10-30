// Package duplication finds structural clones: code fragments with the same
// shape once identifiers and literals are abstracted.
package duplication

import (
	"sort"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Location is a line range in a file. Lines are 1-based and inclusive.
type Location struct {
	Path      string `json:"path"`
	StartLine int    `json:"startLine"`
	EndLine   int    `json:"endLine"`
}

// CloneGroup is every Location of one duplicated fragment.
type CloneGroup struct {
	Locations []Location `json:"locations"`
}

// Find groups fragments, keyed by file path, into clone groups. Only the
// largest clones are reported: a group whose every Location lies inside a
// larger reported clone is not.
func Find(fragments map[string][]analysis.Fragment) []CloneGroup {
	byShape := map[uint64][]Location{}
	for path, fs := range fragments {
		for _, f := range fs {
			byShape[f.Shape] = append(byShape[f.Shape], Location{Path: path, StartLine: f.StartLine, EndLine: f.EndLine})
		}
	}
	var candidates [][]Location
	for _, locs := range byShape {
		if len(locs) > 1 {
			sortLocations(locs)
			candidates = append(candidates, locs)
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		if span(a[0]) != span(b[0]) {
			return span(a[0]) > span(b[0])
		}
		return less(a[0], b[0])
	})

	groups := []CloneGroup{}
	var reported []Location
	for _, locs := range candidates {
		if allInside(locs, reported) {
			continue
		}
		groups = append(groups, CloneGroup{Locations: locs})
		reported = append(reported, locs...)
	}
	sort.Slice(groups, func(i, j int) bool { return less(groups[i].Locations[0], groups[j].Locations[0]) })
	return groups
}

func span(l Location) int { return l.EndLine - l.StartLine + 1 }

// allInside reports whether every Location in locs lies inside one of outer.
func allInside(locs, outer []Location) bool {
	for _, l := range locs {
		if !insideAny(l, outer) {
			return false
		}
	}
	return true
}

func insideAny(l Location, outer []Location) bool {
	for _, o := range outer {
		if o.Path == l.Path && o.StartLine <= l.StartLine && l.EndLine <= o.EndLine {
			return true
		}
	}
	return false
}

func sortLocations(locs []Location) {
	sort.Slice(locs, func(i, j int) bool { return less(locs[i], locs[j]) })
}

func less(a, b Location) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	if a.StartLine != b.StartLine {
		return a.StartLine < b.StartLine
	}
	return a.EndLine < b.EndLine
}
