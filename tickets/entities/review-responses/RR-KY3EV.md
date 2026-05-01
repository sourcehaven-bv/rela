---
id: RR-KY3EV
type: review-response
title: Reword 'if reverse-direction save works' comment
finding: Comment at e2e/tests/reverse-relations.spec.ts:107-109 states the conditional ('if reverse-direction save works'). Reword to the contrapositive that the test actually enforces ('the edge MUST be readable from the source side').
severity: nit
resolution: Reworded comment to 'the edge MUST be readable from the source side for the reverse-direction save to be considered correct'. Phrased as the contrapositive the assertion enforces.
status: addressed
---
