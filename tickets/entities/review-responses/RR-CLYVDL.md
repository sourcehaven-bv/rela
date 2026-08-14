---
id: RR-CLYVDL
type: review-response
title: Missing test for negative out-of-range int literal
finding: typed_coercion_test.go exercises the +/-2^53 overflow guard only for the positive case. The negative-literal fold makes `entity.count > -9007199254740993` reachable; coerceIntLiteral's guard is symmetric (n.v <= -maxExactIntLiteral) and correct, but add the mirror reject case so a future fold refactor can't silently drop the negative bound.
severity: nit
resolution: 'Added the negative out-of-range case to typed_coercion_test.go TestTypedComparison_CoercionErrors: ''entity.count > -9007199254740993'' is rejected, pinning that the negative-literal fold still hits the symmetric ±maxExactIntLiteral guard.'
status: addressed
---
