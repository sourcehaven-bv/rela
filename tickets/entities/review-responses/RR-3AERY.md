---
id: RR-3AERY
type: review-response
title: origin-security canary only covers GET requests
finding: The whole point of an Origin allowlist is to block cross-site writes. A GET-only canary misses the primary attack surface — a regression exempting POST/PATCH/DELETE would pass.
severity: critical
resolution: Covered by Go tests in internal/dataentry/middleware_security_test.go (TestRequireSameOrigin_RejectsCrossOriginOnSensitivePath tests POST/GET/DELETE explicitly) and router_security_test.go. The e2e canary was deleted on follow-up as redundant.
status: addressed
---
