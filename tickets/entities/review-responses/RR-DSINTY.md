---
id: RR-DSINTY
type: review-response
title: Plan does not place redaction relative to the cap
finding: PLAN-VZXHRJ says the cap is applied 'post-gate' but never states where redaction sits. Redacting before capping pays a per-row copy for up to (matched - cap) rows that are then discarded — on a large type that is the dominant cost of the whole operation. The order must be gate -> cap -> redact. Cheap to specify now; annoying to catch in code review, and invisible in tests since both orders produce identical output.
severity: minor
resolution: Plan Approach now specifies the order explicitly as gate -> cap -> redact, with the note that both orders produce identical OUTPUT so no test can catch it — which is precisely why it had to be written into the plan rather than left to code review.
status: addressed
---

Raised by `/design-review` against PLAN-VZXHRJ, before implementation.

Note this is only observable as cost, never as output — which is exactly why it
needs to be written down rather than left to be noticed. A benchmark on a type
larger than the cap would show it; nothing else will.
