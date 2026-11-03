---
id: RR-F45VDR
type: review-response
title: B4 names too_many_requests but the existing type is rate_limited
finding: internal/realtime/errors.go defines TypeRateLimited = "rate_limited". Fix the B4 text so the HTTP tests do not assert a type that does not exist.
severity: nit
resolution: The plan uses rate_limited.
status: addressed
---
