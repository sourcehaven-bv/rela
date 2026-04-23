---
id: RR-ZBRWR
type: review-response
title: MatchedRoute.Values exposed but unused
finding: internal/frontendroutes/routes.go:35-38 MatchedRoute.Values (Vue-name → value map extracted during matching) is never read by any caller. Drop it from MatchedRoute; a future consumer can re-add it when they need it.
severity: minor
resolution: Dropped MatchedRoute.Values. MatchedRoute now carries only Route. Doc comment notes 'Add a field (or a separate MatchParams call) when a caller actually needs extracted values.' Updated routes_test.go and parity_test.go accordingly.
status: addressed
---
