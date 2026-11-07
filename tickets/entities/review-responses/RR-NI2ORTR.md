---
id: RR-NI2ORTR
type: review-response
title: 'Code: test gaps (relation range plan, tricky numbers, self-loop)'
finding: No EXPLAIN test for relations_element_idx; elementEntity had no numbers whose JSON form differs between encoders; the rename step had no self-loop.
severity: minor
resolution: Added a relation range plan check on both backends, the numbers 1e21, 1e-7 and 0.30000000000000004 to elementEntity, and a FEAT-001 depends-on FEAT-001 self-loop that the rename moves.
status: addressed
---
