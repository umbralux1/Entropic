# Entropic

Measures the entropy of a codebase so that humans, agents and CI can see when changes make code harder to change.

## Language

### Entropy and its Signals

**Entropy**:
The degree to which a codebase resists change: growing complexity, duplication, inconsistency, shallow modules and chaotic change patterns. Used only in this sense; the Shannon entropy Scatter reports is called Shannon entropy.
_Avoid_: Code quality, tech debt, mess

**Signal**:
One measurable kind of entropy: Complexity, Duplication, Scatter, Inconsistency or Shallowness.
_Avoid_: Metric, indicator, check

**Complexity**:
How hard a Unit is to understand: breaks in linear control flow, weighted by how deeply they are nested.
_Avoid_: Cyclomatic complexity, McCabe

**Cyclomatic Complexity**:
The number of independent paths through a function. Informational only: it is not Complexity, not a Signal, and feeds no Entropy Score, Delta or Threshold.
_Avoid_: Complexity (for this value), McCabe score

**Duplication**:
The lines of code that lie in additional copies of a Clone Group; the original adds none.
_Avoid_: Copy-paste, redundancy

**Clone Group**:
Every location of one fragment of code that is structurally identical once names and literals are set aside. One location is the original: the one already there at the Baseline, or the first by path and line in a Snapshot.
_Avoid_: Duplicate set, clone pair

**Scatter**:
The number of Modules a single change touches. A Signal of a change, not of code: it is measured between the Baseline and the working tree, not on Units, so it is not part of the Entropy Score.
_Avoid_: Churn, spread, blast radius

**Scatter Budget**:
The Scatter a change may have without counting as chaotic; it grows slowly with the size of the change.
_Avoid_: Scatter limit, allowance

**Weighted Signal Total**:
A Unit's Signal values, each multiplied by its weight, added up.
_Avoid_: Entropy (for this value), score

**Entropy Score**:
A single number summarising all Signals of code as a Weighted Signal Total, normalised by code size so that growth alone does not raise it.
_Avoid_: Total, grade, health

### What is measured

**Unit**:
A piece of code a Signal is measured on: a function, class, file, Module or the whole repository. Unit values roll up into their enclosing Units.
_Avoid_: Target, node, entity

**Module**:
The coarsest Unit below the repository: a package in Go, a package or directory in Python.
_Avoid_: Component, folder

**Test code**:
Files a language's conventions mark as tests. They are measured as a separate group with its own Units and Entropy Score, and never affect production Signals, Scatter or Thresholds.
_Avoid_: Specs, test suite

### Change over time

**Baseline**:
The state of the code that a measurement is compared against.
_Avoid_: Reference, previous version

**Snapshot**:
A measurement without a Baseline: no Delta, no Scatter, no Violations.
_Avoid_: Absolute run, full scan

**Delta**:
The change in entropy between the Baseline and the code being measured.
_Avoid_: Diff, regression

**Unit Status**:
How a Unit compares with the Baseline: added, removed, changed (some Delta) or unchanged. A renamed file keeps its Units.
_Avoid_: Change type, state

**Churn**:
How often a file changed within a recent history window.
_Avoid_: Activity, volatility

**Hotspot**:
A Unit with a high Weighted Signal Total that also has high Churn, so its entropy is paid for repeatedly.
_Avoid_: Problem area, red zone

### Judging a change

**Threshold**:
The most a change may raise one figure (a Unit's Complexity or Duplication, the Entropy Score, or Scatter beyond its Budget).
_Avoid_: Limit, gate, rule

**Violation**:
A Threshold a change exceeded, with the Units responsible.
_Avoid_: Failure, error, breach
