---
id: RR-ZV8VPFU
type: review-response
title: EXPLAIN test pins only the entities arm
finding: The pg plan test asserts entities_id_lower_key only; the marked_entities and retired_ids arms could regress to a seq scan unnoticed.
severity: minor
reason: Those side tables are empty or tiny in the fixture, where a seq scan is the planner's correct choice, so asserting their indexes would make the test depend on planner costing. The indexes exist and were confirmed by hand under a generic plan.
status: deferred
---
