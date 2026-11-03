---
id: RR-8DFPWD
type: review-response
title: NoPromoter detection is a value type assertion
finding: 'internal/realtime/authz.go:76 and service.go:84 s.promoter.(NoPromoter) does not match &NoPromoter{}. Fix: compare once in New and store a bool on Service, or add an optional CanPromote() bool method.'
severity: minor
resolution: isNoPromoter recognises NoPromoter and *NoPromoter and is used only for the startup warning; behaviour no longer depends on the type since NoPromoter answers action_not_offered from its own methods (453779953)
status: addressed
---
