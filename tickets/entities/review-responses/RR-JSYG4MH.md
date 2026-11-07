---
id: RR-JSYG4MH
type: review-response
title: 'Design: shared range plan so backends only render SQL'
finding: pg and sqlite would each re-derive type ordering, fold classification and the face intersection, and Archplan flags such duplication.
severity: significant
resolution: 'Plan changed: store.PlanElementRange validates the scope and range and classifies each type (fold-all, pend-all, per-row) and each relation type; elementnaive and both SQL backends execute the same plan.'
status: addressed
---
