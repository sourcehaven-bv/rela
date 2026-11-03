---
id: RR-2WP0RB
type: review-response
title: Entity-local face grants are not reported
finding: AccessRoutes is type-granular, so a role held on one entity that grants type@face produces no exception.
severity: significant
reason: Pre-existing in acl map exceptions and unrelated to webhooks, whose roles are global assignments. The gap is now stated in docs/webhooks.md and the expectation godoc, so --expect no longer claims to cover it.
status: deferred
---
