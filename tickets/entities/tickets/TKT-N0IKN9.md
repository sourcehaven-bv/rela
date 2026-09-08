---
id: TKT-N0IKN9
type: ticket
title: Decompose god-object types flagged by plimsoll (App, Runtime, FSStore, Server, CLI)
kind: refactor
priority: medium
effort: l
status: backlog
---

We added the [plimsoll](https://github.com/sourcehaven-bv/plimsoll) god-object
linter (caps method/exported-field count per type) to CI. Existing offenders are
grandfathered with `//plimsoll:max-*` directives pinned to their CURRENT count,
so they can't grow. This ticket tracks ratcheting those numbers down.

## Why this happened

Adding the Nth method to an existing struct is frictionless; spinning up a
focused new type is work — so every feature accreted onto the nearest big type.
Nothing failed when `App` grew its 200th method. (Root cause from the sync
read-ACL review: the same "convention, not enforcement" gap.) plimsoll is the
structural brake; this ticket is the cleanup of the debt that accrued before it.

## Current offenders (develop e0187047, 2026-09-04, directives stripped)

| Type | Now | Line | Directive |
| --- | --- | --- | --- |
| `dataentry.App` | 87 methods | 40 | app.go:172 |
| `fsstore.FSStore` | 92 methods / 36 exported | 40 / 20 | fsstore.go:159-160 |
| `lua.Runtime` | 46 methods | 40 | runtime.go:102 |
| `metamodel.Metamodel` | 32 exported | 20 | types.go:22 |
| `metamodel.EntityDef` | 24 exported | 20 | types.go:278 |
| `appbuild.Services` | 30 exported | 20 | appbuild.go:100 |
| `cli.CLI` | 47 exported fields | 20 | kong.go:70 |
| `dataentryconfig.Config` | 22 exported fields | 20 | config.go:93 |
| `memstore` / `pgstore` / `sqlitestore` | 50/32, 51/42, 53/32 | required-interface exception | — |

Dead directives (type already under the line): `internal/docs/runtime.go:87`
(29) and `internal/mcp/server.go:214` (25).

**Ratchet regression.** The worlds feature (#1452) RAISED directives instead of
holding them: fsstore 81→92 / 33→36, memstore 43→50, sqlitestore 43→53, pgstore
49→52 / 39→42, app 86→87. Exported bumps are legitimate (`store.Store` grew);
the total-method bumps are the ratchet being defeated by a large feature PR. A
directive bump in a feature PR should be a review flag.

## Plan (2026-09-04) — one PR per ticket, real abstractions only

Survey principle: extract a type only where a **closed field set** exists that
callers can only reach through a narrow contract, or where the type makes a
prose rule structural. Moving methods to free functions to duck the counter
(pgstore/entity.go:872-875 did this) is explicitly NOT the goal.

**dataentry.App arc** (sub-epic [[TKT-R68TV8]], 87 → ~45; order = safest first):
1. [[TKT-XDJTDC]] `configAPI` — the principal-independent metadata surface; no
store/reader/ACL fields, making "config is not a secret" structural.
2. [[TKT-NLX424]] `nextActionAPI` — suggestion adapter; fixes the unsynchronised
`SetUserState`/`SetNextActionMatchers` writes with a mutex.
3. [[TKT-K9GL4J]] `liveFeed` — one owner for reload/SSE lifecycle (Start/Stop),
consumer-side `storeSubscriber`.
4. [[TKT-CU105Y]] `entityAPI` — the v1 read surface that EXCLUSIVELY holds
`reader`/`visibleReader`/`serializer`/`worldNeighbors`, so the compiler enforces
what `TestWorldCapableRoutesDoNotUseUngatedReader` asserts.
`scopedSortedEntities` (5 consumers) stays on App; a `listPipeline` type is the
possible sixth step, decided after these four.

**lua.Runtime arc** (46 → 29; parallelisable):
- [[TKT-M3X3JO]] `graphReads` — read bindings with the ACL reader as the SOLE
handle (clears the line alone).
- [[TKT-V9NKZY]] `graphWrites` — mutation + `bypass_acl` elevation, writer-only.

**fsstore.FSStore arc** (92 → ~66 unexported; exported floor is 31 =
`store.Store`):
0. [[BUG-S24X52]] self-echo observer never wired in production since #508 —
FIX FIRST so the extractions don't bake a dead field in.
1. [[TKT-9XDEY0]] `attachmentStore` (closed set: attachments/attachKey/streaming).
2. [[TKT-8FXP1B]] `fsIndexLoader` (pure config→data, like `fileLayout`).
3. [[TKT-W9E1NC]] `metaIndex` — map+order pairing as one mutation; removes the
comparator-mismatch bug class documented at fsstore.go:452-472.
4. [[TKT-ONFXVS]] `storeutil.ObserverSet` (cross-backend; needs-design; two
surveys disagreed — recorded on the ticket). Known floor: the 24 `tx.go`
wrappers are structural duplication (fs/mem); a shared `SubscriberSet` is out of
scope because the four `emit`s have deliberately different timing guarantees.
Extracted types take NO lock.

**metamodel** (32 → 18, 24 → 13; both directives deleted):
- [[TKT-OBC7QI]] `migration.SchemaAdapter` in the CONSUMER package (the
interface was already right; the impl landed on the producer) + delete dead
`IsEnumType`.
- [[TKT-EK1JO8]] `SchemaOutput`/`EntityOutput` views (AttachmentPolicy shape),
kills 4 `any` returns and 2 redundant cli interfaces; delete 4 dead EntityDef
methods. `IDPolicy` view is the next step only if EntityDef grows.

**appbuild.Services** (30 → 28, then documented exception):
- [[TKT-R3SMEK]] `ScheduledMailer` — scheduler behaviour off the wiring facade;
restore `FieldRedactor()` (the cap had turned it into a field); convert the
directive to a required-seam exception. Sub-bundles (`Read()`/`Write()`) are
REJECTED: they break every consumer-side interface bound on `*Services`.
Follow-up idea: `dataentry.Deps` struct for the 12-arg `NewApp`.

**cli.CLI** (47 → 7 fields): [[TKT-YJDD7C]] `embed:""` command groups (verified
syntax-preserving) + replace the package-level invocation globals with a bound
`Globals` — the latter is the real fix.

**Housekeeping**: [[TKT-RKR7VX]] dead directives, stale prose (copylist.go:91
cites a directive Manager doesn't have; worldneighbors.go:85 cites 104), and the
two settled exceptions below.

## Settled: not offenders to fix

- **Store backends.** Exported surplus over `store.Store` + optional
capabilities: memstore 0, sqlitestore 1, fsstore 3, pgstore 7 — all consumed
through narrow consumer-side interfaces; versioning already sits behind
`VersionServiceProvider`. `//plimsoll:max-exported-methods` on stores is a
required-interface exception, not a ratchet target.
- **`dataentryconfig.Config`.** Read-only format mirror of `data-entry.yaml`;
`yaml:",inline"` grouping works but is arbitrary and costs 131 call sites. Keep
the directive, document why.

## History

- `dataentry.App`: 227 → 87 across [[TKT-N26KLB]] / [[TKT-R68TV8]] sub-tickets
(`visibleReader` ACL seam, sync/command/attachment/write/views handlers).
- `lua.Runtime`: 119 → 46 ([[TKT-4WBLG6]], [[TKT-DOPCTI]]).
- `fsstore.FSStore`: 95 → 81 ([[TKT-Y683LJ]] fileLayout + mdCodec), then 92
after #1452.
- `mcp.Server`: 49 → 25 ([[TKT-YUETL7]] + round 2). Directive now dead.
- `metamodel.Metamodel`: `AttachmentPolicy` extraction held the line at 30
when attachments landed — the ratchet working as intended.
- `cli.cliServices`: removed entirely ([[TKT-45QYI]]).
- `output.Writer`: directive deleted ([[TKT-NS3XPE]]).
