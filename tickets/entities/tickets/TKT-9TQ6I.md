---
id: TKT-9TQ6I
type: ticket
title: 'Re-verify relation-rename versioning against the atomic store.RenameEntity path (post #1127)'
kind: test
priority: medium
effort: s
status: done
---

## Context

TKT-92JL8P's relation-rename version capture was designed when the entitymanager
renamed via `internal/rename.Rename`, which decomposed a rename into
per-relation **delete-old + create-new** — so the new triple got a FRESH
`rel_record_id`, and our `relationLineageIDs` `prev_from`/`prev_to` walk
STITCHED the two lineages.

**#1127 (BUG-5QDV6F, merged into develop) deleted `internal/rename` and routed
`Manager.RenameEntity` through the store's ATOMIC `RenameEntity`** — a single
`UPDATE relations SET from_id=$2 ...`. This changes the versioning shape:

- The relation row keeps the SAME `rel_record_id` across the rename (in-place
UPDATE, not delete+create). So the lineage is already continuous on one id; the
`prev_from`/`prev_to` stitch walk is now REDUNDANT (harmless, finds no
predecessor).
- The atomic `UPDATE relations` does **NOT bump `updated_at`** (entity.go
~L420/426) — the exact RR-N5YK81 concern, now the live path. If the synchronous
rename-version capture fails (best-effort, logged), the sweep cannot back-fill
it because `updated_at` is unchanged.

Manual analysis says the atomic path is actually CLEANER (continuous single
lineage, no fork) and the current tests pass — but this needs proper
verification, not assumption:

## Tasks

1. **Fix the stale test.** `TestRelationVersionRenameStitchesLineage`
(pgstore/relation_version_test.go) manually models the OLD decomposition
(delete-old + create-new + fresh id + a rename row). Production no longer does
that. Either rewrite it to model the atomic path, or add a NEW test that drives
a real `store.RenameEntity` and asserts the resulting lineage (same
`rel_record_id`, one continuous timeline, no duplicate rename row). The comment
at manager.go was already updated to reference the atomic path.
2. **Assert the end-to-end capture** through `Manager.RenameEntity` on pgstore
(not just the memstore unit test): rename an entity with incident relations,
confirm each relation's history shows a `rename` version on the surviving
lineage with correct `prev_from`/`prev_to`, and NO orphaned/forked lineage.
3. **Decide on `updated_at`.** Either bump `updated_at` in the atomic relation
re-key so the sweep can back-fill a missed rename capture (defense in depth for
RR-N5YK81), or document that rename capture is sync-only-best-effort and accept
it (the lineage is continuous regardless, so a missed rename row only loses the
rename MARKER, not history continuity).

## Origin

Surfaced in the TKT-92JL8P follow-up review while checking for merge drift after
#1127 landed on develop. No user-visible bug found in manual analysis; this
ticket makes the "no bug" conclusion test-backed and closes the RR-N5YK81 loop
now that its concern (atomic UPDATE, no updated_at bump) is the live path.
