---
id: RR-GSHRX
type: review-response
title: UpdateRelation merges meta but the new wire format is replacement-only — semantic mismatch
finding: |-
    AC #5 says: 'PATCH with data: [{id: L1}] (no meta) clears those properties.' Workspace.UpdateRelation at workspace.go:1521-1543 MERGES opts.Properties into rel.Properties — it cannot clear; passing empty Properties leaves existing keys. So if the handler routes update-meta through UpdateRelation, AC #5's 'clears' path fails.

    Fix: handler does NOT call Workspace.UpdateRelation. It builds a fully-formed *model.Relation with the desired final properties map (which may be empty) and stages via tx.WriteRelation. The relation file on disk is rewritten to exactly the new content. Document in plan: 'Workspace.UpdateRelation is intentionally left alone — automation code relies on merge semantics. Unifying these paths is a follow-up if needed.'
severity: critical
resolution: Handler refactor (Approach step 7) builds a fully-formed *model.Relation with desired final state (after applying meta merge + meta_unset + content upsert) and stages via tx.WriteRelation directly. Workspace.UpdateRelation is intentionally bypassed for this code path — its merge semantics are kept for automation code. Documented in research as 'unifying is a follow-up'.
status: addressed
---
