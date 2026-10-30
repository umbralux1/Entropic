// Package config loads the optional entropic.toml at the repository root,
// merges it with the defaults and validates it.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/BurntSushi/toml"
)

// FileName is the config file Entropic looks for at the repository root.
const FileName = "entropic.toml"

// Config is what a run is configured with.
type Config struct {
	// Weights weigh each Signal, by name, in the Entropy Score.
	Weights map[string]float64 `toml:"weights"`
	// Exclude lists slash-separated glob patterns (path.Match syntax) of
	// paths not to measure. A pattern matching a directory excludes
	// everything below it.
	Exclude []string `toml:"exclude"`
	// ChurnWindowDays is how far back Churn looks.
	ChurnWindowDays int `toml:"churn_window_days"`
	// DuplicateMinLines is the smallest duplicate worth reporting.
	DuplicateMinLines int `toml:"duplicate_min_lines"`
	// Thresholds bound how much a change may raise entropy.
	Thresholds Thresholds `toml:"thresholds"`
}

// Thresholds bound the Delta of a change; exceeding any is a violation.
type Thresholds struct {
	// Complexity is the most Complexity any one function may gain.
	Complexity float64 `toml:"complexity"`
	// EntropyScore is the most the Entropy Score may rise.
	EntropyScore float64 `toml:"entropy_score"`
	// Scatter is how many Modules a change may touch beyond its Scatter Budget.
	Scatter float64 `toml:"scatter"`
}

// Default is the configuration used when there is no config file. Values
// are provisional until tuned on real repositories.
func Default() Config {
	return Config{
		Weights:           map[string]float64{"complexity": 1},
		ChurnWindowDays:   90,
		DuplicateMinLines: 6,
		Thresholds:        Thresholds{Complexity: 10, EntropyScore: 10, Scatter: 0},
	}
}

// Load reads FileName from the repository at dir, merged over the defaults,
// and rejects anything it does not understand.
func Load(dir string) (Config, error) {
	cfg := Default()
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, err
	}
	meta, err := toml.Decode(string(data), &cfg)
	if err != nil {
		return Config{}, fmt.Errorf("%s: %w", FileName, err)
	}
	if err := validate(cfg, meta); err != nil {
		return Config{}, fmt.Errorf("%s: %w", FileName, err)
	}
	return cfg, nil
}

func validate(cfg Config, meta toml.MetaData) error {
	if undecoded := meta.Undecoded(); len(undecoded) > 0 {
		keys := make([]string, len(undecoded))
		for i, k := range undecoded {
			keys[i] = k.String()
		}
		return fmt.Errorf("unknown key %s", strings.Join(keys, ", "))
	}
	known := Default().Weights
	for name, w := range cfg.Weights {
		if _, ok := known[name]; !ok {
			return fmt.Errorf("weights: unknown Signal %q (known: %s)", name, strings.Join(sortedKeys(known), ", "))
		}
		if w < 0 {
			return fmt.Errorf("weights: %s must not be negative, got %v", name, w)
		}
	}
	for _, pattern := range cfg.Exclude {
		if _, err := path.Match(pattern, ""); err != nil {
			return fmt.Errorf("exclude: invalid pattern %q: %w", pattern, err)
		}
	}
	if cfg.ChurnWindowDays <= 0 {
		return fmt.Errorf("churn_window_days must be positive, got %d", cfg.ChurnWindowDays)
	}
	for name, limit := range map[string]float64{
		"complexity":    cfg.Thresholds.Complexity,
		"entropy_score": cfg.Thresholds.EntropyScore,
		"scatter":       cfg.Thresholds.Scatter,
	} {
		if limit < 0 {
			return fmt.Errorf("thresholds: %s must not be negative, got %v", name, limit)
		}
	}
	if cfg.DuplicateMinLines <= 0 {
		return fmt.Errorf("duplicate_min_lines must be positive, got %d", cfg.DuplicateMinLines)
	}
	return nil
}

// Excluded reports whether a slash-separated, repository-relative path, or
// any directory above it, matches an exclude pattern.
func (c Config) Excluded(p string) bool {
	for ; p != "." && p != "/"; p = path.Dir(p) {
		for _, pattern := range c.Exclude {
			if ok, _ := path.Match(pattern, p); ok {
				return true
			}
		}
	}
	return false
}

func sortedKeys(m map[string]float64) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
