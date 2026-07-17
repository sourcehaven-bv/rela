---
id: RR-YOFVE3
type: review-response
title: Delete phase remains unconditional — can still clobber a concurrent writer (same bug class, opposite op)
finding: 'The PR hardens the create path but the terminal delete loops (rename.go:108-125) are unchanged and unconditional. Same multi-writer race, delete side: outgoing/incoming relations are snapshotted at lines 76-77; if a concurrent writer modifies an old relation between the snapshot and the delete, rename deletes it anyway and the concurrent update is silently lost. DeleteRelation/DeleteEntity have no compare-and-swap, so it can''t be closed within this decomposition — which is exactly why the atomic store.RenameEntity (RR-PUI4JF) is the real fix. Impact of THIS PR is unchanged from before (delete side was already like this), but the new create-path comments (lines 92-94, 217-219) claiming ''we don''t clobber racing writers'' are now only half-true. Fix: either (preferred) route through st.RenameEntity per RR-PUI4JF, or scope the create-path comments and package doc to be honest that the delete phase stays best-effort-clobber.'
severity: minor
resolution: 'Resolved by the re-route (RR-PUI4JF): the non-atomic delete loops that could clobber a concurrent writer are deleted along with internal/rename. Entity + relation re-keying and the old-row removal now happen together inside the atomic store.RenameEntity (single pgstore transaction), so there is no snapshot-then-delete window on either the create or delete side.'
status: addressed
---
