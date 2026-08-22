---
id: RR-Y151OZ
type: review-response
title: M1-vacuous-dst-test
finding: TestDaysBetween_DSTSpan used midnight endpoints where UTC truncation and local-time truncation both give 6. It passed regardless of the behaviour it claimed to pin and would not have caught a regression to local-time truncation.
severity: minor
resolution: 'Replaced with endpoints where the two implementations disagree (UTC 5 vs local 4): 2026-11-05 23:30 EST and 2026-11-01 01:00 EDT. Verified discriminating before committing.'
status: addressed
---
