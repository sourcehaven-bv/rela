---
id: RR-USAWK8
type: review-response
title: Final-state confirm is a proxy for reversibility of the session, not of the effect
finding: 'For the PoC the rule gives: accept to done (confirm, default tone), abandon to abandoned (confirm, danger). Both are reasonable. But the clicks with the largest external effect get no confirm: allow (the agent runs the requested tool), approve (the agent starts writing code), rework and retry (they relaunch a run). The form path (a required-prop type into a final state, or the composer) never confirms, so the rule only covers the button. Accept the heuristic for this ticket, but write in docs/realtime.md exactly what it covers. Because S5OJ7S does not own an authored override (see above), file a follow-up for a per-transition `confirm:`.'
severity: minor
resolution: Kept the derived final-state rule for this ticket, documented exactly what it covers and does not, and proposed the follow-up for an authored per-transition confirm:.
status: addressed
---
