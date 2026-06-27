---
id: BUG-R8ELTO
type: bug
title: ID generation and delete-safety gate run on partial store-scan data
description: 'collectAllIDs and collectIncidentRelations in entitymanager swallowed iterator errors and returned partial slices. A truncated entity scan let GenerateNextID/GenerateShortID mint an already-existing ID, which upsertEntity''s create-then-update fallback then overwrote. A truncated relation scan let DeleteEntity''s ''has relations?'' gate under-count, allowing a non-cascade delete to orphan relations (the class issue #888 closed at the deletion step, reintroduced at the counting step).'
priority: high
why1: collectAllIDs returned `ids` on iterator error and collectIncidentRelations `continue`d past per-item errors, so both fed safety-critical decisions from silently-incomplete data.
why2: The helpers were written to prefer a partial result over a failure, treating ID generation and the delete gate as best-effort when they are integrity-critical.
why3: The cost of partial data (ID collision/overwrite; orphaned relations) was not weighed against the cost of a hard error when the helpers were written.
why4: Iterator error handling had no convention distinguishing best-effort reads (dedup lookups) from integrity-critical reads (ID allocation, delete gates).
why5: Range-over-func iterators make it easy to drop the error leg silently; nothing flags a swallowed iterator error on a path whose result gates a write.
prevention: collectAllIDs and collectIncidentRelations now return ([]T, error) and propagate iterator errors; generateID and both DeleteEntity callers fail the operation rather than proceed on partial data. Regression tests inject a terminal iterator error and assert create / delete fail loudly; they fail without the fix. findExistingRelationTarget intentionally stays best-effort (cascade dedup, not a safety gate).
status: done
---

## Bug

Found in the 2026-06-09 backend review (write-path theme B3).

Two helpers in `internal/entitymanager/core.go` swallowed `iter.Seq2` errors and
returned partial slices, each feeding a safety-critical decision:

- **`collectAllIDs`** (`core.go:192`) returned whatever it had collected on an iterator error. That partial list feeds `GenerateNextID`/`GenerateShortID` (`core.go:182`) — a truncated scan that misses a high-numbered existing ID makes the generator mint an ID that already exists, and `upsertEntity`'s create-then-update-on-conflict fallback then **overwrites** the existing entity.
- **`collectIncidentRelations`** (`core.go:206`) `continue`d past per-item errors. It feeds `DeleteEntity`'s `totalRelations > 0 && !cascade` gate (`manager.go:474`, `cascadehost.go:116`) — under-counting lets a non-cascade delete proceed and **orphan** the missed relations (issue #888's class, reintroduced at the count step).

## Fix (PR pending)

Both helpers now return `([]T, error)` and propagate the iterator error.
`generateID` fails on an ID-scan error rather than minting a possibly-colliding
ID; both `DeleteEntity` callers return the error before the delete-safety gate
rather than proceeding on partial data.

`findExistingRelationTarget` deliberately stays best-effort (it backs
`if_exists` cascade dedup, not a safety gate) — out of scope.

Regression tests `TestCreate_FailsWhenIDScanErrors` and
`TestDelete_FailsWhenRelationScanErrors` inject a terminal iterator error; both
verified to fail without the fix (create silently succeeded, delete silently
proceeded).
