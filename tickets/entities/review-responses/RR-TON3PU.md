---
id: RR-TON3PU
type: review-response
title: Stop returning on drain timeout abandons a goroutine holding an SMTP connection
finding: With a transport that does not honour ctx (a plausible library, or a blocked syscall), Stop returns after DrainTimeout while the worker goroutine is still running. The doc said it abandons the messages; it also abandons a goroutine holding an SMTP connection. In multi-tenant eviction, where Services.Close runs repeatedly against a shared provider, that accumulates connections against the mail server across evictions — a leak with external blast radius, not just a stray goroutine. goleak.VerifyTestMain does not catch it because every test transport honours cancellation.
severity: significant
resolution: The behaviour is inherent to a bounded drain — the alternative is hanging shutdown, which is worse — so the fix is to stop understating it rather than to change the semantics. Stop's doc now states plainly that a timed-out drain leaves the in-flight send running until it returns on its own, holding its connection, and that a transport ignoring ctx makes this the norm rather than the exception; the operator guide notes the multi-tenant accumulation. The Sender contract already requires honouring ctx, which is what keeps this to a documented edge rather than the default path.
status: addressed
---
