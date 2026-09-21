---
id: TKT-5RAW5I
type: ticket
title: 'Drift adoption is silent: route the middle tier to an audit record, not just a log line'
kind: enhancement
priority: medium
effort: s
status: backlog
---

## Problem

The compatibility gate's middle tier — **drift** — adopts the schema change and
emits only a `slog.Warn` per delta:

```go
// internal/datamigration/gate.go
v.Status = StatusAdopted
for _, d := range v.Report.ByTier(metamodel.TierDrift) {
    slog.Warn("datamigration.drift_adopted", "subject", d.Subject, "detail", d.Detail)
}
```

Drift is not cosmetic. It includes **deleted properties, deleted entity types,
deleted enum values, and new required properties** — changes that strand stored
data and start the GC grace clock that will eventually delete it.

A warning logged at process start in a healthy long-running server is
operationally invisible. Nobody reads the startup logs of a process that came up
fine. So the one signal that "your schema just orphaned some data, and it will
be garbage-collected in 30 days" is emitted into a stream nobody is watching.

`grep -n "audit\|Audit" internal/datamigration/gate.go` returns nothing: the
gate writes **no audit records at all**, while the two other raw-store writers
in the same package — `data-migration` and `data-gc` — both do.

## Why this is the weakest part of the tiering

rela's additive/drift/needs-migration tiering maps one-to-one onto GraphQL
Inspector's NON_BREAKING / DANGEROUS / BREAKING, which is good company. But
**every comparable system routes its middle tier to a human**:

| System | Middle tier | What happens |
|---|---|---|
| GraphQL Inspector | DANGEROUS | reported to a human in CI |
| buf | WIRE_JSON / PACKAGE | a check you *fail* |
| Atlas lint | data-dependent (MF*) | blocks or warns in the pipeline |
| Flyway | checksum mismatch | **fatal**, needs explicit `repair` |
| Liquibase | MD5SUM mismatch | **fatal**, needs `clearCheckSums` |
| rela | drift | adopted, one log line |

Flyway and Liquibase are the sharpest comparison: both concluded that "the
recorded state and the actual state disagree" must stop the world, and both ship
an explicit human-invoked override *precisely so the override is auditable*.
rela's drift tier is the same situation with the override applied automatically
and no record that it happened.

This is the one tier where the design invented something rather than borrowing
it, and the thing invented is a silent-by-default data change.

## Options

**A — Audit record (recommended, effort s).** Keep auto-adopting; emit one
`audit.Audit` record per adoption naming the deltas, the old and new shape, and
the ledger entries created. The sink already exists and is already used by
`data-migration`/`data-gc` in this package, so this is wiring, not new
machinery. Preserves today's non-blocking boot behaviour while making the event
reconstructable after the fact.

**B — Refuse, with an explicit adopt command.** `rela migrate adopt` as the
auditable override, mirroring Flyway's `repair`. Strictly safer and matches the
precedent above, but changes boot behaviour for every existing deployment and
turns a currently-silent path into a startup failure. Harder to justify given
drift genuinely cannot break reads.

**C — Surface it in `rela migrate status`.** Complementary to A rather than an
alternative: status should report pending ledger entries and their GC deadlines
so an operator can see what is scheduled for deletion without grepping logs.

Recommend **A + C**. B is the theoretically correct answer, but drift adoption
failing toward retention (the GC grace period is the real safety net) makes the
blocking version poor value for the disruption.

## Note on ordering

If TKT-XCJ0Y2 lands first, the gate's tiering survives unchanged — that ticket
removes `from`/`to` from migration *files* but keeps the stored projection and
the classifier. These two do not conflict; this one can land either side.

## References

- `internal/datamigration/gate.go` — drift adoption, no audit reference
- `internal/datamigration/gc.go` — the grace clock drift starts
- `internal/metamodel/shapecompare.go:119` — `entity_type_removed` is TierDrift
- Surfaced during the migration-system review that produced TKT-XCJ0Y2
