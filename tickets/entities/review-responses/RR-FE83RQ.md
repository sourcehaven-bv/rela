---
id: RR-FE83RQ
type: review-response
title: A PermitsRead regression surfaces as a misleading 403
finding: The row gate's answer feeds the comment:read permission floor, so a regression returns 403 naming the permission rather than the reported 404.
severity: significant
resolution: Doc comment says so; confirmed by the PermitsRead mutation, which returns 403 'Reading comments requires the comment:read permission'.
status: addressed
---
