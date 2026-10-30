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
	"github.com/umbralux1/entropic/internal/signal"
)

// JSON writes r as the versioned JSON report.
func JSON(w io.Writer, r scoring.Report) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Text writes r as a terminal-friendly report: the Baseline, the Entropy
// Score and Signal summaries, then the Units; test files follow in a
// section of their own. Threshold violations, if any, come first.
func Text(w io.Writer, r scoring.Report) error {
	var b strings.Builder
	violations(&b, r.Violations)
	if r.Baseline == nil {
		b.WriteString("Snapshot (no Baseline)\n\n")
	} else {
		fmt.Fprintf(&b, "Baseline: %s (%s)\n", r.Baseline.Ref, shortCommit(r.Baseline.Commit))
		if s := r.Scatter; s != nil {
			verdict := "within budget"
			if s.OverBudget {
				verdict = "over budget"
			}
			fmt.Fprintf(&b, "Scatter: %d Modules, %d changed lines (budget %s, %s)\n",
				s.Modules, s.ChangedLines, number(s.Budget), verdict)
		}
		b.WriteString("\n")
	}
	if err := group(&b, r.Group); err != nil {
		return err
	}
	// A group without files still has its repository Unit.
	if len(r.Tests.Units) > 1 {
		b.WriteString("\nTests\n\n")
		if err := group(&b, r.Tests); err != nil {
			return err
		}
	}
	_, err := io.WriteString(w, b.String())
	return err
}

// violations writes each exceeded threshold with the Units responsible.
func violations(b *strings.Builder, vs []scoring.Violation) {
	if len(vs) == 0 {
		return
	}
	b.WriteString("Threshold exceeded\n")
	for _, v := range vs {
		fmt.Fprintf(b, "  %s +%s > %s\n", v.Threshold, number(v.Value), number(v.Limit))
		for _, u := range v.Units {
			fmt.Fprintf(b, "    %s %s %s\n", u.Kind, u.Name, location(scoring.Unit{Path: u.Path, StartLine: u.StartLine, EndLine: u.EndLine}))
		}
	}
	b.WriteString("\n")
}

// group writes one group's Entropy Score, Signal summaries and Units.
func group(b *strings.Builder, g scoring.Group) error {
	fmt.Fprintf(b, "Entropy Score  %s\n", figure(g.EntropyScore))
	signals := make([]string, 0, len(g.Signals))
	for s := range g.Signals {
		signals = append(signals, string(s))
	}
	sort.Strings(signals)
	for _, s := range signals {
		fmt.Fprintf(b, "%-14s %s\n", title(s), figure(g.Signals[signal.Signal(s)]))
	}

	b.WriteString("\nUnits\n")
	tw := tabwriter.NewWriter(b, 0, 0, 2, ' ', 0)
	for _, u := range bySeverity(g.Units) {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t", u.Kind, u.Name, location(u))
		for _, s := range signal.OfCode {
			fmt.Fprintf(tw, "%s %s\t", s, unitValue(u, s))
		}
		fmt.Fprintf(tw, "%s\n", u.Status)
	}
	return tw.Flush()
}

// bySeverity returns units grouped by kind, finest first, and within a kind
// ordered by Signal Delta, largest first (by Signal values for a snapshot),
// comparing Signals in reporting order.
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
		if c := compareValues(severity(a), severity(b)); c != 0 {
			return c > 0
		}
		return compareValues(a.Values, b.Values) > 0
	})
	return out
}

// compareValues orders Values by each Signal in reporting order: positive
// when a is larger, negative when b is, zero when equal.
func compareValues(a, b scoring.Values) int {
	for _, s := range signal.OfCode {
		if a[s] != b[s] {
			return a[s] - b[s]
		}
	}
	return 0
}

// severity is a Unit's Delta, or its values for a snapshot.
func severity(u scoring.Unit) scoring.Values {
	if u.Delta == nil {
		return u.Values
	}
	return u.Delta.Values
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

func unitValue(u scoring.Unit, s signal.Signal) string {
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
