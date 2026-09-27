---
id: RR-IKY002
type: review-response
title: 'Test does not pin that the proxied Host is only accepted behind the JWT gate'
finding: 'http_test.go pins the SDK behaviour but not the invariant the change relies on, that the endpoint is only reachable after JWT verification.'
severity: significant
resolution: 'Already pinned in internal/dataentry by TestSetRemoteMCP_RefusesWithoutJWTGate and TestRemoteMCP_UnauthenticatedIsRefused. A router-level test with the real SDK handler is not possible there: arch-lint bars internal/dataentry from importing the go-sdk.'
reason: 'The invariant is already pinned where the mount lives: TestSetRemoteMCP_RefusesWithoutJWTGate and TestRemoteMCP_UnauthenticatedIsRefused in internal/dataentry. A router-level test with the real SDK handler cannot live there, because arch-lint bars internal/dataentry (tests included) from importing the go-sdk.'
status: wont-fix
---
