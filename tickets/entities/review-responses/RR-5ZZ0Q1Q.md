---
id: RR-5ZZ0Q1Q
type: review-response
title: Renumber records of siblings were still shown
finding: A drag reorder writes a renumber record for each sibling, shown as 'Changed a link' on every neighbour.
severity: minor
resolution: changeRecord leaves out records whose trigger starts with renumber:; the managed-names test covers it; docs updated.
status: addressed
---
