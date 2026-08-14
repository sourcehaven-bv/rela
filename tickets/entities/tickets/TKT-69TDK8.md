---
id: TKT-69TDK8
type: ticket
title: 'collectAllIDs scans every entity with content on each create: O(n) latency and no fault isolation'
kind: refactor
priority: high
effort: m
status: backlog
---

## Problem

Every `create_entity` for a sequential-ID type loads **every entity in the
store, including full markdown content**, to compute the next ID number
(`internal/entitymanager/core.go:207`):

```go
existingIDs, err := collectAllIDs(ctx, deps.Store)
...
return entity.GenerateNextID(existingIDs, prefix), nil
```

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

**Do not fetch content.** This is the natural first consumer of
`store.EntityHeader` / `ListEntityHeaders` from the analyze-memory ticket —
headers carry ID, type and properties, never the body. That alone removes the
535 MB transfer.

Better still, do not scan at all. Computing "max sequence number for prefix P"
is a `SELECT max(...) WHERE id LIKE 'P-%'` — an indexed aggregate, O(log n), no
rows to the client. That likely wants a store capability (`NextSequentialID` or
similar), type-asserted like `HistoryReader`, with the existing scan as the
generic fallback for backends that cannot express it.

Note the correctness constraint documented on `collectAllIDs`: a *partial* scan
must never feed ID generation, because a truncated list can hide a high-numbered
existing ID and the generator would mint a duplicate. Any replacement must
preserve fail-loudly-on-error semantics. `createCore` rejects a collision with
`ErrEntityAlreadyExists`, so the failure mode is a spurious conflict rather than
data loss — but the invariant should be kept, not weakened.

Fault isolation is a separate decision that needs an explicit call: should a
malformed entity file be skipped-with-warning during ID generation, or continue
to fail closed? Failing closed is defensible (it is the current documented
intent), but today it fails closed on *all writes* rather than on the affected
entity, which is almost certainly not intended.

## Scope

**In scope**

- Remove content from the ID-generation read path.
- Evaluate and, if viable, implement an indexed max-sequence store capability with a
scan fallback.
- Decide and document the fault-isolation behaviour for a malformed entity.
- Benchmark create latency at 1k / 10k / 100k entities, before and after.

**Out of scope**

- Short-ID generation strategy (`entityDef.IsShortID()` takes a different path through
`GenerateShortID` and needs the count, not the max — assess separately).
- The `_analyze` retention fix (separate ticket; this one consumes its `EntityHeader`
type if that lands first).

## Acceptance criteria

1. Create latency is flat in entity count for sequential-ID types: creating entity
   #100,000 costs approximately what #100 costs.
2. No entity body is transferred from the store during ID generation, verified by
query inspection or a store-level assertion.
3. Uncontended create at 262k entities drops from 0.74 s to well under 50 ms.
4. 80-way concurrent creates no longer produce multi-second latencies.
5. Generated IDs remain collision-free — the existing "partial scan must not feed the
generator" invariant holds, with a test that a store error during ID generation
fails the create rather than minting a possibly-duplicate ID.
6. Documented, tested behaviour for a malformed entity file during ID generation.

## Test plan

- **Latency benchmark** at 1k / 10k / 100k entities asserting sub-linear scaling
(this is the test that would have surfaced the defect).
- Collision test: concurrent creates of the same type must not produce duplicate IDs.
- Error-propagation test: an injected store error during ID generation fails the
create; no ID is minted.
- Malformed-entity test pinning whichever fault-isolation behaviour is chosen.
- `storetest` conformance for any new capability, plus explicit fallback coverage.

## Related

- Analyze OOM ticket — introduces `store.EntityHeader` / `ListEntityHeaders`; this is
its second consumer.
- Scheduler amplification bug — the leak it caused made every subsequent create slower
via this path, so the two defects compounded in production.
