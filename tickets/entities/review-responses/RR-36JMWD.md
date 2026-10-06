---
id: RR-36JMWD
type: review-response
title: 'Design: do not assert on the implicit default world'
finding: DEC-NPZICR removes ?world=default once worlds are declared. A spec asserting its current behaviour would break on a planned stage.
severity: nit
resolution: No spec names ?world=default; world switching uses the two declared worlds only.
status: addressed
---

DEC-NPZICR removes ?world=default once worlds are declared. A spec asserting its
current behaviour would break on a planned stage.
