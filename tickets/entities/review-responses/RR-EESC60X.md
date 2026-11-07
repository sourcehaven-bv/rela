---
id: RR-EESC60X
type: review-response
title: 'Security: pending reveals whether a hidden property is set'
finding: Pending is set only when a hidden key is present; on the wire or through timing it reveals presence.
severity: minor
resolution: Godoc names the bit and says it never reaches the wire; the timing residual is noted for step 5's acl-security docs.
status: addressed
---
