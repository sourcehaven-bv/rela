---
id: RR-ZWB950
type: review-response
title: 'The plan still describes the session-level final: list'
finding: 'The decision for per-state final: true changes several parts of the plan, and none of them is updated yet. (1) rawSession loses Final []string, and rawSessionState gains Final bool. (2) The AC 2 rows "final naming an undeclared state" and "listed twice" go away, and so do their messages. (3) Add a rule that refuses an initial state marked final: an unset session reads as initial, so every channel would be final from the start and nothing could ever move. (4) A session-level `final:` key is now an unknown key; add it to refusedKeys with "mark the state with final: true" if you want an actionable message. (5) Remove the "Per-state final: true" entry from Alternatives rejected. (6) Update the example, AC 1 ("the final set") and the wording of the "final state with on:" rule ("remove final: true").'
severity: significant
resolution: 'Plan updated: final states are per state final: true throughout; a session-level final: is refused with a hint and an initial state marked final is refused.'
status: addressed
---
