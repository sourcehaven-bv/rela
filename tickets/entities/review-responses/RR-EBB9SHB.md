---
id: RR-EBB9SHB
type: review-response
title: Generic plan ran the CTE semi join as a nested loop
finding: 'Measured: (id, face) IN (SELECT ... FROM text_hits) ran as a nested-loop semi join under a generic plan. It took 4.5 s for gateway.'
severity: significant
resolution: 'Fixed: the matching faces travel as an array on a JOIN. TestWorldSearchExplain fails on a semi join.'
status: addressed
---
