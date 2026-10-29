---
id: RR-0OYT8O
type: review-response
title: Duplicate test cases in TestBusy_States
finding: 'internal/realtime/busy_test.go:160-163 ("queued") and :193-196 ("never moved, initial not working") have the same type and setup. Fix: drop one of them, or make "queued" an explicit move into queued (for example claim, ask, answer, then a transition back to queued, if the config allows).'
severity: nit
resolution: Dropped the queued duplicate and renamed the remaining case to never moved, initial queued.
status: addressed
---
