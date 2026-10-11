---
id: RR-XL5A43
type: review-response
title: tokenHops and overwrite cases untested; test nits
finding: No tests for legacy or malformed tokens or for the overwrite; the jobHopsKey ctx value looked ambiguous; the fixed 50 ms wait would break if followUpDelay grows.
severity: nit
resolution: Added TestTokenHops and the overwrite test, commented the ctx value and the legacy-token contract, and derived the wait from the follow-up schedule.
status: addressed
---
