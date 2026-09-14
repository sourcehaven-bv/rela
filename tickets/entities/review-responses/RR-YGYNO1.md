---
id: RR-YGYNO1
type: review-response
title: dropEntitiesStep.Run drops the partial cascade result on its error path
finding: 'IB-review on rela#1488 (CISO, blocking): the ticket fixes partial-cascade audit capture on Manager.DeleteEntity and cascadeHost.DeleteEntity, but misses the third DeleteEntity call site. internal/datamigration/steps.go dropEntitiesStep.Run calls x.Store.DeleteEntity directly, bypassing entitymanager, and returns immediately on error without reading del.DeletedRelations. Confirmed: the store returns a partial DeleteResult ALONGSIDE the error (fsstore entity.go builds `removed` as it goes, because its Tx is a write mutex with no rollback), so relations already off disk on that path left no audit trail at all. This is the exact defect the ticket exists to fix, on the one path the first fix did not reach.'
severity: significant
resolution: |-
    ACCEPTED. The error path now captures del.DeletedRelations before returning, matching the success path directly below it and the two entitymanager paths this ticket already fixed. The step still fails — the cascade genuinely did — but the relations that came off disk reach the audit trail.

    Capture failures there append to res.Notes rather than replacing the original error: the cascade failure is the more important one to surface, and swapping it for a capture error would hide why the migration stopped.

    Nil-guarded: del is nil when the store fails before removing anything, and on a backend whose Tx rolls the whole cascade back. The range is a no-op in both cases, so the guard is about not dereferencing nil rather than about correctness of the audit.

    Both call sites now share a captureCascaded helper. Inlining the loop on the error path pushed dropEntitiesStep.Run to cognitive complexity 32 against a limit of 30, and the two copies were identical anyway -- one place for the nil guard and the best-effort rule, rather than two that can drift.

    The neighbouring ErrNotFound branch was checked and deliberately left alone: fsstore returns that error before any relation file is removed, so it cannot carry a partial result.

    Pinned by TestDropEntities_PartialCascadeIsCaptured, which uses a store wrapper that fails DeleteEntity while reporting one removed relation — the real backend contract. Mutation-verified: with the fix reverted the test fails with "partial cascade left no audit trail; captured relations = []", and passes with it restored.
status: addressed
---
