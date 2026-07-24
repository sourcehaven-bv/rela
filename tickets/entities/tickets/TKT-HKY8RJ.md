---
id: TKT-HKY8RJ
type: ticket
title: Extract dataentry write nucleus into writeHandler
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of the [[TKT-R68TV8]] `dataentry.App` decomposition arc (M5.4).
Follows [[TKT-QTPUA]] (attachment cluster).

## What

Moved the 18-method write nucleus off `App` onto a new `writeHandler`
(`internal/dataentry/write_handler.go`): the entity CRUD handlers
(`handleV1CreateEntity`, `handleV1DryRunCreate`, `handleV1UpdateEntity`,
`handleV1DeleteEntity`), the relation CRUD handlers (`handleV1CreateRelation`,
`handleV1UpdateRelation`, `handleV1DeleteRelation`), clone
(`handleV1CloneEntity`), conflict-resolve (`handleV1ConflictResolve` +
`authorizeConflictResolve` + `recordConflictResolveAudit`), the shared
relation-error writers (`writeRelationsValidationError`,
`writeRelationsApplyError`), and the modern relations reconciler
(`relations_modern.go`: `validateRelationsModern`, `collectEdgeWarnings`,
`applyRelationsModern`, `writeCreateRelation`, `writeUpdateRelation` — receiver
change in place; the file IS the reconciler). `resolveConflictPath` demoted to a
package-level function shared with the read-side conflict-detail GET that stays
on `App`.

- **`App`: 131 → 114 methods** — `//plimsoll:max-methods` directive ratcheted
132 → 115 (counts include one test-file helper method).
- **PURE STRUCTURAL extraction** — handlers moved verbatim; concurrency model
untouched. `writeMu` stays owned by `App` and is shared by **pointer**, exactly
like `attachmentHandler`/`syncHandler`, so every write still serializes against
the residual App write paths (Lua actions, webhook) and the sync/attachment
handlers. Replacing the mutex with the store `Tx` contract is the separate
[[DEC-8UIL0]] arc.
- Collaborator shape mirrors `attachmentHandler`: fixed services by value
(`store`/`manager`/`reader`/`serializer`/`affordances`), swappable-in-test deps
as closures over App (`schema`/`acl`/`audit`), and the helpers shared by BOTH
read and write paths as closures (`gateRead`, `denyAfford`, `computeETag`,
`currentEdgesByPeer`) so the two paths cannot drift (uniform-404 read gate,
affordance-denial audit, one ETag definition).
- `rebindApp` (test helper) rebuilds `app.write` mirroring production wiring,
so narrow test fixtures keep working.

## Verification

- `go test -race ./...` (full suite)
- Builds across default / `postgres` / `memorybackend`
- `just plimsoll`, `just arch-lint`, `golangci-lint` (0 issues), `gofmt`
