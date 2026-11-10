---
id: RR-QI6OYY9
type: review-response
title: complete affordance rebuilds the write check
finding: mayUpdate builds its own acl.WriteRequest while Complete uses AuthorizeUpdate, so the button and the POST could drift.
severity: minor
reason: Needs a dry-run decision on entitymanager. The POST stays authoritative; recorded on TKT-FWQZGAM.
status: deferred
---
