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

// CloneGroup is every location of one duplicated fragment. One of them is
// the original; Duplication counts only the others, the additional copies.
type CloneGroup struct {
	Locations []CloneLocation `json:"locations"`
}

// CloneLocation is one location of a clone group.
type CloneLocation struct {
	Location
	// Original marks the location the others are copies of.
	Original bool `json:"original"`
}

// Existed reports whether a fragment of the given shape was already in the
// file at path before the change being measured.
type Existed func(path string, shape uint64) bool

// Find groups fragments, keyed by file path, into clone groups. Only the
// largest clones are reported: a group whose every Location lies inside a
// larger reported clone is not. The original of each group is its first
// location, by path and line, that existed; the first location if none did
// or existed is nil.
func Find(fragments map[string][]analysis.Fragment, existed Existed) []CloneGroup {
	byShape := map[uint64][]Location{}
	for path, fs := range fragments {
		for _, f := range fs {
			byShape[f.Shape] = append(byShape[f.Shape], Location{Path: path, StartLine: f.StartLine, EndLine: f.EndLine})
		}
	}
	var candidates [][]Location
	shapeOf := map[*Location]uint64{}
	for shape, locs := range byShape {
		if len(locs) > 1 {
			sortLocations(locs)
			candidates = append(candidates, locs)
			shapeOf[&locs[0]] = shape
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
		groups = append(groups, group(locs, shapeOf[&locs[0]], existed))
		reported = append(reported, locs...)
	}
	sort.Slice(groups, func(i, j int) bool {
		return less(groups[i].Locations[0].Location, groups[j].Locations[0].Location)
	})
	return groups
}

// group makes a clone group of sorted locs of one shape, marking its original.
func group(locs []Location, shape uint64, existed Existed) CloneGroup {
	original := 0
	if existed != nil {
		for i, l := range locs {
			if existed(l.Path, shape) {
				original = i
				break
			}
		}
	}
	g := CloneGroup{Locations: make([]CloneLocation, len(locs))}
	for i, l := range locs {
		g.Locations[i] = CloneLocation{Location: l, Original: i == original}
	}
	return g
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
