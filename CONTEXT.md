# Entropic

Measures the entropy of a codebase so that humans, agents and CI can see when changes make code harder to change.

## Language

**Entropy**:
The degree to which a codebase resists change: growing complexity, duplication, inconsistency, shallow modules and chaotic change patterns.
_Avoid_: Code quality, tech debt, mess

**Signal**:
One measurable kind of entropy: Complexity, Duplication, Scatter, Inconsistency or Shallowness.
_Avoid_: Metric, indicator, check

**Complexity**:
How hard a Unit is to understand: breaks in linear control flow, weighted by how deeply they are nested.
_Avoid_: Cyclomatic complexity, McCabe

**Scatter**:
The number of Modules a single change touches.
_Avoid_: Churn, spread, blast radius

**Scatter Budget**:
The Scatter a change may have without counting as chaotic; it grows slowly with the size of the change.
_Avoid_: Scatter limit, allowance

**Module**:
The coarsest Unit below the repository: a package in Go, a package or directory in Python.
_Avoid_: Component, folder

**Entropy Score**:
A single number summarising all Signals as a weighted combination, normalised by code size so that growth alone does not raise it.
_Avoid_: Total, grade, health

**Unit**:
A piece of code a Signal is measured on: a function, class, file or module. Unit values roll up into their enclosing Units.
_Avoid_: Target, node, entity

**Churn**:
How often a file changed within a recent history window.
_Avoid_: Activity, volatility

**Hotspot**:
A Unit with high entropy that also has high Churn, so its entropy is paid for repeatedly.
_Avoid_: Problem area, red zone

**Baseline**:
The state of the code that a measurement is compared against.
_Avoid_: Reference, previous version

**Delta**:
The change in entropy between the Baseline and the code being measured.
_Avoid_: Diff, regression
