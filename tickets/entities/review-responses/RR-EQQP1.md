---
id: RR-EQQP1
type: review-response
title: Purging a rename row orphans/forks lineage — 'no lineage fencing on delete' is false
finding: 'cranky C3. The ticket says ''No lineage fencing needed on delete — a purge is an explicit row removal.'' FALSE for rename rows. Entities: lineageCTE (version.go) reconstructs ''history of B including its life as A'' by walking op=rename rows via prev_id. Purge the rename row (entity_id=B, prev_id=A, vseq=V) and A''s entire pre-rename history becomes unreachable through B (and if A''s id was reused, un-addressable entirely) — AND the fence math (max(vseq) WHERE prev_id=$1 AND op=rename) shifts, potentially MERGING two unrelated entities'' histories (the reclaimed-id leak the CTE exists to prevent). Relations: relationLineageIDs walks rename rows via prev_from/prev_to; purge the rename row and the predecessor rel_record_id is undiscoverable, orphaning the pre-rename relation history. Worst outcome for a compliance tool: the data you were told is gone is still on disk (just invisible/un-restorable) and the data you wanted to keep is severed. Fix: purge must NOT treat rename rows as ordinary. v1 (effort:m) honest scope: purge NON-RENAME snapshot rows only (by content-hash/vseq), document rename-row purge as unsupported; OR refuse to purge a rename row unless the operator confirms the lineage severance a dry-run shows. Strike the ''no lineage fencing needed'' sentence.'
severity: critical
resolution: 'Design revised: v1 purges NON-RENAME rows only; a rename-row target is refused; dry-run flags rename rows. Rename-severance handling is documented v2. ''No lineage fencing'' sentence struck. See revised design #2.'
status: addressed
---
