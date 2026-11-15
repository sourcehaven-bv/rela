---
id: RR-EMEEQP4
type: review-response
title: The explain test pins a planner choice
finding: TestGraphQueryExplainWindowReadsInIndexOrder asserts the derived index name; at 25 percent selectivity the planner might pick the primary key with a filter instead.
severity: minor
reason: Using the derived index in id order is the behaviour this ticket adds, and the test runs ANALYZE before EXPLAIN. If a planner change breaks it, the test should fail so the plan is looked at.
status: wont-fix
---
