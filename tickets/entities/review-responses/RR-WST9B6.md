---
id: RR-WST9B6
type: review-response
title: AC1.7 asserted rule_kind non-empty rather than by value
finding: A mutation emitting a constant like unknown would satisfy a non-empty check while telling the operator nothing
severity: nit
resolution: 'rule_kind is now asserted by value (role-grant). rule_id and reason stay non-empty assertions deliberately: acl.Decision.Reason is documented as never carrying raw policy data; pinning its wording would invite widening it later to make a test pass.'
status: addressed
---

The point of AC1.7 is that the operator learns *which* gate fired, so
`rule_kind` is now asserted by value (`role-grant`, the documented kind for "a
role's write list either matched or didn't").

`rule_id` and `reason` stay non-empty assertions deliberately:
`acl.Decision.Reason` is documented as never carrying raw policy data, and
pinning its exact wording would invite widening it later to make a test pass —
the opposite of what that doc promises.
