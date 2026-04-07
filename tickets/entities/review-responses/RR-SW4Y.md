---
id: RR-SW4Y
type: review-response
title: 'S2: sensitive-path matcher is opt-in not opt-out — future endpoints might silently leak'
finding: isSensitivePath returns true only for paths under /api/. Any new HTTP route outside /api/ silently bypasses the Origin allowlist. A future /metrics or /export/csv endpoint would be cross-origin readable until somebody noticed.
severity: minor
reason: Inverting to default-sensitive is the more robust design but requires every existing static-asset path to be enumerated and tested. The current /api/ prefix is internally consistent (the SPA only ever calls /api/v1 plus /api/events) and a code review is required for every new top-level route anyway. Documented the footgun in the sensitivePathPrefixes comment block. Tracked as future hardening rather than blocking this PR.
status: deferred
---
