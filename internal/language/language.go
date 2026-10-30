// Package language knows every supported language adapter.
package language

import (
	"path/filepath"

	"github.com/umbralux1/entropic/internal/analysis"
	"github.com/umbralux1/entropic/internal/language/golang"
)

var all = []analysis.Language{golang.Language}

// ForPath returns the language a file is written in, judged by its extension.
func ForPath(path string) (analysis.Language, bool) {
	ext := filepath.Ext(path)
	for _, lang := range all {
		for _, e := range lang.Extensions {
			if e == ext {
				return lang, true
			}
		}
	}
	return analysis.Language{}, false
}
