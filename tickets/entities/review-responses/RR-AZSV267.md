---
id: RR-AZSV267
type: review-response
title: Width caps, nested prefixes, cast safety and duplicate negation
finding: Cap leaves and relation leaves; nest CTE prefixes; check int range before casting in postgres; forbid Pred.Negate inside Where; reuse propmatch.Integer in affordances bindings.
severity: minor
resolution: MaxCondDepth, MaxCondLeaves and MaxCondRelations cap trees; Pred.Negate is rejected inside Where; postgres checks integer range before casting; affordances coerceInt now uses propmatch.Integer.
status: addressed
---
