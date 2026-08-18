---
id: TKT-69TDK8
type: ticket
title: 'collectAllIDs scans every entity with content on each auto-ID create: O(n) latency and no fault isolation'
kind: refactor
priority: high
effort: m
status: backlog
---

## Problem

Every `create_entity` for an auto-ID type loads **every entity in the store,
including full markdown content**, to compute the new ID
(`internal/entitymanager/core.go:207`):

```go
existingIDs, err := collectAllIDs(ctx, deps.Store)
if err != nil {
    return "", fmt.Errorf("collect existing IDs: %w", err)
}
if entityDef.IsShortID() {
    return entity.GenerateShortID(existingIDs, prefix, len(existingIDs), entityDef.GetIDCaps()), nil
}
return entity.GenerateNextID(existingIDs, prefix), nil
```

The scan happens **before** the branch, so it costs the same whether the type
uses short or sequential IDs. Since `GetIDType()` defaults to `short`, this is
the common path for nearly every create in a typical project — see Fix
direction, where this reverses the priorities this ticket was first written with.

`collectAllIDs` iterates `ListEntities` and keeps only `e.ID`, but pgstore's
`buildEntityListSQL` selects `id, type, properties, content, updated_at` with no
`LIMIT`. At 262k entities the scan streams **535 MB per create** to extract a
few hundred KB of ID strings.

## Measured impact

Two distinct defects, both confirmed by reproduction against pgstore.

### 1. Latency — O(n) per create

| Dataset | Uncontended create | Under 80-way concurrency |
|---------|--------------------|--------------------------|
| ~1k entities | 2-6 ms | — |
| 11.7k entities | 34-169 ms | — |
| 262k entities | 0.74 s | **22 s** |

At 113k entities, 300 simultaneous creates produced a **1 m 50 s** max latency.
The pgx pool caps concurrent scans at `max(4, NumCPU)`, so requests queue rather
than running in parallel — the failure mode is timeouts, not memory. Peak RSS
stayed at 53-55 MB throughout, because `collectAllIDs` discards each entity's
content as the iterator advances.

This makes it the *inverse* of the `_analyze` bug (see Related): same full scan,
opposite memory profile, because that one retains the slice and this one does
not. Worth stating explicitly in the fix so the two are not conflated.

### 2. No fault isolation

A single malformed entity file breaks **every create in the project**.
Encountered live while filing these tickets:

```
collect existing IDs: failed to parse frontmatter:
  yaml: line 4: mapping values are not allowed in this context
```

One committed file with an unquoted colon in a YAML value
(`tickets/entities/automated-measures/AM-date-property-write-roundtrip.md`,
fixed separately) made the tickets project read-only. The blast radius of a
corrupt file should be that file, not all writes.

## Fix direction

### Short IDs are the primary path, not an afterthought

`GetIDType()` **defaults to `IDTypeShort`** when `id_type` is unset, so short is
both the declared majority and the implicit default. In `tickets/schema.yaml`,
17 of 24 types are `short` and 7 are `manual` (which skips generation entirely).
Across the whole repo `id_type: sequential` appears **only in
`prototypes/data-entry/project/schema.yaml`** — no shipped schema uses it.

So the sequential path this ticket was originally written around is the rare
case, and `GenerateShortID` is what actually runs on nearly every create. Fix
short IDs first; sequential is the secondary case.

### Short IDs need almost nothing from the store

```go
existingIDs, err := collectAllIDs(ctx, deps.Store)   // O(n), full content
...
return entity.GenerateShortID(existingIDs, prefix, len(existingIDs), caps), nil
```

`GenerateShortID` uses the full id set for exactly two things:

1. `calculateIDLength(entityCount)` — birthday-paradox thresholds at
   500 / 1500 / 10000 / 50000. This needs a **count**, and only its bucket. A
   `COUNT(*)` is an indexed aggregate; even an approximate count would do, since
   the thresholds are order-of-magnitude heuristics.
2. A membership set for collision retry (100 attempts, lengthening periodically).

Point 2 does not need a materialised set at all. `store.CreateEntity` already
returns `ErrConflict` on a duplicate id, and `createCore` already maps that to
`ErrEntityAlreadyExists` — so **retry-on-conflict at the write** is strictly more
correct than pre-checking against a snapshot, which is racy by construction:
between the scan and the insert another writer can take the id. The current
membership check does not prevent collisions, it only makes them rarer.

