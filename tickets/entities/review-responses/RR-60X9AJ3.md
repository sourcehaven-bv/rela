---
id: RR-60X9AJ3
type: review-response
title: General constant folding in predicate prefilter
finding: Folding arbitrary rhs runs host functions in the prefilter with a different step budget and error behaviour.
severity: significant
reason: 'Not built: date lowering is deferred to TKT-4Z2BIRG. That ticket should fold today(), now() and date_add(today(), N, unit) to a bound parameter in queryplan rather than add general constant folding.'
status: deferred
---
