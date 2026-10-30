// Package golang is the language adapter for Go.
package golang

import (
	"regexp"

	sitter "github.com/tree-sitter/go-tree-sitter"
	tsgo "github.com/tree-sitter/tree-sitter-go/bindings/go"

	"github.com/umbralux1/entropic/internal/analysis"
)

// Language tells analysis how to read Go.
var Language = analysis.Language{
	Name:       "go",
	Grammar:    sitter.NewLanguage(tsgo.Language()),
	Extensions: []string{".go"},
	Functions: map[string]bool{
		"function_declaration": true,
		"method_declaration":   true,
	},
	Branches: map[string]bool{
		"if_statement":  true,
		"for_statement": true,
	},
	VendorDirs: []string{"vendor"},
	TestPaths:  []string{"*_test.go"},
	// The convention from "go help generate".
	Generated: regexp.MustCompile(`(?m)^// Code generated .* DO NOT EDIT\.$`),
}