Target shape: generate a candidate from a cheap count, attempt the insert, and
on `ErrConflict` regenerate (lengthening as now). This removes the scan from the
common path entirely and closes the TOCTOU window at the same time.

### Sequential IDs: an indexed aggregate, with two traps

Secondary in priority, but the scan must go here too. Computing "highest
sequence number for prefix P" is an indexed aggregate — O(log n), one row to the
client — behind a store capability type-asserted like `HistoryReader`, with the
existing scan as the generic fallback.

**Trap 1 — `max(id)` is lexicographic and mints duplicates.**
`ExtractHighestNumber` compares `id.Number` **numerically**; SQL `max(id)`
compares **lexicographically**. Today's `%03d` padding makes the two agree below
1000, but padding does not truncate: the 1000th entity is `TKT-1000`, and
`'TKT-1000' < 'TKT-999'`. A `max(id)` implementation passes every small-fixture
test and starts minting duplicates at exactly 1000 entities of one prefix. The
aggregate must extract and cast the numeric part, e.g.

```sql
SELECT max((substring(id from '^TKT-(\d+)$'))::int) FROM entities
```

ignoring ids that do not match, mirroring `ParseEntityID`'s skip-on-error.

**Trap 2 — scope by PREFIX, not by entity type.** Narrowing with
`EntityQuery{Type: entityType}` looks free but is unsafe: `GetIDPrefixes`
returns a *list*, `MatchesID` scans it, and nothing forbids two types sharing a
prefix. `ExtractHighestNumber` deliberately filters by **prefix across all
types**. Scoping to one type would mint duplicates wherever a prefix is shared.

### `EntityHeader` is the fallback, not the fix

`store.EntityHeader` / `ListEntityHeaders` removes the body bytes but keeps the
O(n) row scan, and still fetches and unmarshals the `properties` JSONB per row
for a function that reads only `e.ID`. It is the right generic fallback for
backends that cannot express the aggregates, but it does not reach AC1 or AC3.

### Fault isolation is resolved as a side effect

`collectAllIDs`'s doc comment justifies failing loudly because *a partial scan
can hide a high-numbered ID*. That is exactly right for a scan — and dissolves
under an aggregate, which never returns a partial. An unparseable entity file
cannot corrupt a `COUNT(*)` or a `max()` over the `id` column: the id is intact
whether or not the frontmatter parses. So the aggregates remove the "one corrupt
file makes the project read-only" defect without needing a fail-open/fail-closed
policy call. The header path would *not* fix it — it still runs the file through
the parser and still fails the whole iterator.

The policy decision remains only for the fallback scan, which must keep
fail-loudly semantics.

## Scope

**In scope**

- **Short-ID generation (primary)**: replace the full scan with a cheap count
plus retry-on-`ErrConflict` at the write.
- **Sequential-ID generation (secondary)**: prefix-scoped, numeric max-sequence
store capability.
- `EntityHeader` for the fallback read path, so no fallback fetches bodies.
- Document fault-isolation behaviour for a malformed entity on the fallback path.
- Benchmark create latency at 1k / 10k / 100k entities, before and after.

**Out of scope**

- The `_analyze` retention fix (landed in TKT-1ESTYJ; this consumes its
`EntityHeader` type for the fallback path).
- Changing the birthday-paradox thresholds or the short-ID alphabet.

## Acceptance criteria

1. Create latency is flat in entity count for **short-ID** types (the default):
   creating entity #100,000 costs approximately what #100 costs.
2. The same holds for sequential-ID types.
3. No entity body is transferred from the store during ID generation, verified by
query inspection or a store-level assertion.
4. Uncontended create at 262k entities drops from 0.74 s to well under 50 ms.
5. 80-way concurrent creates no longer produce multi-second latencies.
6. **Short-ID collisions are resolved at the write, not by pre-check**: a
concurrent create that loses the race retries and succeeds rather than
surfacing `ErrEntityAlreadyExists` to the caller.
7. **Numeric, not lexicographic** (sequential): crossing 999 → 1000 mints
`TKT-1000` then `TKT-1001`, never a duplicate.
8. **Prefix-scoped, not type-scoped** (sequential): two entity types sharing an
`id_prefix` never receive the same generated id.
9. A malformed entity file no longer fails creates of *other* entities when the
aggregate path is available; documented, tested behaviour on the fallback path.

## Test plan

