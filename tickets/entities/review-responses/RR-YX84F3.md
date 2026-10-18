---
id: RR-YX84F3
type: review-response
title: Orphan restore onto a reused id leaves stale channels live and never purged
finding: retention.go:243 untombstones an orphan batch whenever the id exists, including when rela created a new entity with that id. The channels carry the old incarnation, so reads hide them, but they are live on an existing id and no later sweep erases them. Compare incarnation before restoring, or let EntityCreated mark the id's orphan batches non-restorable.
severity: significant
resolution: EntityCreated calls ConfirmTombstones for the new anchor, and the store refuses to restore a final batch (ErrConflict), which the sweep then confirms and purges.
status: addressed
---
