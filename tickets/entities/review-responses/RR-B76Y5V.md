---
id: RR-B76Y5V
type: review-response
title: Small and large reals render differently from naive
finding: txtExpr renders a real as its JSON token (0.000001) while graphquerynaive uses fmt.Sprint (1e-06).
severity: minor
reason: Same divergence in both SQL backends; filed as BUG-LCHDSR.
status: deferred
---