- **Latency benchmark** at 1k / 10k / 100k entities asserting sub-linear scaling,
run for a **short-ID type** (the default path) — this is the test that would
have surfaced the defect.
- **Short-ID concurrency test** — the pin for AC6. N concurrent creates of one
short-ID type; assert N distinct ids and zero surfaced conflicts. Should fail
against the current pre-check, which is racy between scan and insert.
- **Short-ID length test**: the id length still follows the count thresholds
(500 / 1500 / 10000 / 50000) once the count comes from an aggregate.
- **999 → 1000 boundary test** — the pin for AC7. Seed a prefix to `TKT-0999`,
create, assert `TKT-1000`, then `TKT-1001`. Fails against `max(id)`.
- **Shared-prefix test** — the pin for AC8. Two types declaring the same
`id_prefix`; interleaved creates produce strictly increasing distinct ids.
- Error-propagation test: an injected store error during ID generation fails the
create; no ID is minted.
- Malformed-entity test: with the aggregates available, a corrupt file does not
block creates of other entities; on the fallback path, pin the chosen behaviour.
- `storetest` conformance for the new capabilities, plus explicit fallback
coverage (fsstore/memstore exercise the fallback, pgstore the native path).

**In scope**

- Implement an indexed max-sequence store capability, scoped by prefix, with a
scan fallback for backends that cannot express it.
- Remove content from the ID-generation fallback read path (via `EntityHeader`).
- Short-ID generation (see below) — either solved here or split into its own
ticket before this one is called done, but not left unstated.
- Document the fault-isolation behaviour for a malformed entity on the fallback path.
- Benchmark create latency at 1k / 10k / 100k entities, before and after.

**Short IDs need a different capability.** `entityDef.IsShortID()` takes the
`GenerateShortID` path, which needs *both* a count (for `calculateIDLength`'s
birthday-paradox thresholds) *and* the full existing-ID set as a membership
check for collision retry. A max-aggregate does nothing for either. It wants
`COUNT(*)` plus a per-candidate existence probe. Note `store.CreateEntity`
already returns `ErrConflict`, so retry-on-conflict against the store is a
plausible and much cheaper design than materialising the id set. Left
unaddressed, a short-ID type keeps the full scan and this ticket only half-lands.

**Out of scope**

- The `_analyze` retention fix (landed in TKT-1ESTYJ; this consumes its
`EntityHeader` type for the fallback path).

## Acceptance criteria

1. Create latency is flat in entity count for sequential-ID types: creating entity
   #100,000 costs approximately what #100 costs.
2. No entity body is transferred from the store during ID generation, verified by
query inspection or a store-level assertion.
3. Uncontended create at 262k entities drops from 0.74 s to well under 50 ms.
4. 80-way concurrent creates no longer produce multi-second latencies.
5. Generated IDs remain collision-free — the existing "partial scan must not feed the
generator" invariant holds on the fallback path, with a test that a store error
during ID generation fails the create rather than minting a possibly-duplicate ID.
6. **Numeric, not lexicographic**: crossing the 999 → 1000 boundary for a prefix
mints `TKT-1000` then `TKT-1001`, never a duplicate of an existing id.
7. **Prefix-scoped, not type-scoped**: two entity types sharing an `id_prefix`
never receive the same generated id.
8. A malformed entity file no longer fails creates of *other* entities when the
aggregate path is available; documented, tested behaviour on the fallback path.

## Test plan

- **Latency benchmark** at 1k / 10k / 100k entities asserting sub-linear scaling
(this is the test that would have surfaced the defect).
- **999 → 1000 boundary test** — the pin for AC6. Seed a prefix up to `TKT-0999`,
create, assert `TKT-1000`; create again, assert `TKT-1001`. This test fails
against a `max(id)` implementation and is the reason it exists.
- **Shared-prefix test** — the pin for AC7. Two entity types declaring the same
`id_prefix`; interleaved creates must produce strictly increasing distinct ids.
- Collision test: concurrent creates of the same type must not produce duplicate IDs.
- Error-propagation test: an injected store error during ID generation fails the
create; no ID is minted.
- Malformed-entity test: with the aggregate available, a corrupt file does not
block creates of other entities; on the fallback path, pin whichever behaviour
is chosen.
- `storetest` conformance for the new capability, plus explicit fallback coverage
(fsstore/memstore exercise the fallback, pgstore the native path).

## Related

- Analyze OOM ticket — introduces `store.EntityHeader` / `ListEntityHeaders`; this is
its second consumer.
- Scheduler amplification bug — the leak it caused made every subsequent create slower
via this path, so the two defects compounded in production.
