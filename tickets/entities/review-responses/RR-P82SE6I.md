---
id: RR-P82SE6I
type: review-response
title: list_id naming an unknown list or uncompiled condition widens silently
finding: scopeListCondition returns rows unchanged when the list has no compiled condition.
severity: minor
reason: The position mirrors the list endpoint, which reads list_id the same way. Refusing here would make N of M disagree with the list on screen. Rows stay ACL-gated, and a condition that does not compile fails validation.
status: wont-fix
---
