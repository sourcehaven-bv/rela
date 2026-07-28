---
id: RR-QSO6HF
type: review-response
title: dirtyFormRegistry should be DELETED, not wired — its union semantics are wrong for merge arbitration
finding: 'The plan recommends wiring dirtyFormRegistry into the merge flow. Deciding against: anyFormDirty(entityId, property) answers ''is ANY registered form dirty for this property'', unioning across every form registered for that entity (dirtyFormRegistry.ts:5-7,30-37). The merge does not need a cross-form union — it needs THIS instance''s dirty state, which useAutoSave already has locally and precisely (isDirty at :225-231 checks pending, timers, and the dirtyWindowMs recency window; mergeServerResponse already consults exactly that via ''k in pending''/''k in timers'' at :514-515). Failure if wired: a side panel and the main form are both open on TKT-1; the user edits status in the side panel only; the main form 412s on title; the merge consults anyFormDirty(''TKT-1'',''status''), sees dirty=true, and preserves the main form''s STALE status — clobbering the server value with data from a form the user is not editing, because a DIFFERENT form is dirty. The union semantics are right for the original purpose (suppressing an SSE refetch) and wrong for merge arbitration. RECOMMENDATION: delete it — zero production consumers, the SSE-refetch path it was built for does not exist, and the composable''s local isDirty is strictly better for the merge.'
severity: significant
status: open
---
