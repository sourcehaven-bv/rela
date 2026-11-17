---
id: RR-8JE0G2Q
type: review-response
title: No test compares the store with the Go evaluator
finding: 'RunGraphDifferential compares SQL with graphquerynaive only. Add a lowering-level differential: random conditions over awkward raw values, Evaluator vs LowerCondition plus a store.'
severity: significant
resolution: 'Added queryplan TestLowerCondition_AgreesWithEvaluator: 300 random and/or/not conditions over int and string props with awkward raw values, Go evaluator with LoweringSemantics vs LowerCondition run on memstore; storetest.RunGraphDifferential holds every backend to graphquerynaive with random Where trees.'
status: addressed
---
