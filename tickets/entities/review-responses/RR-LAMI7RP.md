---
id: RR-LAMI7RP
type: review-response
title: applyViewCondition doc says every current-user form is false on Nil
finding: 'cranky: not true for ~=, which is true on Nil; the reasoning still holds because the verdict is the same on every row.'
severity: nit
resolution: 'Addressed: the doc now says the verdict on Nil is the same on every row and notes the ~= case and why the pushdown sends hidden fields to Go.'
status: addressed
---
