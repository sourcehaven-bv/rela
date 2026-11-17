---
id: RR-R15R2DT
type: review-response
title: Nil ordered comparisons as false widens ACL and automations
finding: Changing the predicate engine globally so a nil ordered operand is false (not an error) flips not/or conditions in ACL grants, write rules, flow guards, triggers, realtime guards and computed properties, which fail closed today.
severity: critical
resolution: Reverted the global change. NilOrderedIsFalse is an opt-in EvalOption; Evaluator.LoweringSemantics uses it only for surfaces that are lowered (view conditions, query scopes, MCP filter). ACL, write rules, flows and triggers keep the error and fail closed.
status: addressed
---
