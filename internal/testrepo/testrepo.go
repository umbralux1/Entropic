// Package testrepo builds throwaway git repositories for tests.
package testrepo

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/umbralux1/entropic/internal/cli"
)

// Repo is a git repository in a temporary directory, removed when the test ends.
type Repo struct {
	t   *testing.T
	Dir string
}

// New creates an empty git repository on branch main.
func New(t *testing.T) *Repo {
	t.Helper()
	r := &Repo{t: t, Dir: t.TempDir()}
	r.git("init", "-q", "-b", "main")
	return r
}

// Write creates or replaces a file, creating parent directories as needed.
func (r *Repo) Write(path, content string) *Repo {
	r.t.Helper()
	full := filepath.Join(r.Dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		r.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		r.t.Fatal(err)
	}
	return r
}

// Remove deletes a file from the working tree without staging the deletion.
func (r *Repo) Remove(path string) *Repo {
	r.t.Helper()
	if err := os.Remove(filepath.Join(r.Dir, path)); err != nil {
		r.t.Fatal(err)
	}
	return r
}

// Commit stages everything and commits it.
func (r *Repo) Commit(message string) *Repo {
	r.t.Helper()
	r.git("add", "-A")
	r.git("commit", "-q", "-m", message)
	return r
}

// Result is what one entropic run produced.
type Result struct {
	t      *testing.T
	Code   int
	Stdout string
	Stderr string
}

// Report is the JSON report as a consumer reads it.
type Report struct {
	SchemaVersion int                     `json:"schemaVersion"`
	EntropyScore  float64                 `json:"entropyScore"`
	Signals       map[string]ReportSignal `json:"signals"`
	Units         []ReportUnit            `json:"units"`
}

// ReportSignal is the repository-wide summary of one Signal.
type ReportSignal struct {
	Value float64 `json:"value"`
}

// ReportUnit is one Unit in the JSON report.
type ReportUnit struct {
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	Path       string `json:"path"`
	Module     string `json:"module"`
	StartLine  int    `json:"startLine"`
	EndLine    int    `json:"endLine"`
	Lines      int    `json:"lines"`
	Complexity int    `json:"complexity"`
}

// Unit returns the Unit of the given kind and name, failing the test if
// there is none.
func (rep Report) Unit(t *testing.T, kind, name string) ReportUnit {
	t.Helper()
	for _, u := range rep.Units {
		if u.Kind == kind && u.Name == name {
			return u
		}
	}
	t.Fatalf("no %s Unit named %q in %+v", kind, name, rep.Units)
	return ReportUnit{}
}

// UnitsOfKind returns the Units of one kind, in report order.
func (rep Report) UnitsOfKind(kind string) []ReportUnit {
	var units []ReportUnit
	for _, u := range rep.Units {
		if u.Kind == kind {
			units = append(units, u)
		}
	}
	return units
}

// Run runs entropic in-process in the repository.
func (r *Repo) Run(args ...string) Result {
	r.t.Helper()
	var out, errOut bytes.Buffer
	code := cli.Run(args, r.Dir, &out, &errOut)
	return Result{t: r.t, Code: code, Stdout: out.String(), Stderr: errOut.String()}
}

// Report fails the test unless the run exited with want, then decodes stdout
// as a JSON report.
func (res Result) Report(want int) Report {
	res.t.Helper()
	if res.Code != want {
		res.t.Fatalf("exit code: want %d, got %d; stderr: %s", want, res.Code, res.Stderr)
	}
	var rep Report
	if err := json.Unmarshal([]byte(res.Stdout), &rep); err != nil {
		res.t.Fatalf("stdout is not a JSON report: %v\n%s", err, res.Stdout)
	}
	return rep
}

func (r *Repo) git(args ...string) {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
