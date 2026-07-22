---
id: RR-W5TQNB
type: review-response
title: 401 response timing distinguishes denial classes
finding: 'The three denial paths have observably different latency: no-header returns without touching the verifier; invalid-token performs a full parse and signature check; keys-unavailable can stall up to RateLimitWaitMax (5s). An attacker can therefore distinguish "I sent no token" from "I sent a token that was evaluated" from "the IdP is down" by timing alone.'
severity: nit
reason: Not exploitable in any constructible way, and the reviewer explicitly raised it for completeness rather than as a fix request. The channel reveals server state, not secret material — and the keys-unavailable case is externally observable anyway (the IdP being down is not a secret). Constant-time denial would mean padding every 401 to the 5s worst case, which converts a non-issue into a real availability and DoS-amplification problem. The oracle that WOULD matter — the response body explaining why verification failed — is correctly closed and pinned by TestRequireVerifiedJWT_DenialLeaksNoReason.
status: wont-fix
---

Reported by cranky-code-reviewer against `internal/dataentry/jwtgate.go:88,97`.

Recorded rather than silently dropped so the reasoning is on file if someone
raises it again in a future audit.
