---
id: RR-CMQ75CI
type: review-response
title: '[security] A kept change is taken without re-checking guard and claim'
finding: '[security] A change kept while the state settled was taken later even if its guard no longer held or another person had claimed the step.'
severity: minor
resolution: settleStep re-evaluates the guard on the current subject and the claim (keptChangeHolds) and drops the event otherwise; a test pins the guard case.
status: addressed
---
