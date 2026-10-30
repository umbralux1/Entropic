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

	"github.com/umbralux1/entropic/internal/duplication"
	"github.com/umbralux1/entropic/internal/scoring"
	"github.com/umbralux1/entropic/internal/signal"
)

// JSON writes r as the versioned JSON report, Units in severity order.
func JSON(w io.Writer, r scoring.Report) error {
	r = r.InSeverityOrder()
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(r)
}

// Text writes r as a terminal-friendly report holding what the JSON does:
// the Baseline, Scatter, the Entropy Score and Signal summaries, the Units
// and clone groups, and the Hotspots; test files follow in a section of
// their own. Threshold violations, if any, come first.
func Text(w io.Writer, r scoring.Report) error {
	r = r.InSeverityOrder()
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
			fmt.Fprintf(&b, "  informational: %d files, Shannon entropy %s bits, %s effective Modules\n",
				s.Informational.Files, number(s.Informational.ShannonEntropy), number(s.Informational.EffectiveModules))
		}
		if len(r.SkippedAtBaseline) > 0 {
			fmt.Fprintf(&b, "Skipped at Baseline, not parsable: %s\n", strings.Join(r.SkippedAtBaseline, ", "))
		}
		b.WriteString("\n")
	}
	if err := group(&b, r.Group); err != nil {
		return err
	}
	hotspots(&b, r.Hotspots, r.ChurnWindowDays)
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
			fmt.Fprintf(b, "    %s %s %s\n", u.Kind, u.Name, u.Location)
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
	for _, u := range g.Units {
		fmt.Fprintf(tw, "  %s\t%s\t%s\t", u.Kind, u.Name, u.Location)
		for _, s := range signal.OfCode {
			fmt.Fprintf(tw, "%s %s\t", s, unitValue(u, s))
		}
		fmt.Fprintf(tw, "%s\n", u.Status)
	}
	if err := tw.Flush(); err != nil {
		return err
	}
	cloneGroups(b, g.CloneGroups)
	return nil
}

// cloneGroups writes every location of each clone group, marking the original.
func cloneGroups(b *strings.Builder, groups []duplication.CloneGroup) {
	if len(groups) == 0 {
		return
	}
	b.WriteString("\nClone groups\n")
	for i, g := range groups {
		fmt.Fprintf(b, "  %d.", i+1)
		for _, l := range g.Locations {
			fmt.Fprintf(b, " %s", l.Location)
			if l.Original {
				b.WriteString(" (original)")
			}
		}
		b.WriteString("\n")
	}
}

// hotspots writes the Hotspots, highest score first.
func hotspots(b *strings.Builder, hs []scoring.Hotspot, windowDays int) {
	if len(hs) == 0 {
		return
	}
	fmt.Fprintf(b, "\nHotspots (Churn over %d days)\n", windowDays)
	for _, h := range hs {
		fmt.Fprintf(b, "  %s %s: weighted total %s, churn %d, score %s\n",
			h.Kind, h.Location, number(h.WeightedTotal), h.Churn, number(h.Score))
	}
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
