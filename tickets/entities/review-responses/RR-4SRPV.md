---
id: RR-4SRPV
type: review-response
title: RenameEntity re-reads via GetEntity post-commit; can skip observer notification
finding: 'cranky-code-reviewer #4: after commit, RenameEntity does `s.GetEntity(ctx, newID)` and calls notifyPut only `if err == nil`. A concurrent delete or any transient query error between commit and this read means the search observer never gets the put for the renamed entity — in-process index missing an entity that exists in the DB. The fully-formed `renamed` entity is already in hand from the RETURNING clause; the re-query is both a correctness hole and an extra round-trip.'
severity: significant
resolution: 'Fixed in entity.go RenameEntity: the rename UPDATE now RETURNs id/type/properties/content/updated_at into `renamed` (scanEntity), and the post-commit observer notification uses that in-hand value (s.notifyPut(renamed)) instead of re-querying via GetEntity. Removes the correctness hole (concurrent delete / transient error skipping the put) and an extra round-trip. Conformance + the mixed-case-rename regression test (search_test.go) still green.'
status: addressed
---

## Resolution plan (fix now)

Notify with the in-hand value from the rename's `RETURNING`:
`s.notifyPut(renamed)` (it has id/type/properties/content/updated_at). Drop the
post-commit `GetEntity` round-trip. Smaller and correct.
