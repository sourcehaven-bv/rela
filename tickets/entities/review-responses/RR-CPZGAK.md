---
id: RR-CPZGAK
type: review-response
title: LISTEN/NOTIFY is database-global — per-test schemas cross-talk on 'rela_changed'
finding: 'Design-review verification: PostgreSQL NOTIFY channels are DATABASE-global, not schema-scoped. The conformance harness (testdb_test.go) gives each test its own schema on ONE shared database. Two tests running in parallel would both LISTEN on the constant channel ''rela_changed'' and each would receive the OTHER''s notifications — cross-test contamination, and flaky multi-writer assertions. A fixed channel name ''rela_changed'' will fail under parallel test execution (and is also wrong in production if two unrelated rela databases ever share a server — they don''t, since the channel is per-database, but the per-SCHEMA test isolation is the real break).'
severity: critical
resolution: 'Implemented the schema-qualified channel: resolveChannel() derives the channel as feedChannelPrefix + current_schema(), resolved identically by the producer (pg_notify) and the listener (LISTEN), via pgQuoteIdentifier. Each test owns a schema, so the channel is per-test isolated; production processes of one deployment share a schema => same channel. Documented the deployment constraint (all processes must share DB+schema) in GUIDE-postgres-backend.md. Verified by TestChannelIsolationAcrossSchemas (writes in schema A not seen by a listener in schema B).'
status: addressed
---

## Resolution (plan update)

**The channel name must be scoped to the logical store instance group, not a
constant.** Options:
- **Schema-qualified channel** (chosen for tests): derive the channel from the
current schema, e.g. `rela_changed_<schema>` via `quote_ident('rela_changed_' ||
current_schema())` at LISTEN time and the same in the producer's pg_notify.
Since each test owns a schema and production runs in one schema (public or the
DSN's), this gives per-test isolation for free and is correct in prod (all
processes of one deployment share the schema => same channel).
- Producer and listener MUST compute the channel identically. Resolve the schema
once (the search_path's first entry) at Open and pass the channel name into both
the notify helper and the listener. Channel names are limited to 63 bytes
(identifier limit) — the schema-prefixed name fits (schemas are short).

This also documents a real deployment constraint: all rela processes that should
see each other's writes must share the same schema (they do — they're the same
project's DB). Add to docs.
