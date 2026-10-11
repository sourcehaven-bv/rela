---
id: RR-PGHTI9
type: review-response
title: listLineage DISTINCT can keep an arbitrary hi
finding: One vseq can appear with two hi values; FinishLineage keeps an arbitrary one.
severity: minor
resolution: min(hi) grouped by vseq on both backends.
status: addressed
---
