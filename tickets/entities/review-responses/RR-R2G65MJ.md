---
id: RR-R2G65MJ
type: review-response
title: Size the re-load batch from the room left
finding: 'Code review: Each batch resolves up to 512 refs even when one slot is left.'
severity: nit
reason: Sizing from the room turns hidden violators into one query per remaining slot, an N+1 when many violators are hidden. One batch costs one query whatever its size, so 512 is cheaper in the worst case.
status: wont-fix
---
