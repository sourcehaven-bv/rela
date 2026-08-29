---
id: TKT-Y683LJ
type: ticket
title: Extract fileLayout (and mdCodec) off fsstore.FSStore (plimsoll ratchet 95 → ~81)
kind: refactor
priority: medium
effort: m
status: done
---

Sub-ticket of [[TKT-N0IKN9]], opening the `fsstore.FSStore` arc (95 methods / 33
exported; the exported surface is largely the mandated `store.Store` interface,
so the ratchet target is the unexported set).

## What

**Pure structural extraction, no behavior change, no `store.Store` surface
change, no locking/event-ordering change.**

Step 1 — `fileLayout`: immutable path/key resolver holding `{entitiesKey,
relationsKey, attachKey, schemas, rooted}` with `entityFileKey`,
`relationFileKey`, `propertyOrder`, `absPath` (from fsstore.go) plus
`buildPluralToTypeMap`, `resolveEntityType` (from index.go). Pure function of
config fixed at `New`; no lock, no mutable state, no events. −6.

Step 2 (same PR, separate commit, only if step 1's gates are green) — `mdCodec`:
the markdown read/write seam from markdown.go (`readEntityFile`,
`writeEntityFile`, `readRelationFile`, `writeRelationFile`,
`buildInaccessibleEntity`, `buildInaccessibleRelation`, `readDataFile`,
`writeDataFile`) over `{rooted, layout}`. Touches no mutable index state and no
lock; the git-crypt inaccessible-shell path moves with it (pinned by gitcrypt
tests). −8.

Explicit do-not-touch (from the design survey): `tx.go` (DEC-8UIL0 pairing),
`emit`/observer notification (called under `mu.Lock`; ordering pinned by
storetest), the watcher/reconciler, `notifyRenamed` single-event semantics.

Ratchet `//plimsoll:max-methods` 95 → 89 (step 1) → 81 (step 2).

## Done when

plimsoll with lowered directive; `go test ./internal/store/...` (the storetest
conformance harness + fuzz seeds + gitcrypt/recovery/persistence tests) and the
full suite green; arch-lint/comment-lint/lint clean.
