// Package revisions reads code from git: the working tree, any commit, and
// which commit is the Baseline. It never modifies the working tree.
package revisions

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// --base values with special meaning.
const (
	// None asks for a snapshot without a Baseline.
	None = "none"
	// Auto picks HEAD when the working tree has uncommitted changes, and the
	// merge-base with main otherwise.
	Auto = ""
)

const mainBranch = "main"

// File is a file's content at some revision. Path is slash-separated and
// relative to the repository root.
type File struct {
	Path   string
	Source []byte
}

// Baseline is the commit a measurement is compared against. Ref says how it
// was chosen.
type Baseline struct {
	Ref    string
	Commit string
}

// Resolve turns a --base value into a Baseline, or nil for None.
func Resolve(dir, base string) (*Baseline, error) {
	switch base {
	case None:
		return nil, nil
	case Auto:
		if _, err := git(dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
			return nil, nil // no commits yet: nothing to compare against
		}
		status, err := git(dir, "status", "--porcelain")
		if err != nil {
			return nil, err
		}
		if len(bytes.TrimSpace(status)) > 0 {
			return Resolve(dir, "HEAD")
		}
		return mergeBaseWithMain(dir)
	}
	commit, err := git(dir, "rev-parse", "--verify", "--quiet", base+"^{commit}")
	if err != nil {
		return nil, fmt.Errorf("unknown --base %q: %w", base, err)
	}
	return &Baseline{Ref: base, Commit: strings.TrimSpace(string(commit))}, nil
}

// mergeBaseWithMain finds where HEAD forked from main, using origin/main when
// there is no local main (as in a CI checkout).
func mergeBaseWithMain(dir string) (*Baseline, error) {
	var lastErr error
	for _, main := range []string{mainBranch, "origin/" + mainBranch} {
		commit, err := git(dir, "merge-base", "HEAD", main)
		if err == nil {
			return &Baseline{Ref: "merge-base(HEAD, " + main + ")", Commit: strings.TrimSpace(string(commit))}, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("find merge-base with %s (pass --base to choose a Baseline): %w", mainBranch, lastErr)
}

// WorkingTree returns the files git tracks or would track, as they are on
// disk, keeping only those want accepts.
func WorkingTree(dir string, want func(path string) bool) ([]File, error) {
	out, err := git(dir, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	if err != nil {
		return nil, fmt.Errorf("list files: %w", err)
	}
	var files []File
	for _, path := range splitNUL(out) {
		if !want(path) {
			continue
		}
		source, err := os.ReadFile(filepath.Join(dir, path))
		if errors.Is(err, fs.ErrNotExist) {
			continue // tracked by git but deleted from the working tree
		}
		if err != nil {
			return nil, err
		}
		files = append(files, File{Path: path, Source: source})
	}
	return files, nil
}

// AtCommit returns the files in a commit, keeping only those want accepts.
func AtCommit(dir, commit string, want func(path string) bool) ([]File, error) {
	out, err := git(dir, "ls-tree", "-r", "-z", "--name-only", commit)
	if err != nil {
		return nil, fmt.Errorf("list files at %s: %w", commit, err)
	}
	var files []File
	for _, path := range splitNUL(out) {
		if !want(path) {
			continue
		}
		source, err := git(dir, "cat-file", "blob", commit+":"+path)
		if err != nil {
			return nil, fmt.Errorf("read %s at %s: %w", path, commit, err)
		}
		files = append(files, File{Path: path, Source: source})
	}
	return files, nil
}

// Churn counts, per file path at HEAD, the non-merge commits reachable from
// HEAD that changed the file and were committed after since. Changes made
// under an earlier name count towards the file's name at HEAD.
func Churn(dir string, since time.Time) (map[string]int, error) {
	churn := map[string]int{}
	if _, err := git(dir, "rev-parse", "--verify", "--quiet", "HEAD"); err != nil {
		return churn, nil // no commits yet: no history
	}
	out, err := git(dir, "log", "-z", "--format=", "--name-status", "-M", "--no-merges",
		"--since="+since.Format(time.RFC3339), "HEAD")
	if err != nil {
		return nil, fmt.Errorf("read history for Churn: %w", err)
	}
	// Walk newest first, remembering which later name each earlier name became.
	renamedTo := map[string]string{}
	nameAtHead := func(path string) string {
		if later, ok := renamedTo[path]; ok {
			return later
		}
		return path
	}
	fields := splitNUL(out)
	for i := 0; i < len(fields); i++ {
		status := strings.TrimSpace(fields[i])
		if strings.HasPrefix(status, "R") && i+2 < len(fields) {
			from, to := fields[i+1], nameAtHead(fields[i+2])
			churn[to]++
			renamedTo[from] = to
			i += 2
			continue
		}
		if i+1 < len(fields) {
			churn[nameAtHead(fields[i+1])]++
			i++
		}
	}
	return churn, nil
}

func git(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var errOut bytes.Buffer
	cmd.Stderr = &errOut
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w: %s", args[0], err, strings.TrimSpace(errOut.String()))
	}
	return out, nil
}

func splitNUL(out []byte) []string {
	var parts []string
	for _, p := range strings.Split(string(out), "\x00") {
		if p != "" {
			parts = append(parts, p)
		}
	}
	return parts
}
