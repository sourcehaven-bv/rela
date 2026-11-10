---
id: RR-FU825I9
type: review-response
title: Read-only check could live in authorizeAndAudit bypass branch
finding: 'M1: move the read-only refusal into authorizeAndAudit''s bypassACL branch.'
severity: nit
reason: That branch serves every elevated write including automation bypass_acl cascades so the move changes behaviour outside this PR. The scoped check is tested here; the general move belongs in its own ticket.
status: deferred
---
