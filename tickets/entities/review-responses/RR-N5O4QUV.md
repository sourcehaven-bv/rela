---
id: RR-N5O4QUV
type: review-response
title: Mixed-type queries rank against different enums
finding: Each group ranked against its own first row type's enum, so order depended on row order for multi-type queries.
severity: minor
resolution: Declared order applies only when all rows share one type; otherwise count order. Documented in the guide; unit test.
status: addressed
---
