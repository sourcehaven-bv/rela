---
id: BUG-TT73D0P
type: bug
title: Seed script on the fs backend hits the 30 s rela script timeout
description: Each auto-id create reads every entity file and each index write does an extra synchronous persist / so a seed of a few hundred entities exceeds the script timeout on fs.
priority: medium
effort: m
why1: Each create in the seed got slower as the store grew / and the script hit the fixed 30 s timeout of rela script after about 220 entities.
why2: Minting an auto id lists every entity with ListEntities(AllFaces). fsstore reads and parses each entity file for that / so N creates cost N squared file reads. Each index write also did a second synchronous persist of Index.LastModified.
why3: entitymanager only had the generic entity listing to get ids. No store capability returned ids alone / although fsstore keeps them in memory. LastModified lost its only reader when the startup check moved to the backfill watermark / but its write stayed.
why4: Performance work on id minting (TKT-69TDK8) focused on pgstore and was not done. Nothing measured write throughput of a script on fs.
why5: No budget test pins the store reads of a create / and removing a reader does not flag the writer it leaves behind.
prevention: TestCreate_GeneratesIDFromIDListing pins that id minting uses the id listing and does no full scan. TestEntityIDs_MatchesAllFacesScan pins that the listing equals a full scan. BUG-UR0520H tracks the remaining fsync cost and the fixed script timeout.
started: "2026-10-30"
completed: "2026-10-30"
status: done
---

## Description

Running a seed script with `rela script` on the fs backend fails with `context
deadline exceeded` after the fixed 30 s script timeout. Reported by the GRC demo
agent for `scripts/seed.lua` (341 entities, 508 relations).

## Reproduction

Copy the demo config without `.rela/` (so the fs backend is used) and run `rela
script --run-as system:seed scripts/seed.lua`. On develop the script stops after
about 220 entities.

## Cause

Two costs per write that do not need to be there:

1. Every auto-id create lists all entities with `ListEntities(AllFaces)` to
mint the next id. fsstore reads and parses every entity file for that, so N
creates cost N² file reads.
2. Every bleve index write also persisted `Index.LastModified` with a
separate synchronous `SetInternal`. Nothing reads that value any more; the
startup check uses the backfill watermark.

## Fix

- entitymanager prefers an optional `EntityIDs` store capability when
minting ids. fsstore answers it from its in-memory index.
- Remove `Index.LastModified` and its per-write persist. Correct the
`Close` comment, which claimed writes use `unsafe_batch`.

## Measured (full seed, 30 min timeout build, loaded machine)

| build | wall | user+sys CPU |
| --- | --- | --- |
| develop | 49-82 s | 9-10 s |
| fix | 41-52 s | 5.8-6.3 s |
| fix + `unsafe_batch` (not shipped) | 30 s | 5.2 s |

The remaining wall time is fsync: one per markdown file, one bleve persist per
entity, the audit record. On this loaded machine the seed still exceeds 30 s
with the fix. Follow-up: BUG-UR0520H. The broader redesign for all backends and
short ids stays open as TKT-69TDK8.
