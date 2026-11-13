---
id: RR-OGWC2FH
type: review-response
title: Evaluator lock taken per clause
finding: e.evaluator() locks evMu on every clause evaluation, although the evaluator is fixed after construction.
severity: nit
reason: Unchanged from the old engine, which took the same lock per clause. The lock is uncontended and recurrence_condition_test replaces the evaluator after construction, which a cached copy would ignore. Not worth the churn in a no-behaviour-change refactor; M5c removes this engine.
status: wont-fix
---
