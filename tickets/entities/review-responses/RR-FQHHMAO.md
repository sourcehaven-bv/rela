---
id: RR-FQHHMAO
type: review-response
title: Superset independence from LoweringSemantics untested
finding: 'Code review: ConditionSuperset''s godoc claims soundness without LoweringSemantics but only one semantics was tested.'
severity: minor
resolution: TestConditionSuperset_KeepsEveryMatch also checks rows the default-semantics evaluator matches.
status: addressed
---
