---
id: BUG-2W3AJ
type: bug
title: DeleteEntity silently swallows I/O errors on relation cleanup
description: |-
    `Manager.DeleteEntity` and `cascadeHost.DeleteEntity` looped over a deleted entity's incident relations and, on a non-`ErrNotFound` error from `DeleteRelation`, executed `continue` — no log, no audit record, no error returned. Execution then fell through to deleting the entity anyway. Result: the entity is removed while one or more of its relations are left behind — an inconsistent store state, silently. The store has no transaction, so there is no rollback.

    A secondary quirk: when a relation was already gone (`ErrNotFound`), the loop still emitted a "deleted" audit record for a deletion that never happened.

    **Fix (fail-secure, per ISMS direction):**
    - **Store (`fsstore.DeleteEntity`):** the cascade path stopped swallowing relation-file removal errors. A real `Remove` error now aborts before the entity file is touched and before the in-memory index is mutated, so the entity is never removed while a relation could not be — the store holds its lock for the whole op. (Still not transactional: a relation file removed before a later failure stays removed; but the entity is never orphaned-from.)
    - **Manager + cascadeHost:** replaced the per-relation delete loop with a single `store.DeleteEntity(cascade)` call, then emit one audit record per relation the store reports deleting (with the `cascade:delete-entity:<id>` triggered-by) plus the entity record. This removes the duplicated cleanup logic, keeps full per-relation audit, surfaces real errors to callers instead of swallowing them, and eliminates the phantom `ErrNotFound` record.

    **Behavior change (intended):** a relation that genuinely cannot be removed now surfaces as a failed delete to HTTP / MCP / CLI / Lua / cascade callers, rather than a silent "success" with an orphan left behind.
priority: medium
effort: m
why1: A non-ErrNotFound error from DeleteRelation in the cascade loop was handled with `continue` — silently skipped, with no log, no audit, no returned error.
why2: The loop's intent was best-effort cleanup (delete what you can), but it also fell through to deleting the entity regardless, so a partial failure left the entity gone and a relation orphaned.
why3: The Manager re-implemented relation cleanup as its own loop (instead of using the store's atomic cascade) specifically so it could emit a per-relation audit record — the store cascade emits none — which is why the swallow lived at the Manager layer.
why4: Even the store's own cascade ignored relation-file removal errors (`_ = s.rooted.Remove(...)`), so delegating alone wouldn't have been fail-secure — both layers swallowed.
why5: There is no transactional delete primitive and no single chokepoint guaranteeing "an entity is only removed once all its relations are"; consistency on multi-file deletes was by-convention, and the convention silently degraded on I/O error.
prevention: |-
    Two regression tests pin the fix:

    - `TestDeleteEntity_RelationRemoveError_FailsSecure`
      (`internal/store/fsstore/recovery_test.go`) injects a relation-file
      Remove failure via storage.ErrorFS and asserts the cascade delete
      errors AND leaves both the entity and the relation in place — never
      orphaned.
    - `TestDeleteEntity_PropagatesStoreError` and
      `TestDeleteEntity_CascadeAuditsReportedRelations`
      (`internal/entitymanager/manager_delete_test.go`) assert the Manager
      surfaces a store delete error instead of swallowing it, and that a
      successful cascade emits one delete-relation audit record per reported
      relation (with the cascade triggered-by) plus the delete-entity record.

    storage.ErrorFS gained a configurable Remove fault (RemoveError /
    RemoveErrorOn) so future store error-path tests have a reusable way to
    inject filesystem failures.
status: done
---

See GitHub issue #888 (also referenced as BUG-C20T).
