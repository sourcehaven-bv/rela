---
id: RR-OS2CGB5
type: review-response
title: total can be smaller than the rows served
finding: 'searchplan.go page: the page read and CountMatched are separate statements, so a delete between them can give total < len(rows).'
severity: minor
resolution: 'Addressed: page returns max(total, len(rows)).'
status: addressed
---
