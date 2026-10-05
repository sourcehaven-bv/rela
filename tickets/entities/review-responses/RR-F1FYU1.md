---
id: RR-F1FYU1
type: review-response
title: Duplicated collect loop in ScriptReader
finding: ListRelations and ListRelationsStrict repeat the same collect loop.
severity: nit
reason: Two short loops with different error handling; a helper adds indirection for little gain.
status: wont-fix
---
