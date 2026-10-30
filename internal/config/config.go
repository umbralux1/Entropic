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
	"slices"
	"strings"

	"github.com/BurntSushi/toml"

	"github.com/umbralux1/entropic/internal/signal"
)

// FileName is the config file Entropic looks for at the repository root.
const FileName = "entropic.toml"

// Config is what a run is configured with.
type Config struct {
	// Weights weigh each Signal of code in the Entropy Score.
	Weights map[signal.Signal]float64 `toml:"weights"`
	// Exclude lists slash-separated glob patterns (path.Match syntax) of
	// paths not to measure. A pattern matching a directory excludes
	// everything below it.
	Exclude []string `toml:"exclude"`
	// ChurnWindowDays is how far back Churn looks.
	ChurnWindowDays int `toml:"churn_window_days"`
	// DuplicateMinLines is the smallest duplicate worth reporting.
	DuplicateMinLines int `toml:"duplicate_min_lines"`
	// Thresholds are the limit of each threshold, keyed by its
	// entropic.toml key (signal.Threshold.Key).
	Thresholds map[string]float64 `toml:"thresholds"`
}

// Default is the configuration used when there is no config file. Values
// are provisional until tuned on real repositories.
func Default() Config {
	thresholds := map[string]float64{}
	for _, t := range signal.Thresholds {
		thresholds[t.Key] = t.DefaultLimit
	}
	return Config{
		Weights:           signal.DefaultWeights(),
		ChurnWindowDays:   90,
		DuplicateMinLines: 6,
		Thresholds:        thresholds,
	}
}

// Limit is how far a change may go before exceeding threshold t.
func (c Config) Limit(t signal.Threshold) float64 {
	return c.Thresholds[t.Key]
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
	for name, w := range cfg.Weights {
		if !slices.Contains(signal.OfCode, name) {
			return fmt.Errorf("weights: unknown Signal %q (known: %s)", name, joined(signal.OfCode, func(s signal.Signal) string { return string(s) }))
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
	for key, limit := range cfg.Thresholds {
		known := slices.ContainsFunc(signal.Thresholds, func(t signal.Threshold) bool { return t.Key == key })
		if !known {
			return fmt.Errorf("thresholds: unknown threshold %q (known: %s)", key, joined(signal.Thresholds, func(t signal.Threshold) string { return t.Key }))
		}
		if limit < 0 {
			return fmt.Errorf("thresholds: %s must not be negative, got %v", key, limit)
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

// joined lists the names of items, comma-separated.
func joined[T any](items []T, name func(T) string) string {
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = name(item)
	}
	return strings.Join(names, ", ")
}
