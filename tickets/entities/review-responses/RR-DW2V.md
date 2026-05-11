---
id: RR-DW2V
type: review-response
title: primaryInaccessibleReason returns first reason regardless of which field is locked
finding: 'internal/dataentry/mentions.go lines 67-74: iterates Inaccessible and returns the first non-empty Reason. With a single InaccessibleField (content, today), that''s correct. But the comment ''Today only git-crypt is produced; the indirection keeps the call site stable when other reasons are added'' anticipates richer cases. When that happens, an entity with multiple inaccessible fields could have different reasons per field (e.g. title=sops, content=git-crypt). Returning the FIRST non-empty reason ties UI behavior to map-iteration order. Slice iteration is deterministic in Go, but the implicit assumption is ''all reasons are equal'' — not documented. If you keep this shape, document the precedence; if you want to be honest, return the reason matched to the inaccessible field most relevant to the rewrite intent (content or title). Otherwise leave a TODO so the next dev hits the question deliberately.'
severity: nit
resolution: 'Doc-comment on lockedReasonFor explains precedence: a field matching either the display property or InaccessibleFieldContent sentinel triggers the lock; the first match wins. With one inaccessible field per entity today (git-crypt locks the whole file) precedence is deterministic; comment captures the rule for future loaders.'
status: addressed
---
