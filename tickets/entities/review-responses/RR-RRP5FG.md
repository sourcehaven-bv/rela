---
id: RR-RRP5FG
type: review-response
title: Plural lookup duplicated
finding: entityTypeForPlural already loops plurals.
severity: nit
reason: The word set is built once per snapshot and serves a different purpose; sharing would couple the log to API dispatch.
status: wont-fix
---
