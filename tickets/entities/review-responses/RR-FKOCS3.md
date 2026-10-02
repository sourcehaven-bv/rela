---
id: RR-FKOCS3
type: review-response
title: Per-role analysis ignored the implicit everyone role
finding: Every principal holds everyone; omitting it understates joins and can overstate visible fields.
severity: significant
resolution: Role set is {role, everyone}; AC5 includes an everyone-join case.
status: addressed
---
