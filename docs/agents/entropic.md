# Acting on Entropic output

How an agent reads Entropic's JSON after an edit and turns it into fixes. Terms are from `CONTEXT.md`; flags are in `go run ./cmd/entropic --help`.

## Pick the Baseline

The Delta is only meaningful against the right Baseline:

- **Uncommitted edits**: run with no `--base`; the Baseline is `HEAD`, so the Delta is your edit alone.
- **Already committed on a branch**: no `--base` compares against the merge-base with `main`. When the branch forked from another branch, pass `--base "$(git merge-base HEAD <that-branch>)"`.
- **Current state only**: `--base none` gives a snapshot: no Delta, no Scatter, never a violation.

## Exit code

- `0`: no threshold exceeded.
- `1`: at least one threshold exceeded; `violations` says which.
- `2`: Entropic itself failed (bad `entropic.toml`, not a git repo, parse failure). Read stderr and fix the cause; the report is absent.

## Work through the report in this order

Done means every violation is fixed or explained in your final message, and every Unit your change made worse has been looked at.

1. **`violations`**: each names a `threshold` (`complexity`, `duplication`, `entropy_score`, `scatter`), its `limit`, the worst `value`, and the `units` that exceed it with path and line range. These are the CI failures; fix them first.
2. **`units`** (production code; test Units live under `tests`): ordered by severity, biggest Delta first. Look at Units with a positive `delta.complexity` or `delta.duplication`, and at `status: added` Units your change introduced. For Complexity, flatten nesting (early returns, extracted helpers) rather than moving the same branches elsewhere. A Unit you did not touch can still show Delta if its file or Module rolled up from one you did.
3. **`cloneGroups`**: each group lists every location of one structural clone (identifiers and literals abstracted). If one location is code you just wrote, reuse or extract the existing code instead of keeping the copy.
4. **`scatter`**: `modules` touched by the change versus the Scatter Budget (`budget`, `overBudget`). Over budget means the change is smeared across Modules; check whether it can be narrowed or split. A mechanical rename can legitimately exceed it; say so rather than contorting the change. `informational` (files, entropy, effectiveModules) is detail only.
5. **`hotspots`**: files ranked by entropy times Churn. Not something your change failed; it tells you where reducing entropy pays off most. Prefer not to add Complexity to a top Hotspot.

Test code (`tests`) is reported separately and carries no thresholds; duplication there is acceptable.

## Text output

`--format text` renders the same report for humans: violations first, then the Baseline, Scatter, Entropy Score, Signals, Units, clone groups and Hotspots. Use JSON when acting on results.
