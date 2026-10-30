# Acting on Entropic output

How an agent reads Entropic's JSON after an edit and turns it into fixes. Terms are from `CONTEXT.md`; flags are in `go run ./cmd/entropic --help`; every JSON field is described in `docs/json-report.md` (schemaVersion 2).

Names: the JSON uses camelCase (`entropyScore`), `entropic.toml` uses snake_case (`entropy_score`). The table in `docs/json-report.md#names` maps each threshold between the two.

## Pick the Baseline

The Delta is only meaningful against the right Baseline:

- **Uncommitted edits**: run with no `--base`; the Baseline is `HEAD`, so the Delta is your edit alone.
- **Already committed on a branch**: no `--base` compares against the merge-base with `main`. When the branch forked from another branch, pass `--base "$(git merge-base HEAD <that-branch>)"`.
- **Current state only**: `--base none` gives a snapshot: no Delta, no Scatter, never a violation.

## Exit code

- `0`: no threshold exceeded.
- `1`: at least one threshold exceeded; `violations` says which.
- `2`: Entropic itself failed (bad `entropic.toml`, not a git repo, a file in the working tree that does not parse). Read stderr and fix the cause; the report is absent.

A file that does not parse at the Baseline is not a failure: it is skipped with a warning on stderr and listed in `skippedAtBaseline`, and the run goes on.

## Work through the report in this order

Done means every violation is fixed or explained in your final message, and every Unit your change made worse has been looked at.

1. **`violations`**: each names a `threshold` by its JSON name (`complexity`, `duplication`, `entropyScore`, `scatter`; in `entropic.toml` the keys are `complexity`, `duplication`, `entropy_score`, `scatter`), its `limit`, the worst `value`, and the `units` that exceed it with path and line range. These are the CI failures; fix them first.
2. **`units`** (production code; test Units live under `tests`): ordered by severity, the Delta weighted as in the Entropy Score, biggest first, so a large Duplication Delta can come before a small Complexity one. A file renamed since the Baseline keeps its Units, so a pure rename shows no Delta. Look at Units with a positive `delta.complexity` or `delta.duplication`, and at `status: added` Units your change introduced. For Complexity, flatten nesting (early returns, extracted helpers) rather than moving the same branches elsewhere. A Unit you did not touch can still show Delta if its file or Module rolled up from one you did.
3. **`cloneGroups`**: each group lists every location of one structural clone (identifiers and literals abstracted). The location marked `original` was there at the Baseline and adds no Duplication; the others are the copies. If a copy is code you just wrote, reuse or extract the original instead of keeping the copy.
4. **`scatter`**: production Modules touched by the change versus the Scatter Budget (`budget`, `overBudget`); test files do not count. Over budget means the change is smeared across Modules; check whether it can be narrowed or split. A mechanical move across Modules can legitimately exceed it; say so rather than contorting the change. `informational` (files, shannonEntropy, effectiveModules) is detail only.
5. **`hotspots`**: file Units ranked by `weightedTotal` (Weighted Signal Total) times Churn. Not something your change failed; it tells you where reducing entropy pays off most. Prefer not to add Complexity to a top Hotspot.

Test code (`tests`) is reported separately, carries no thresholds and does not count towards Scatter; duplication there is acceptable.

## Text output

`--format text` renders the same report for humans, Units in the same order: violations first, then the Baseline, Scatter with its informational values, files skipped at the Baseline, Entropy Score, Signals, Units, clone groups (original marked) and Hotspots. Use JSON when acting on results.
