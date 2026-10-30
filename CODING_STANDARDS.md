# Coding Standards

Judgement calls for review. Mechanical rules live in `.golangci.yml` and CI, not here.

## Domain vocabulary

Name types, functions, JSON fields and tests with the terms in `CONTEXT.md`. A concept missing from the glossary is either invented language (rename it) or a gap (flag it for `/domain-modeling`).

## Language knowledge stays in adapters

Everything a programming language contributes (grammar, which syntax nodes form Units, Complexity rules, file extensions) belongs in its adapter. Code outside the adapters is language-agnostic: adding a language is a new adapter plus one registry entry. `depguard` enforces the import side; review enforces the rest (no `if lang.Name == "go"` branches).

## Tests live at the two seams

- **Seam 1**: the CLI run in-process against a temporary git repo, through `internal/testrepo`. Assert on the JSON report and exit code.
- **Seam 2**: `analysis.Analyze` on a source snippet, only for per-language Complexity rules.

Expected values come from an independent source (a worked example, the spec, a hand count in a comment), never recomputed the way the code does. A test outside these seams needs a reason in the review.

## Every value belongs to a Unit

Each Signal value is attached to the Unit it was measured on; repository-level numbers are roll-ups of Unit values, so consumers can always trace a number back to code.
