---
id: RR-A3RNT0
type: review-response
title: Live read-gate on history leaks pre-access & later-redacted content (no point-in-time ACL)
finding: '''Same read gate as live entity'' evaluates only against the CURRENT graph (PermitsRead→computeForEntity, resolver.go:118; no asOf — IDEA-CQMKMD out of scope). So a principal who gains read access today can read the entity''s ENTIRE history from before they had any relationship to it (common for local-role-conferred ACL, resolver.go:145-162), and can read old-version content that was later redacted/edited out for compliance. Not a bug per se, but NOT what ''same read gate'' implies. Fix (minimum): document loudly that history read is all-or-nothing from creation — reading the live entity exposes its full history incl. pre-access and later-removed content; compliance redaction is NOT history-aware. Stronger follow-up: a ''purge version'' operator primitive so a compliance redaction actually deletes the snapshot row (capture as a follow-up ticket).'
severity: significant
resolution: Addressed by documentation + a scoped follow-up. Plan documents (docs/acl-security.md) that history read is all-or-nothing from creation — reading a live entity exposes its full history incl. pre-access and later-removed content; history is NOT point-in-time-ACL-aware (that is the IDEA-CQMKMD epic, out of scope) and NOT redaction-aware. The compliance-redaction gap (an old version resurrecting content deliberately edited out) is handled by a new follow-up TKT-BW6UUL (operator 'purge version' primitive to hard-delete a snapshot row). Acceptable for v1 given the documented semantics.
status: addressed
---
