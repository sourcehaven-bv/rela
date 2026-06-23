---
id: BUG-WXFZO6
type: bug
title: Entity audit record skipped when post-write step (cascade/upsert) fails
description: Manager.CreateEntity and UpdateEntity recorded the entity audit row only after the automation cascade ran. A failure between the durable store write and that point — a failed post-automation re-write (reachable via any transient store error) or a cascade hard-error (latent) — left a committed on-disk entity with no audit record, breaking the 'every successful write is audited' invariant.
priority: medium
why1: recordEntityAudit was called after Cascade.Process and after the post-automation upsert, so an error in either returned early before the audit was written.
why2: The audit call was placed at the end of the happy path during the workspace-decomposition refactor, treating audit as a final step rather than a consequence of the durable write.
why3: Audit-ordering relative to the persisting write was never pinned by a test; existing audit tests only exercised the success path where ordering is invisible.
why4: The audit log's 'every successful write is audited' invariant was documented as a rule but had no failure-path regression guarding it.
why5: Write-path side effects (audit, events) that must be tied to durability have no structural pattern forcing them to be emitted at the persistence point rather than at function end.
prevention: Audit is now recorded immediately after the persisting store write succeeds, before any automation re-write or cascade. A regression test (failingUpdateStore) forces the post-automation re-write to fail and asserts the durable create is still audited; the test fails without the fix.
status: done
---

## Bug

Found in the 2026-06-09 backend review (write-path theme B1).

`Manager.CreateEntity` (`manager.go:306-322`) and `Manager.UpdateEntity`
(`:436-452`) recorded the entity audit row only *after* `Cascade.Process`. Two
failure windows left a durable, committed write unaudited:

- **Reachable:** on create, `createCore` persists the entity, then automation that sets a property triggers a second `upsertEntity`. If that second write hits a transient store error, the call returns an error while the entity is already on disk — and the audit row was never written.
- **Latent:** if the cascade ever grows a hard-error return (today `autocascade.Runner.Process` only hard-errors on a nil trigger, which the Manager never passes), the same gap appears for both create and update.

## Fix (PR pending)

Record `recordEntityAudit` immediately after the persisting write succeeds,
before the automation re-write / cascade. Audit rows carry no property values,
so recording before the post-automation rewrite loses nothing. The two later
audit calls are removed.

Regression test `TestCreate_AuditsDurableWriteWhenPostAutomationUpsertFails`
uses a `failingUpdateStore` to fail the second persist and asserts the create is
still audited (fails without the fix); `TestUpdate_AuditsBeforeCascade` pins the
update path on the cascade-enabled branch.
