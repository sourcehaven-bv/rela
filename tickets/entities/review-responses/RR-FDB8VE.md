---
id: RR-FDB8VE
type: review-response
title: A refused write leaves its type record behind
finding: RecordTypes commits before the config_changed check and the write, so a refused write can leave a permanent record of the old shape.
severity: nit
reason: The window is narrow (a reload between snapshot and record) and the record still describes channels written under that shape; clearing records is an operator tool deferred to the T4 retention work.
status: deferred
---
