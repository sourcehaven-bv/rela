---
id: RR-AB6R1J
type: review-response
title: Separate tag does not separate the journald rate limit
finding: journald rate-limits per unit, not per tag; one access line per request consumes the same budget as the application log, so a flood can drop warnings and errors.
severity: significant
resolution: Documented in GUIDE-server-security (Operational limits) with LogRateLimitBurst= and an authenticating proxy as mitigations. On atlas pratique authenticates every request before rela, so unauthenticated floods do not reach it.
status: addressed
---
