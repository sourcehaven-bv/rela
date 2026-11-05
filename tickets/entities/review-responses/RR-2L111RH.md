---
id: RR-2L111RH
type: review-response
title: Conflict resolve uses only the handler field check
finding: A PATCH runs the handler field check and then the manager FieldWriteGate. Conflict resolve writes the file directly so it gets only the handler check and could drift if the manager gate gains a rule.
severity: minor
reason: Both checks call affordances.CheckFieldWrite so today they agree. Routing conflict writes through entitymanager is a larger change to the conflict write path.
status: deferred
---
