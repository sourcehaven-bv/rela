---
id: RR-24J6HOS
type: review-response
title: Records could not be appended to a multi-face owner
finding: Writable resolved the owner through WriteTarget, which is ambiguous when several faces are readable, so a faced owner always got a 400.
severity: significant
resolution: 'Rows belong to the family: Writable now requires update on every readable face, and the closed-period check holds on every face.'
status: addressed
---
