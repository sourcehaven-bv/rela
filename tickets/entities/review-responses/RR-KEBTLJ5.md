---
id: RR-KEBTLJ5
type: review-response
title: Baseline from different runner hardware flags hardware as regression
finding: GitHub runners vary in CPU model; benchstat's significance test cannot tell a hardware difference from a regression.
severity: significant
resolution: 'Addressed: check compares the cpu: lines of both files and skips the comparison with a report note when they differ. TestCompareBench_Notes covers it.'
status: addressed
---
