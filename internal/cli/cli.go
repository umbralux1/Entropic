// Package cli is the entropic command line: it wires flags to the analysis
// pipeline and maps outcomes to exit codes.
package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/language"
)

// Exit codes.
const (
	ExitOK    = 0
	ExitError = 2
)

const schemaVersion = 1

type report struct {
	SchemaVersion int          `json:"schemaVersion"`
	Units         []unitReport `json:"units"`
}

type unitReport struct {
	Kind       analysis.UnitKind `json:"kind"`
	Name       string            `json:"name"`
	Path       string            `json:"path"`
	StartLine  int               `json:"startLine"`
	EndLine    int               `json:"endLine"`
	Complexity int               `json:"complexity"`
}

// Run executes entropic with args in the repository at dir and returns the
// process exit code.
func Run(args []string, dir string, stdout, stderr io.Writer) int {
	flags := flag.NewFlagSet("entropic", flag.ContinueOnError)
	flags.SetOutput(stderr)
	base := flags.String("base", "none", "Baseline revision to compare against, or none for a snapshot")
	format := flags.String("format", "json", "output format: json")
	if err := flags.Parse(args); err != nil {
		return ExitError
	}
	if *base != "none" {
		fmt.Fprintf(stderr, "entropic: --base %s is not supported yet; use --base none\n", *base)
		return ExitError
	}
	if *format != "json" {
		fmt.Fprintf(stderr, "entropic: --format %s is not supported yet; use --format json\n", *format)
		return ExitError
	}

	r, err := snapshot(dir)
	if err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	enc := json.NewEncoder(stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		fmt.Fprintf(stderr, "entropic: %v\n", err)
		return ExitError
	}
	return ExitOK
}

func snapshot(dir string) (report, error) {
	paths, err := trackedFiles(dir)
	if err != nil {
		return report{}, err
	}
	r := report{SchemaVersion: schemaVersion, Units: []unitReport{}}
	for _, path := range paths {
		lang, ok := language.ForPath(path)
		if !ok {
			continue
		}
		source, err := os.ReadFile(filepath.Join(dir, path))
		if errors.Is(err, fs.ErrNotExist) {
			continue // tracked by git but deleted from the working tree
		}
		if err != nil {
			return report{}, err
		}
		units, err := analysis.Analyze(source, lang)
		if err != nil {
			return report{}, fmt.Errorf("%s: %w", path, err)
		}
		for _, u := range units {
			r.Units = append(r.Units, unitReport{
				Kind:       u.Kind,
				Name:       u.Name,
				Path:       path,
				StartLine:  u.StartLine,
				EndLine:    u.EndLine,
				Complexity: u.Complexity,
			})
		}
	}
	return r, nil
}

// trackedFiles lists files git knows about or would track, in slash form,
// relative to dir.
func trackedFiles(dir string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = dir
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("list files: %w: %s", err, strings.TrimSpace(errOut.String()))
	}
	var paths []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}
