---
id: RR-1SUTXH
type: review-response
title: Queue was wired but never started; Enqueue before Start gave a confusing upstream error
finding: 'appbuild constructed the queue and stored it but never called Start, and the memory backend only creates its queue channel in Start. So the first real producer calling svc.Jobs().Enqueue(...) would have received ''no processor configured for queue: rela'' — an error describing neoq''s internals rather than the caller''s situation. The Queue interface documented ordering for Register but said nothing about Enqueue.'
severity: significant
resolution: Two changes. (1) buildJobQueue now starts the queue, so Services.Jobs() hands out a running queue; handlers may still register afterwards since the dispatcher resolves per job. (2) Added the ErrNotStarted sentinel and documented the ordering on the interface. The deferral path deliberately does NOT require a started queue — a transaction may collect before start, since the enqueue happens at commit. TestServices_JobsIsWired now asserts a second Start is refused.
status: addressed
---
