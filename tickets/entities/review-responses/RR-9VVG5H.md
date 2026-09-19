---
id: RR-9VVG5H
type: review-response
title: Two hasMigrations probes disagreed on the safety-critical un-baselined decision
finding: appbuild's probe used loader.List + IsMigrationFileName (a NAME check); the CLI's used loadDataMigrations (a full LoadDir PARSE). They diverge in two ways that matter. A directory holding one malformed migration makes the CLI's probe return an error - propagated out of classifyUnrecorded, failing the whole command - while appbuild's returns true. And LoadDir errors on a YAML file with a bad NAME by design, so the CLI probe can fail where appbuild succeeds. The un-baselined decision is safety-critical and two implementations of 'does this project have migrations' is one too many.
severity: significant
resolution: 'Fixed: a single exported datamigration.HasMigrations does the name-only check, and both wiring sites call it. Its doc states why it must not parse - a full parse lets one malformed file turn ''does this project have migrations at all?'' into an error, which the gate would surface as a refusal to classify, converting a recoverable file error into a stuck store.'
status: addressed
---

## Finding

Two implementations of the same question, feeding a safety-critical decision.

- **appbuild** (`migstate.go`): `loader.List` + `IsMigrationFileName` — a *name*
check.
- **CLI** (`migrate_data.go`): `loadDataMigrations` — a full `LoadDir` **parse**.

They diverge in two ways that matter:

1. A directory holding one malformed migration makes the CLI's probe return an
error — propagated out of `classifyUnrecorded`, failing the whole command —
while appbuild's returns `true`.
2. `LoadDir` errors on a YAML file with a bad *name* by design (that refusal is
deliberate and tested), so the CLI probe can fail where appbuild succeeds.

The probe decides `StatusUnbaselined`, which is the guard against both
data-corruption paths in this review. Two answers to "does this project have
migrations?" is one too many.

## Severity

Significant. Not itself a corruption path, but it makes the guard's behaviour
depend on which entry point asked — and one of the two turns a recoverable file
error into a stuck store.

## Resolution

A single exported `datamigration.HasMigrations(fsys fs.FS)` does the name-only
check, and both wiring sites call it. Its doc states why it must not parse:

> A full parse would let one malformed migration answer "I cannot tell" to the
> question "does this project have migrations at all", turning a recoverable file
> error into a refusal to classify.

Both call sites now read through the same `fs.FS` seam, so a project whose
config lives in a SQLite file answers the same way as one on disk.
