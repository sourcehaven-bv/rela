---
id: RR-K3ZDZ5B
type: review-response
title: Cancel can still be logged outside writeListPipelineError
finding: attachACLRequest answers 500 acl_identity_unresolved on a canceled membership walk (router.go:483-488); acl.PrimeLocalRoles logs WARN on a canceled ctx (localroleprime.go:84-85), reached from handleV1Search.
severity: significant
resolution: 'Plan changed: both paths treat a canceled request as silent; AC1 lists the stages covered and the tests cover identity resolution and verdict priming.'
status: addressed
---
