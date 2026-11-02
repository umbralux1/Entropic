# Calibration of defaults

Findings behind the default weights, thresholds, duplicate minimum size and Scatter Budget (issue #14). Terms are from `CONTEXT.md`.

## What was run

Entropic at `5419b13`, provisional defaults (weights complexity 1 / duplication 0.1; thresholds complexity 10, duplication 20, entropy_score 10, scatter 0; duplicate_min_lines 6; Scatter Budget `1 + log2(1 + lines/10)`), on two repositories:

| Repository | Language | Commits replayed | Production functions | Lines of Python/Go |
|---|---|---|---|---|
| Entropic (`main`) | Go | 31 | 131 | ~5k |
| metal (a FastAPI shop) | Python | 104 | 372 | ~13k |

- **Snapshot**: `--base none` at the tip of each repository.
- **Replay**: every non-merge commit checked out in a clone and measured with `--base <commit>^`, so each Delta is one commit's change.

## Observed

### Snapshot

| | Entropic | metal |
|---|---|---|
| Entropy Score | 16.2 | 16.8 |
| Complexity per function p50 / p90 / p95 / p99 / max | 2 / 8 / 12 / 18 / 21 | 1 / 8 / 10 / 23 / 90 |
| Cyclomatic per function p95 / max | 9 / 13 | 10 / 43 |
| Functions with Complexity ≥ 15 | 2 | 10 |
| Clone Groups | 2 | 12 |

The two repositories land on almost the same Entropy Score, though metal has one 90-Complexity function and far more Duplication: with weight 0.1, metal's 399 duplicated lines add 40 to a Weighted Signal Total of ~1140, so Duplication barely moves the Entropy Score.

### Replay: how often each Threshold would fire

Commits with a Violation, per candidate limit (Entropic of 31 / metal of 104):

| Threshold | Candidate limit | Entropic | metal |
|---|---|---|---|
| complexity (per-Unit Delta) | 10 | 4 | 14 |
| | 15 | 1 | 7 |
| | 20 | 1 | 3 |
| duplication (per-Unit Delta, lines) | 10 | 1 | 5 |
| | 20 | 1 | 3 |
| entropy_score (Delta) | 1 | 3 | 3 |
| | 2 | 1 | 2 |
| | 3 | 1 | 0 |
| | 10 | 1 | 0 |
| scatter (Modules over Budget) | 0 | 6 | 4 |
| | 1 | 2 | 3 |
| | 2 | 2 | 0 |

- **complexity 10** fires on 13% of metal commits, mostly on new functions of Complexity 11–23 that are ordinary for a request handler or a seed script. Both repositories' p95 per function sits at 10–12.
- **entropy_score 10** fired once in all 135 commits, on an early Entropic commit when the repository was a few hundred lines and any change swung the density. In a settled repository the largest Delta seen was 2.9.
- **scatter 0** fires on small overshoots (0.1–0.7 Modules) in Entropic, whose Modules are small Go packages; a feature in Entropic typically touches CLI, scoring and a reporter at once.

### Duplication

Sweep of `duplicate_min_lines` on metal's snapshot:

| min lines | Clone Groups | Duplication |
|---|---|---|
| 4 | 20 | 479 |
| 5 | 15 | 459 |
| 6 | 12 | 399 |
| 8 | 7 | 332 |
| 10 | 2 | 60 |

Most of metal's Duplication (320 of 399 lines) is in `app/catalog`, and most of that is in data: rows of dict literals in `demo_catalog.py` and `seed.py`, structurally identical once literals are set aside. These are Clone Groups by the definition, but nobody would extract them. Migrations also contribute.

## Decisions

| Setting | Was | Now | Why |
|---|---|---|---|
| `[thresholds] complexity` | 10 | 15 | Fires on 7% of metal commits and 3% of Entropic's instead of 13%; a new function of Complexity 11–15 is ordinary. |
| `[thresholds] entropy_score` | 10 | 3 | 10 never fired in a settled repository; 3 catches the largest rises seen (2.8–2.9 in metal sit just under it). |
| `[thresholds] scatter` | 0 | 1 | A change may touch one Module beyond its Scatter Budget; the overshoots of 0.1–0.7 came from small Go packages, not chaotic changes. |
| `[weights] duplication` | 0.1 | 0.5 | At 0.1, 399 duplicated lines moved metal's Entropy Score by 3%. At 0.5 Duplication is visible in the score without outweighing Complexity. |
| `[thresholds] duplication` | 20 | 20 | Fired only on large copies (28, 48 and 200 lines). |
| `duplicate_min_lines` | 6 | 6 | 8 already drops real clones; 10 drops almost all. |
| Scatter Budget | `1 + log2(1 + lines/10)` | unchanged | With the allowance of 1 it flags only changes that touch several Modules more than their size explains. |

Consequences:

- **Small repositories.** The Entropy Score is a density per 100 lines, so in a repository of a few dozen lines nearly any change rises past 3. The fixtures in Entropic's own tests are that small; tests about anything but thresholds lift `entropy_score` in their `entropic.toml`. A young real repository can do the same until it grows.
- **Data literals.** Rows of literals in seed and demo data count as Clone Groups, inflating Duplication (most of metal's). Weight 0.5 makes that more visible, so it is tracked as its own issue (#16) rather than tuned away here.
