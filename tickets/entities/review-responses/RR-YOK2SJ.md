---
id: RR-YOK2SJ
type: review-response
title: Syntactic leaf check does not prove the lowering is exact
finding: ConstEqualities dedups per attribute (prefilter.go:150) and conditionEqualities silently drops leaves failing the metamodel gate, current_user.tool, and empty literals (queryplan.go:374-387). Once the store answer is final, each dropped leaf widens the result.
severity: significant
resolution: 'Plan: LowerScope accounts leaf by leaf; exact only if every AND-spine leaf yields exactly one pushed predicate. Two leaves on one attribute, eq.List, gated-out or empty-literal leaves decline. Added to the AC4 negative table.'
status: addressed
---
