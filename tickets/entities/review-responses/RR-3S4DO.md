---
id: RR-3S4DO
type: review-response
title: Hardcoded test strings repeated across rrule cases
finding: format.test.ts repeats 'NOT_A_RULE' twice and 'FREQ=DAILY' across three tests. Per project test guidance (CLAUDE.md > Test Writing Best Practices), extract to local consts at the top of the describe block.
severity: nit
resolution: 'Refactor into an `it.each` table eliminated the duplicated literals: each rrule string appears exactly once as a row in the cases array. The single-element-array test extracts `''FREQ=DAILY''` into a local const.'
status: addressed
---
