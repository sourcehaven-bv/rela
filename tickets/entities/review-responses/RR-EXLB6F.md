---
id: RR-EXLB6F
type: review-response
title: rela.xml.text exponential on hand-built cyclic tables
finding: Depth cap bounds recursion but not work; a node whose 10 children are itself takes 10^64 visits.
severity: significant
resolution: textWalk with a node budget (MaxElements) and byte cap, checked in the child loop too. Test with the 10-way self-reference.
status: addressed
---
