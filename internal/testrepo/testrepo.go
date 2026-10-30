// Package testrepo builds throwaway git repositories for tests.
package testrepo

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

// CICheckout clones the repository the way CI checks out a branch: HEAD
// detached at that branch, and main present only as origin/main.
func (r *Repo) CICheckout(branch string) *Repo {
	r.t.Helper()
	clone := &Repo{t: r.t, Dir: r.t.TempDir()}
	clone.git("clone", "-q", r.Dir, clone.Dir)
	clone.git("switch", "-q", "--detach", "origin/"+branch)
	for _, local := range strings.Fields(clone.git("for-each-ref", "--format=%(refname:short)", "refs/heads")) {
		clone.git("branch", "-q", "-D", local)
	}
	return clone
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

// Report is the JSON report as a consumer reads it. Its own figures and
// Units are production code; Tests holds the same for test files.
type Report struct {
	SchemaVersion int             `json:"schemaVersion"`
	Baseline      *ReportBaseline `json:"baseline"`
	ReportGroup
	Tests *ReportGroup `json:"tests"`
}

// ReportGroup is the Entropy Score, Signals and Units of one group of files.
type ReportGroup struct {
	EntropyScore ReportFigure            `json:"entropyScore"`
	Signals      map[string]ReportFigure `json:"signals"`
	Units        []ReportUnit            `json:"units"`
}

// ReportBaseline is the Baseline a report was compared against.
type ReportBaseline struct {
	Ref    string `json:"ref"`
	Commit string `json:"commit"`
}

// ReportDelta is a Unit's change against the Baseline.
type ReportDelta struct {
	Lines      int `json:"lines"`
	Complexity int `json:"complexity"`
}

// ReportFigure is a repository-wide number (a Signal or the Entropy Score) and its Delta.
type ReportFigure struct {
	Delta float64 `json:"delta"`
	Value float64 `json:"value"`
}

// ReportUnit is one Unit in the JSON report.
type ReportUnit struct {
	Kind       string      `json:"kind"`
	Name       string      `json:"name"`
	Path       string      `json:"path"`
	Module     string      `json:"module"`
	StartLine  int         `json:"startLine"`
	EndLine    int         `json:"endLine"`
	Lines      int         `json:"lines"`
	Complexity int         `json:"complexity"`
	Status     string      `json:"status"`
	Delta      ReportDelta `json:"delta"`
	// Informational values are reported but feed no Signal or Entropy Score.
	Informational ReportInformational `json:"informational"`
}

// ReportInformational holds a function Unit's informational values.
type ReportInformational struct {
	Cyclomatic int `json:"cyclomatic"`
}

// Unit returns the Unit of the given kind and name, failing the test if
// there is none.
func (rep ReportGroup) Unit(t *testing.T, kind, name string) ReportUnit {
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
func (rep ReportGroup) UnitsOfKind(kind string) []ReportUnit {
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

// Text fails the test unless the run exited with want, then returns stdout
// as the text report.
func (res Result) Text(want int) string {
	res.t.Helper()
	if res.Code != want {
		res.t.Fatalf("exit code: want %d, got %d; stderr: %s", want, res.Code, res.Stderr)
	}
	return res.Stdout
}

// Branch creates a branch at HEAD and switches to it.
func (r *Repo) Branch(name string) *Repo {
	r.t.Helper()
	r.git("switch", "-q", "-c", name)
	return r
}

// Switch switches to an existing branch.
func (r *Repo) Switch(name string) *Repo {
	r.t.Helper()
	r.git("switch", "-q", name)
	return r
}

// State is git's porcelain status plus the content of every file on disk,
// so tests can prove a run left the working tree as it found it.
func (r *Repo) State() string {
	r.t.Helper()
	state := r.git("status", "--porcelain", "--untracked-files=all")
	err := filepath.WalkDir(r.Dir, func(path string, d os.DirEntry, err error) error {
		switch {
		case err != nil:
			return err
		case d.IsDir() && d.Name() == ".git":
			return filepath.SkipDir
		case d.IsDir():
			return nil
		}
		content, err := os.ReadFile(path)
		state += "\n== " + path + "\n" + string(content)
		return err
	})
	if err != nil {
		r.t.Fatal(err)
	}
	return state
}

// Rev returns the commit a revision points at.
func (r *Repo) Rev(ref string) string {
	r.t.Helper()
	return strings.TrimSpace(r.git("rev-parse", ref))
}

func (r *Repo) git(args ...string) string {
	r.t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = r.Dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		r.t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
