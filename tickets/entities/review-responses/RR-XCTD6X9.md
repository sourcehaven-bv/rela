---
id: RR-XCTD6X9
type: review-response
title: Refused channel uploads are not audited
finding: Entity uploads audit refused MIME/scan as denied-write (CONTROL-8-15); the plan was silent.
severity: significant
resolution: Refused channel files are written to the global audit log as a denied write naming channel key and property and the sanitized display name and reason; never to the per-entity channel trail.
status: addressed
---
