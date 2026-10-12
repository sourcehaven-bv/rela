---
id: RR-3BNIXB
type: review-response
title: Replays lack a timeout, a pass budget and fairness
finding: 'Security design review: one expensive target could hold a replay slot for the whole watch, and one identity could starve the others.'
severity: minor
resolution: 5 s per replay, 10 s per pass with rotation, 4 watches per identity, and at most one replay slot per identity at a time. The plan notes the 8-replay ceiling together with slice 2's pool.
status: addressed
---
