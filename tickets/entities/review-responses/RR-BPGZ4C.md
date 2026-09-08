---
id: RR-BPGZ4C
type: review-response
title: 'Slow-query tooling: pg_stat_statements needs a server restart; docs claim tsvector search'
finding: The baseline plan names pg_stat_statements, which requires shared_preload_libraries and a restart of the local Postgres.app instance used by other projects. Separately, dropping entities_search_tsv_idx contradicts docs/postgres-backend.md:149 which says search uses a tsvector GIN index; the docs must be corrected in the same change or the drop is confusing.
severity: minor
resolution: Baseline uses ALTER DATABASE ... SET log_min_duration_statement and session_preload_libraries=auto_explain on a dedicated rela_perf database (reload only, no restart); pg_stat_statements is optional. docs/postgres-backend.md search paragraph is corrected to describe the trigram LIKE + similarity strategy in the same change as the index drop.
status: addressed
---
