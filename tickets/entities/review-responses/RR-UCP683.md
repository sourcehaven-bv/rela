---
id: RR-UCP683
type: review-response
title: State captured more than once per request
finding: The handler reads a.State() and then the ACL, scope resolver, view conditions and listPage separately.
severity: minor
reason: Same shape as navItemsFor and the nav status endpoint, which this endpoint reuses. listPage resolves its own state by design; threading s through it is a wider refactor than this ticket.
status: wont-fix
---
