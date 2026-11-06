---
id: RR-9WXUC1D
type: review-response
title: Configure could loosen or remove an enforced rule
finding: Only validations.[].enforce was locked; when/then stayed editable and the rule item could be removed.
severity: significant
resolution: enforce is protective and pins entity_type, faces, when, when_condition, then, then_condition. Pinned by three TestCheckEdit cases.
status: addressed
---
