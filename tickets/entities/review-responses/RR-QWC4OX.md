---
id: RR-QWC4OX
type: review-response
title: Migration 0003 builds indexes non-CONCURRENTLY — write stall on large tables
finding: 0003 creates entities_seq_idx/relations_seq_idx on already-populated tables. pgstore.Migrate runs all migrations in ONE tx under an advisory lock, and CREATE INDEX CONCURRENTLY cannot run in a tx block. So this takes a write-blocking SHARE lock for the build duration. It's the first migration to index a table that may hold production data — a large-dataset upgrade gets a write stall.
severity: significant
resolution: 'Documented the lock behavior explicitly in 0003_sync.sql: the build is non-concurrent by necessity (the single-tx runner can''t do CONCURRENTLY), so a large-dataset upgrade should apply during a maintenance window. A concurrent-index migration path would require restructuring the runner — noted as the alternative, not built. Accepted as a documented deployment constraint.'
status: addressed
---
