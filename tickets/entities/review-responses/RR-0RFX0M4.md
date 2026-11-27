---
id: RR-0RFX0M4
type: review-response
title: 'A when: join no token moved is never settled'
finding: Settle after commit only covers touched runs and OnCreate.Settle skips forked runs.
severity: significant
resolution: After commit every forked run on the subject with arrived branches is settled through a path that does not skip forked runs.
status: addressed
---
