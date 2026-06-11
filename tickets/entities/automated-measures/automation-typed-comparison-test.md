---
id: automation-typed-comparison-test
type: automated-measure
title: 'Test: automation when/validate comparisons are type-aware'
description: 'Regression for BUG-YZ2BK0: asserts a when: count>9 condition fires numerically for count=10 with the metamodel wired (and does not on a string-only engine), validate: count<9 warns for count=10, and an undeclared property still matches via string fallback. Fails if automation comparison reverts to string-only filter.MatchValue.'
kind: test
location: internal/automation/typed_comparison_test.go (TestEngine_WhenCondition_IntegerComparisonIsNumeric, TestEngine_Validation_IntegerComparisonIsNumeric, TestEngine_WhenCondition_UnknownPropertyFallsBackToString)
status: active
---
