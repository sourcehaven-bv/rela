---
id: RR-YAARP
type: review-response
title: Self-referential rename stages duplicate DeleteRelation calls
finding: 'For self-ref A--rel-->A, the rename closure stages the same DeleteRelation twice (once via the outgoing loop, once via the incoming loop). Today this works because repository.transaction.commit silently swallows the second-attempt remove error in phase-2 deletes. If that pre-existing repo bug is fixed, self-ref rename will start failing. PR fix: skip self-ref entries in the incoming delete loop, mirroring writeRenamedRelations.'
severity: significant
resolution: Fixed in this PR. The new Workspace.Rename closure's incoming-delete loop skips entries where rel.From == oldID (self-referential), exactly mirroring writeRenamedRelations. The duplicate DeleteRelation is no longer staged. The existing TestRename_SelfReferential test continues to pass.
status: addressed
---
