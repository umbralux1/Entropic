## Testing

Test at the two seams in `CODING_STANDARDS.md`: Seam 1 through `internal/testrepo`, Seam 2 through `analysis.Analyze`.

## Entropic

Run Entropic on your own work before finishing any change to code: `go run ./cmd/entropic`. With uncommitted edits the Baseline is `HEAD`, so the JSON shows the Delta of your edit alone. Exit 1 means a threshold was exceeded: fix it or say why it is acceptable. Exit 2 means Entropic itself failed. To read the output and act on violations, Units with Delta, clone groups, Scatter and Hotspots, see `docs/agents/entropic.md`.

## Agent skills

### Issue tracker

Issues and specs live in GitHub Issues (umbralux1/Entropic), via `gh`. See `docs/agents/issue-tracker.md`.

### Triage labels

Default five labels: needs-triage, needs-info, ready-for-agent, ready-for-human, wontfix. See `docs/agents/triage-labels.md`.

### Domain docs

Single-context: root `CONTEXT.md` + `docs/adr/`. See `docs/agents/domain.md`.
