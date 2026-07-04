---
id: RR-7AQX4E
type: review-response
title: feedBaseURL trusts X-Forwarded-Proto for scheme
finding: feed_handler.go:92 derives http/https from the spoofable X-Forwarded-Proto header. Only flips scheme in the emitted deep link; on bare loopback there's no proxy attacker, and behind a real proxy it's the intended signal. Host is properly validated (hostUnsafeForCSP) with a relative-URL fallback. Low impact; noted for completeness.
severity: minor
reason: Acknowledged as acceptable (reviewer concurred). X-Forwarded-Proto only flips http/https in the emitted deep link; on bare loopback there is no proxy attacker, and behind a real proxy it is the intended signal (same trust model as the rest of the server). Host is validated (hostUnsafeForCSP) with a relative-URL fallback. No behavior change warranted for Phase 1's loopback-trust model; revisit if a stricter proxy-aware base-URL policy is introduced.
status: wont-fix
---
