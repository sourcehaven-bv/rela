---
id: RR-GGMO6I2
type: review-response
title: 'AC3 coverage: today() in a write rule or scope'
finding: No test checks that the fixed clock reaches today() in a write rule or a query scope.
severity: minor
reason: 'Deferred to TKT-E2IMTMG: the dogfood suite exercises write rules with dates. The clock is wired through appbuild.WithClock, which those paths read.'
status: deferred
---
