---
id: RR-9O9RFZ
type: review-response
title: Delete-then-recreate with identical content left timeline stuck at 'delete' for a live entity
finding: 'cranky-code-reviewer: the sweep dedup was content-only. Sequence: create X (hash H) → delete X (delete row captured with hash H) → re-create X with identical content H. The sweep''s latest-version probe found the delete row (hash H), so captureOne dedup''d and wrote NO create. Result: the entity is live but its timeline ends in op=delete — the history lies about liveness.'
severity: critical
resolution: 'The sweep now uses two LATERALs: `lv` (latest of any op) to detect that the latest is a delete (=> live-lineage=false => next capture is op=create), and `lvc` (latest version SINCE the last delete) whose content_hash is the dedup key. Deduping only within the current lifecycle means a re-creation with identical bytes still records a create. Regression test TestDeleteThenRecreateIdenticalContent (DB-verified) asserts the post-recreate timeline ends in create, not delete.'
status: addressed
---
