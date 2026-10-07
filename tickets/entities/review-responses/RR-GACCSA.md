---
id: RR-GACCSA
type: review-response
title: Refused replace wrote delete versions before the transaction
finding: A refused replace recorded delete versions in relation history before the transaction ran. History then showed deletes that never happened.
severity: minor
resolution: Fixed in 21e383051. Versions are recorded only after the transaction succeeds. Covered by TestReplaceRelations_RecordsVersionAndAudit.
status: addressed
---

Review finding R1-7.
