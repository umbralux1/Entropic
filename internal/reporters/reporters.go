// Package reporters renders a measurement report for its consumers: JSON for
// agents and CI, text for humans. Both render the same scoring.Report.
package reporters

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/scoring"
)

// JSON writes r as the versioned JSON report.
func JSON(w io.Writer, r scoring.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Text writes r as a terminal-friendly report: the Baseline, the Entropy
// Score and Signal summaries, then the Units.
func Text(w io.Writer, r scoring.Report) error {
	var b strings.Builder
	if r.Baseline == nil {
		b.WriteString("Snapshot (no Baseline)\n\n")
	} else {
		fmt.Fprintf(&b, "Baseline: %s (%s)\n\n", r.Baseline.Ref, shortCommit(r.Baseline.Commit))
	}

	fmt.Fprintf(&b, "Entropy Score  %s\n", figure(r.EntropyScore))
	signals := make([]string, 0, len(r.Signals))
	for s := range r.Signals {
		signals = append(signals, string(s))
	}
	sort.Strings(signals)
	for _, s := range signals {
		fmt.Fprintf(&b, "%-14s %s\n", title(s), figure(r.Signals[scoring.Signal(s)]))
	}

	b.WriteString("\nUnits\n")
	tw := tabwriter.NewWriter(&b, 0, 0, 2, ' ', 0)
	for _, u := range bySeverity(r.Units) {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t", u.Kind, u.Name, location(u))
		for _, s := range scoring.Signals {
			fmt.Fprintf(tw, "%s %s\t", s, unitValue(u, s))
		}
		fmt.Fprintf(tw, "%s\n", u.Status)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// bySeverity returns units grouped by kind, finest first, and within a kind
// ordered by weighted Signal Delta, largest first (by weighted Signal values
// for a snapshot).
func bySeverity(units []scoring.Unit) []scoring.Unit {
	rank := map[analysis.UnitKind]int{}
	for _, u := range units {
		if _, ok := rank[u.Kind]; !ok {
			rank[u.Kind] = len(rank)
		}
	}
	out := append([]scoring.Unit(nil), units...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if rank[a.Kind] != rank[b.Kind] {
			return rank[a.Kind] < rank[b.Kind]
		}
		if severity(a) != severity(b) {
			return severity(a) > severity(b)
		}
		return a.Values.Weighted() > b.Values.Weighted()
	})
	return out
}

func severity(u scoring.Unit) float64 {
	if u.Delta == nil {
		return u.Values.Weighted()
	}
	return u.Delta.Values.Weighted()
}

func figure(f scoring.Figure) string {
	if f.Delta == nil {
		return number(f.Value)
	}
	sign := "+"
	if *f.Delta < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s (%s%s)", number(f.Value), sign, number(*f.Delta))
}

// number rounds v to two decimals for reading, dropping trailing zeros.
func number(v float64) string {
	s := strings.TrimRight(fmt.Sprintf("%.2f", v), "0")
	return strings.TrimSuffix(s, ".")
}

func unitValue(u scoring.Unit, s scoring.Signal) string {
	if u.Delta == nil {
		return fmt.Sprint(u.Values[s])
	}
	return fmt.Sprintf("%d (%+d)", u.Values[s], u.Delta.Values[s])
}

// location is where a Unit lives: path and line range for functions and
// files, the path alone for Modules, and nothing for the repository.
func location(u scoring.Unit) string {
	if u.StartLine == 0 {
		return u.Path
	}
	return fmt.Sprintf("%s:%d-%d", u.Path, u.StartLine, u.EndLine)
}

func title(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func shortCommit(c string) string {
	if len(c) > 12 {
		return c[:12]
	}
	return c
}
