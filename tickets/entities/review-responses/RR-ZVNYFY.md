---
id: RR-ZVNYFY
type: review-response
title: 'Code: rename hook errors reported as CalDAV alias failures'
finding: Errors returned from EntityRenamed were logged by the alias fanout as alias-rewrite failures; every read error was swallowed.
severity: significant
resolution: EntityRenamed logs its own failures with an automation message and returns nil.
status: addressed
---
