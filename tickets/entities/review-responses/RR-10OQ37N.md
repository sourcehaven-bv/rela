---
id: RR-10OQ37N
type: review-response
title: A pile whose key is no longer declared stays half-special
finding: When a declaration is removed the stored pile keeps its key so it skips the name rule and the pile limit and add_to_pile by name cannot reach it.
severity: minor
reason: Removing a declaration is rare and needs a cleanup decision (clear the key or drop the pile). Follow-up rather than part of this change.
status: deferred
---
