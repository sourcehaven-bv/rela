---
id: RR-8BC6AU
type: review-response
title: Speculative exports and unexported "reusable" helpers
finding: '`Session.StatesWaitingFor(role)` has no consumer in this slice. `waitingView` and `waitingFacts` are unexported in `internal/realtime`, so "the inbox will reuse it" holds only if the inbox lives in that package. Change: drop `StatesWaitingFor` until the inbox ticket needs it, and say where the inbox evaluation will live.'
severity: nit
resolution: StatesWaitingFor dropped; the inbox will be a Service method in internal/realtime, so waitingView stays unexported (Approach 3).
status: addressed
---
