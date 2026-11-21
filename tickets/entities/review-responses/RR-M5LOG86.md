---
id: RR-M5LOG86
type: review-response
title: Absent-key truth should not import graphquerynaive
finding: Use a table over the ops LowerCondition emits instead of a visibility->graphquerynaive dependency.
severity: minor
resolution: absentLeaf is a closed table over the comparisons LowerCondition emits; any other comparison is refused. No graphquerynaive dependency.
status: addressed
---

## Finding

Use a table over the ops LowerCondition emits instead of a
visibility->graphquerynaive dependency.
