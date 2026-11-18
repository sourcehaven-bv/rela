---
id: RR-UNVHA37
type: review-response
title: joinAuditFor returns nil in a mixed-tag build
finding: A build with both tags falls back silently.
severity: nit
reason: Tags are exclusive per binary; no build sets both.
status: wont-fix
---

A build with both tags falls back silently.
