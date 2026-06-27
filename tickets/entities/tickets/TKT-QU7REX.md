---
id: TKT-QU7REX
type: ticket
title: Filter _analyze results through the ACL read gate (TKT-VQGN follow-through)
kind: refactor
priority: high
effort: m
status: done
---

handleV1Analyze (GET /api/v1/_analyze) runs runAnalysis over the ENTIRE graph
and returns every issue's `entityId`, `entityType`, and `title` to any
authenticated principal with no read-gate filtering (the only gate present is a
loopback check on script-error *detail*, not on the issue list). Confirmed in a
live pen-test: a zero-grant principal enumerated all person + team entities with
titles. Which entities surface depends on which checks trip (orphans,
cardinality, validation, dangling relations, cycles), but the channel is
unfiltered — tickets and their titles leak whenever a check fires on them.

**Fix:** filter each issue through the read gate (`PermitsRead` / batched
`PermitsReadMany` by type) before adding to the response; drop issues for
entities the principal cannot read. Different shape from the GET-style endpoints
(per-issue filtering, not a single gate call), hence its own ticket and
effort=m. Add a read-gate test asserting a denied principal sees no hidden ids
in the issue list.
