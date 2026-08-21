---
id: TKT-E4I6NX
type: ticket
title: Migration registry holds mutable singletons; SetMetamodel races across concurrent migrate runs
kind: refactor
priority: medium
effort: s
status: backlog
---

## Description

`migration.Register` stores a **single mutable instance** of each migration in a
package-level `registry` slice (`internal/migration/migration.go`). For
`MetamodelAware` migrations the runner then mutates that shared instance via
`SetMetamodel(meta)` on every call (`DetectFromNodeWithMetamodel`).

Two projects migrated concurrently — or parallel tests — can therefore have one
run's metamodel observed by another run's Detect/Apply.

## Why it matters more now

Until recently a stale `m.meta` meant a wrong *detect* (a key stripped or not).
`FormRelationDirectionMigration` (TKT-860BNJ) raises the stakes: it **writes
inferred directions into the user's data-entry.yaml**. Inferring against the
wrong schema writes a wrong `direction:` into operator config — silently binding
the wrong side of a relation, which is the exact failure class TKT-860BNJ exists
to eliminate.

The race detector is on in CI, so this surfaces eventually — but only if a test
happens to migrate two projects concurrently, which none do today.

## Approach sketch

Registry should hold **constructors** (or the runner should clone before use) so
each migrate run gets its own instance and metamodel. Alternatively pass the
metamodel as a parameter to `Detect`/`Apply` rather than stashing it on the
receiver — that removes the mutable state entirely and makes the interface
honest about its inputs.

The second is cleaner but touches every migration's signature; the first is
mechanical. Worth deciding before the next metamodel-aware migration lands.

## Source

Raised as RR-NRYAFC during the TKT-860BNJ code review; deferred there as a
pre-existing pattern affecting all `MetamodelAware` migrations, not a defect
introduced by that ticket.
