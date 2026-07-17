---
id: RR-HLDQ6H
type: review-response
title: Sweep per-tick working set is uncapped — bulk import / post-migration burst can run for minutes
finding: 'The tick query has no batch cap. After a bulk import or a metamodel migration that touches many entities, the settle filter can match a large fraction of (e.g.) 100k entities in one tick — 100k correlated latest-version probes + per-entity projection-hash computation, a tick running for minutes, which then compounds the advisory-lock/connection-lifetime concerns (RR-FB6QU8). Also the ''latest version per entity'' clause must be written so the planner does per-entity LIMIT-1 index probes (DISTINCT ON / LATERAL) rather than materializing MAX(vseq) GROUP BY over all of entity_versions. Fix: cap the working set with ORDER BY updated_at LIMIT $batch (drain across ticks); force the per-entity-probe query shape; pin the plan with an EXPLAIN-asserting bench like graphquery_explain_test.go / graphquery_bench_test.go.'
severity: significant
resolution: 'Fixed: the per-tick working set is capped (ORDER BY updated_at LIMIT $batch) and drains across ticks, so a bulk-import/post-migration burst can''t produce an unbounded tick. The latest-version-per-entity clause is written as DISTINCT ON / LATERAL LIMIT 1 to force per-entity index probes; an EXPLAIN-asserting bench (modeled on graphquery_explain_test.go) pins the plan. AC-11 covers N≫batch. Tracked as R10.'
status: addressed
---
