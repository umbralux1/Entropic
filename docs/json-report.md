# JSON report schema

The JSON Entropic writes to stdout with `--format json` (the default). Terms are from `CONTEXT.md`. Field names are camelCase; `entropic.toml` keys are snake_case (see [Names](#names)).

## Versioning

`schemaVersion` changes only when a field is removed or renamed, or its meaning changes. Adding a field does not change it, so consumers must ignore fields they do not know.

| Version | Change |
|---|---|
| 1 | First release. |
| 2 | `hotspots[].entropy` renamed `weightedTotal`; hotspots reference their Unit (`kind`, `name`, `path`, `startLine`, `endLine`); `scatter.informational.entropy` renamed `shannonEntropy`. Added `skippedAtBaseline` and `cloneGroups[].locations[].original`. Duplication counts only additional copies. |

## Top level

| Field | Type | Meaning |
|---|---|---|
| `schemaVersion` | int | Version of this schema: `2`. |
| `baseline` | Baseline or `null` | What the report is compared against; `null` for a Snapshot. |
| `skippedAtBaseline` | string[] | Paths left out of the Baseline because they do not parse there (each also warned about on stderr). Empty in a Snapshot. |
| `entropyScore` | Figure | Production code's Entropy Score: Weighted Signal Total per 100 non-blank lines. |
| `signals` | object of Figure | Production code's total per Signal of code, keyed `complexity`, `duplication`. |
| `units` | Unit[] | Production Units, in severity order (below). |
| `cloneGroups` | CloneGroup[] | Production Clone Groups, by first location. |
| `tests` | Group | `entropyScore`, `signals`, `units`, `cloneGroups` for Test code, as above. Test code has no Thresholds. |
| `churnWindowDays` | int | How many days of history Churn counts. |
| `hotspots` | Hotspot[] | Production files with both a Weighted Signal Total and Churn, highest `score` first. |
| `scatter` | Scatter | Absent in a Snapshot. |
| `violations` | Violation[] | Thresholds exceeded; empty when none (exit 0) and always in a Snapshot. |

## Types

**Baseline**: `ref` (string, how it was chosen: `HEAD`, `merge-base(HEAD, main)`, or the `--base` given), `commit` (full SHA).

**Figure**: `value` (number), `delta` (number; absent in a Snapshot).

**Location**: `path` (repository-relative, slash-separated), `startLine`, `endLine` (1-based, inclusive). A Module has `path` only; the repository neither.

**Unit**: a Location plus

| Field | Type | Meaning |
|---|---|---|
| `kind` | string | `function`, `class`, `file`, `module`, `repository`. |
| `name` | string | Function or class name (methods qualified: `T.M`, `Class.method`); the path for a file or Module; `""` for the repository. |
| `module` | string | Module of a function, class or file. |
| `lines` | int | Non-blank lines. |
| `complexity` | int | Complexity. A class, file, Module or repository rolls up its functions. |
| `duplication` | int | Non-blank lines lying in additional copies of a Clone Group. |
| `status` | string | Unit Status: `added`, `removed`, `changed`, `unchanged`. Absent in a Snapshot. Files renamed since the Baseline keep their Units. |
| `delta` | object | `lines`, `complexity`, `duplication` change since the Baseline. Absent in a Snapshot. |
| `informational` | object | Functions only: `cyclomatic` (Cyclomatic Complexity). Feeds nothing and does not roll up. |

Severity order: by `kind` (function, class, file, module, repository); within a kind, by the Delta weighted as in the Entropy Score, largest first (by the weighted values in a Snapshot); then by weighted value; ties keep source order. Text output lists Units in the same order.

**CloneGroup**: `locations`, each a Location plus `original` (bool). Exactly one location per group is the original: the one already there at the Baseline, or the first by path and line in a Snapshot. Only the others count as `duplication`.

**Hotspot**: `kind` (`file`), `name`, `path`, `startLine`, `endLine` of the file Unit, plus `churn` (commits in the window, following renames), `weightedTotal` (the file's Weighted Signal Total) and `score` (`weightedTotal × churn`).

**Scatter**: Scatter of production code changed since the Baseline (Test code and generated, vendored or excluded files left out).

| Field | Type | Meaning |
|---|---|---|
| `modules` | int | Modules touched: the Scatter Signal. |
| `touchedModules` | string[] | Their names, sorted. |
| `changedLines` | int | Lines added plus deleted. |
| `budget` | number | Scatter Budget: `1 + log2(1 + changedLines / 10)`. |
| `overBudget` | bool | `modules > budget`. |
| `informational` | object | `files` (files touched), `shannonEntropy` (bits, of changed lines across Modules; unrelated to the Entropy Score), `effectiveModules` (`2^shannonEntropy`). |

**Violation**: `threshold` (name below), `limit`, `value` (the worst Delta measured against the limit), `units` (the Units responsible, worst first: each `kind`, `name` and Location).

## Names

| Threshold | JSON `threshold` | `entropic.toml` `[thresholds]` key | Default | Judged on |
|---|---|---|---|---|
| Complexity | `complexity` | `complexity` | 10 | Complexity Delta of each function |
| Duplication | `duplication` | `duplication` | 20 | Duplication Delta of each file |
| Entropy Score | `entropyScore` | `entropy_score` | 10 | Entropy Score Delta |
| Scatter | `scatter` | `scatter` | 0 | `modules − budget` |

Signal weights in `entropic.toml`'s `[weights]` use the Signal names `complexity` (default 1) and `duplication` (default 0.1).
