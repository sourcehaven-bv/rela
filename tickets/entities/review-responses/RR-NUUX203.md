---
id: RR-NUUX203
type: review-response
title: Ordered ops chosen by enum position in graphdiff
finding: graphdiff picked ordered operators with p.Op >= PropGreaterEqual, which breaks when an operator is added.
severity: nit
resolution: It now checks an explicit list of the ordered operators.
status: addressed
---
