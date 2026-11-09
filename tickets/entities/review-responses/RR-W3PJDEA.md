---
id: RR-W3PJDEA
type: review-response
title: Fingerprint ranges load rows outside the stated row budget
finding: The 2000-row budget counts list limits only; a fingerprint range on a type with a hidden field may load and redact up to 2000 rows, so a request can load 32 x 2000 and the docs said otherwise.
severity: significant
resolution: 'The real bound is stated in the handler threat model, api-reference.md and acl-security.md: up to 2000 rows per range, 32 ranges per request, with the slots bounding concurrency and the deadline checked between ranges.'
status: addressed
---
