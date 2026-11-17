---
id: RR-HHJ15NW
type: review-response
title: RequestEditor returns an interface
finding: RequestEditor returns TriggerEditor instead of a concrete type.
severity: nit
resolution: The concrete type is unexported on purpose so callers cannot reach the request through it; CheckTriggerACL takes the interface.
reason: The concrete type is unexported on purpose so callers cannot reach the request through it; CheckTriggerACL takes the interface.
status: wont-fix
---
