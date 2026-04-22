---
id: RR-K6DJL
type: review-response
title: api fixture always sets Origin, masking CSRF regressions
finding: 'Every api call sets Origin: serverUrl, so the Origin allowlist middleware is never exercised. A regression that disables the allowlist would pass tests silently. Backend security canary is missing.'
severity: critical
resolution: Initially added origin-security.spec.ts as a GET-only e2e canary, then expanded to POST/PATCH/DELETE on review. On follow-up we realised internal/dataentry/middleware_security_test.go + router_security_test.go already cover the same invariants at the unit-test layer (rejection of cross-origin writes, missing Origin/Referer, allowlist extra origin). Deleted the e2e spec — the middleware canary is load-bearing in Go, not in Playwright.
status: addressed
---
