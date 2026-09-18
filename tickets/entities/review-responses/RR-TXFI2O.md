---
id: RR-TXFI2O
type: review-response
title: Bound-parameter CASE loses the index once Postgres switches to a generic plan
finding: 'Design review S9, escalated to critical after measurement. The plan requires enum values to be bound parameters in ORDER BY (never concatenated) while the index DDL must use quoteLiteral, because DDL cannot be parameterised. Postgres matches an expression index by expression equivalence, so a parameterised CASE and a literal-valued CASE index are not obviously the same expression. Measured on Postgres 18, 200k rows: with a CUSTOM plan (executions 1-5) the bound-parameter CASE DOES use the index - 4 buffers, 0.083ms. With plan_cache_mode = force_generic_plan (what pgx reaches after 5 executions of a prepared statement) the SAME query falls to a Parallel Seq Scan: 1,915 buffers, 32.2ms. So the approach works in a test that runs the query once and silently degrades ~480x in production steady state. Mitigation measured and confirmed: emitting the enum values as LITERALS in the ORDER BY CASE (via quoteLiteral, values are operator-authored config) keeps the Index Scan under a forced generic plan at 4 buffers / 0.043ms. Consequences for the plan: (1) the ORDER BY must use literals, not bound params, reversing the plan''s stated rule, so quoteLiteral becomes security-load-bearing on the query path and not only in DDL; (2) the EXPLAIN test must execute the prepared statement at least 6 times or force a generic plan, otherwise it proves nothing about production.'
severity: critical
status: open
---
