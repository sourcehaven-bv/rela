---
id: RR-U74D0D
type: review-response
title: Unauthenticated callers share one identity cap
finding: 'Code review: with no authentication every caller is principal.Unknown. The 64-subscription identity cap was shared by all users.'
severity: minor
resolution: The identity cap does not apply to the unknown user; the per-stream cap still does. Documented in api-reference.md and pinned in TestLive_PutIdentityCap.
status: addressed
---
