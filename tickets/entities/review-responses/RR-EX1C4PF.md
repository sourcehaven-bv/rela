---
id: RR-EX1C4PF
type: review-response
title: A create cut off by the round deadline is skipped for good
finding: writeOutput kept the claim when its context ended during a create. The worker's 1m round deadline, every shutdown and every tenant eviction cut creates routinely, and a cut create usually never landed, so the output was lost with one Warn line. Found by both cranky-code-reviewer (significant) and rela-security-reviewer (minor, with a nit on the worker comment).
severity: significant
resolution: 'The create now runs on a context detached from the caller (context.WithoutCancel) and bounded by outputTimeout (30s). A round deadline or shutdown no longer cuts it. Only a create that runs out of its own time keeps the claim, and that skip logs at Error. The give-back and LinkProduced use the same context. Tests: TestDue_CreateOutlivesItsCaller and TestDue_TimedOutCreateIsNotWrittenAgain. The worker and DueRound comments now say this.'
status: addressed
---
