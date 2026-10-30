# Go with tree-sitter for parsing

Entropic is written in Go and parses analysed code with tree-sitter, even though the first target language is Python. Go was chosen over Rust for fast compile times (a tight TDD feedback loop), simple idioms that agents write reliably, and single-binary distribution for CI and agent hooks. Go has no native Python parser, so tree-sitter is used from day one; this costs a cgo/C toolchain dependency but makes adding further languages a grammar swap rather than a rewrite.

## Considered Options

- **Python + stdlib `ast`**: strongest Python parsing, but slower, harder to distribute, and Python-only.
- **Rust + tree-sitter or ruff's parser**: fastest at scale, but slow compiles and borrow-checker friction slow the agent loop; raw speed is not the bottleneck for this tool.
