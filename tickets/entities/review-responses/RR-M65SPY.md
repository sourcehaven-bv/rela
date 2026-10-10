---
id: RR-M65SPY
type: review-response
title: 'Premise wrong: #1811 already pinned the defect'
finding: 'SweepBacklog/UnchangedSaveAfterForceLivePurge (added by #1811) fails on the old column order; the earlier local check used a -run pattern that missed the nested subtest.'
severity: significant
resolution: 'Ticket description, why3 and prevention corrected: the new case is a narrower test isolating the purge-then-sweep path, not the first one.'
status: addressed
---
