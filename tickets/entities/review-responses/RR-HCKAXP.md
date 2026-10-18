---
id: RR-HCKAXP
type: review-response
title: AC4 redaction is only simulated, and degraded mode is never driven through real wiring
finding: lifecycle_test.go:178 "redaction" deletes the key from the fake. realtime_service_test.go:145 runs under NopACL with Degraded never set through an acl.yaml reload. Plan section 13 restated AC4 as the degraded case, so add an appbuild test that reloads an acl.yaml restricting the condition's field and asserts the op is neither offered nor allowed.
severity: minor
resolution: Added TestChannelService_DegradedRefusesConditions in appbuild. It reloads acl.yaml with a `visible:` grant that hides the doc title the condition reads, builds the successor with WithRealtimeFallback, and asserts the snapshot is Degraded with the visible-grant problem. Close is then not in _actions.lifecycle, and the POST is 403 lifecycle_denied with the degraded title.
status: addressed
---
