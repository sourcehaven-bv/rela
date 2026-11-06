---
id: RR-GALL4FA
type: review-response
title: Guards and templates may read roll-up properties
finding: Roll-ups are folded per reader, so a run would copy or branch on an aggregate over rows its readers may not see.
severity: significant
resolution: 'Templates refuse a roll-up reference. Guards already could not compile one (ScalarTypeForProp excludes roll-ups); pinned by a test. Tests: template rollup, guard rollup.'
status: addressed
---
