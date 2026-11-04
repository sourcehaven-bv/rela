---
id: RR-KCUJVQW
type: review-response
title: Read gate on existing ends not rechecked inside the Tx
finding: An end that becomes hidden between the gate and the commit still gets the edge.
severity: minor
reason: Same window as single CreateRelation; the end was readable when checked, so the caller learns nothing new.
status: wont-fix
---
