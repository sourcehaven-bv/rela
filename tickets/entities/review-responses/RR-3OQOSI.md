---
id: RR-3OQOSI
type: review-response
title: '`today()` timezone and the evaluator clock are unspecified'
finding: '"Capture once per round" reads `now` once, but `NewEvaluatorWithClock` takes a func, so it must return the captured value or rules can see two different times in one round. The timezone that `today()` uses for midnight crossings (picked up by the 15m reconcile) is also not stated.'
severity: minor
resolution: The evaluator clock returns the captured round time. today() uses the server time zone as elsewhere in predicate; documented.
status: addressed
---
