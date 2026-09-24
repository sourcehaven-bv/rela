---
id: RR-H636MD
type: review-response
title: Ranking before filtering costs a sort under selective worlds
finding: The inner DISTINCT ON now ranks every candidate face before Props trims.
severity: minor
reason: Inherent to correct semantics; the id semi-join pre-filter is recorded for when a world-scoped list measures slow on the perf seed. Default-world plans are unchanged (EXPLAIN tests pass).
status: deferred
---
