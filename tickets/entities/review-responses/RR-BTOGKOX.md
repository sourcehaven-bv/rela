---
id: RR-BTOGKOX
type: review-response
title: trigramIndexable is conservative
finding: Needles such as ab-cd-ef could use the index but keep the plain shape.
severity: nit
resolution: Costs speed only and never correctness. The test table documents it.
reason: It costs speed only and never correctness. A wider rule would need pg_trgm's word-boundary padding rules to stay exact.
status: wont-fix
---
