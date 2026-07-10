---
id: RR-EZ4I5Q
type: review-response
title: rel_record_id must live on the relations row, not be reconstructed per sweep tick
finding: 'The design reconstructs the surrogate lineage id from the composite key at each sweep tick (bare sequence + composite lookup). This races the SYNCHRONOUS delete/rename capture, which runs on a different connection and does NOT hold the sweep advisory lock: two allocations fork history, or the sweep attaches to a stale record_id the sync path just terminated (merge across a delete boundary). Fix (both reviewers): put rel_record_id ON the relations row as a column with DEFAULT nextval(<dedicated seq>), assigned at CreateRelation, carried verbatim through the cascade UPDATEs, gone on delete. Lineage then reads straight off the row; delete-recreate naturally gets a fresh id; rename-capture is a plain read. This dissolves the reused-id-merge / delete-recreate-dedup race class rather than probabilistically rebuilding identity every tick. The sweep must resolve-or-attach under its advisory lock; first-ever allocation authority must be stated explicitly (architect C1, cranky S2).'
severity: critical
resolution: 'Design revised: rel_record_id is now a column ON the relations row (DEFAULT nextval), assigned at CreateRelation, carried through cascade UPDATEs. Sweep resolves off the row under its advisory lock; never reconstructs/allocates. See revised ''Identity'' section.'
status: addressed
---
