---
id: RR-MNBL47
type: review-response
title: Field denial audit lacks op=attachment-write
finding: denyAffordance writes a generic affordance denial, so an operator searching attachment denials misses it.
severity: minor
resolution: The preflight records audit.AttachmentWriteDenied with rule_kind affordance, so the row carries op=attachment-write. The test asserts two such rows.
status: addressed
---
