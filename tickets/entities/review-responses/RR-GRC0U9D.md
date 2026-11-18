---
id: RR-GRC0U9D
type: review-response
title: Rename with no renamed rows writes no record
finding: familyRename.audit returned nil when nothing was renamed on a TxAudit sink.
severity: minor
resolution: Returns errRenameUnaudited, failing the write.
status: addressed
---

familyRename.audit returned nil when nothing was renamed on a TxAudit sink.
