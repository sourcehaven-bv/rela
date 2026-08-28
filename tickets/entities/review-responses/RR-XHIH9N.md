---
id: RR-XHIH9N
type: review-response
title: mdEntityRefs ctx closure comment gave wrong rationale (timeout does not flow through parentCtx)
finding: 'The ctx field comment claimed the closure exists ''so a timeout applied after registration still propagates''. Wrong: applyTimeout hands the timeout context to L.SetContext and never writes parentCtx, so callerCtx() returns the timeout-free parent. The closure is still correct — it makes registration-vs-option ordering robust — but the stated rationale was false, in prose adjacent to the load-bearing RR-ZA452J ACL note.'
severity: minor
resolution: 'Rewrote the comment: closure reads the WithContext parent at call time, keeping correctness if registration/option ordering ever changes; noted explicitly that the execution timeout reaches Lua via L.SetContext, not this path.'
status: addressed
---

Review finding from the TKT-4WBLG6 code review (cranky-code-reviewer).
Stale/wrong rationale in a security-adjacent comment; no behavior impact. The
reviewer also noted a leverage opportunity — mdASTConverter could drop its ls
field entirely (NewTable is receiver-independent) for full urlHelpers parity —
deliberately deferred to keep this PR purely structural; candidate for a later
TKT-N0IKN9 ratchet step.
