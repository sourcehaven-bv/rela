---
id: RR-X5FMH7N
type: review-response
title: queryFields ignored control flow and scope
finding: A key assigned anywhere in the function (an if branch, after the call, another scope) exempted the call.
severity: significant
resolution: 'Only assignments in statements before the call in each enclosing block count, in order; whole reassignment resets fields. Tests: key set on one branch only, key set after the call, variable reassigned without key, key inside the call''s own block.'
status: addressed
---
