---
id: RR-WB0R42I
type: review-response
title: No test that distinct Where trees get distinct scans
finding: 'Code review: a scan-key collision would judge one rule over another''s rows.'
severity: minor
resolution: TestEvaluate_DistinctWhereTreesDoNotShareAScan; mutation-checked by dropping the where tree from scanKey.
status: addressed
---
