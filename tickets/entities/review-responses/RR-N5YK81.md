---
id: RR-N5YK81
type: review-response
title: 'Rename cascade: updated_at not bumped (sweep-blind) + no ON CONFLICT (PK collision merges/aborts)'
finding: 'Two problems in the RenameEntity relation cascade (pgstore/entity.go ~L420/426, `UPDATE relations SET from_id=$2 ...` / `SET to_id=$2 ...`). (1) It does NOT set updated_at, so the debounced sweep is structurally blind to endpoint renames — synchronous rename capture is load-bearing, not optional. (2) It has no ON CONFLICT against the composite PK (from_id,rel_type,to_id): renaming A→B when a (B,t,X) already exists collides the rewritten (A,t,X)→(B,t,X) and today throws a unique-violation aborting the whole rename. The design''s per-relation `rename` version with prev_from/prev_to implies lineage continuity, which is a LIE for a collision (A,t,X was absorbed into a pre-existing edge, not continued). Fix: capture rename versions post-commit in the entitymanager hook by extending RenameResult (keeps attribution at the boundary per CLAUDE.md); keep rename ABORTING on relation-PK collision and document that a `rename` version is only written for a 1:1 re-key. If merge is ever wanted it needs its own `op` and a terminal `delete` for the absorbed lineage — never a silent PK overwrite (architect C2, cranky C2).'
severity: critical
resolution: 'Design revised: rename versions captured post-commit via extended RenameResult (attribution at boundary); rename stays ABORTING on relation-PK collision; rename version only for 1:1 re-key; merge deferred as explicit follow-up op. See revised ''Synchronous rename'' section.'
status: addressed
---
