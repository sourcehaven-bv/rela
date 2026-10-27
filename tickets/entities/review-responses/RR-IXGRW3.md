---
id: RR-IXGRW3
type: review-response
title: Stored hash is not evidence of the frozen state
finding: Whoever can rewrite the bytes can rewrite the stored hash.
severity: significant
resolution: 'Plan: the audit record of the attachment write carries sha256 and the render source (document or list name and entry and format). Attachment security guide says the audit entry is the tamper-evident record.'
status: addressed
---
