---
id: RR-R7BDG
type: review-response
title: Migrate not concurrency-safe; schema_version permits multiple rows
finding: 'cranky-code-reviewer #2 + #3: Migrate''s godoc claims ''safe to call on every startup'' but two processes against a fresh DB both read max(version)=0 and both run 0001_init.sql (no IF NOT EXISTS on CREATE TABLE/INDEX), so the loser''s tx aborts and that process fails to start. Open is called from rela, rela-server, AND rela mcp — running mcp + server against one DB concurrently is normal. Compounding: schema_version has no uniqueness constraint and `UPDATE schema_version SET version=$1` has no WHERE, so if >1 row ever exists, all rows are rewritten and currentVersion masks it via max().'
severity: significant
resolution: 'Fixed in migrate.go: (1) the whole migration sequence now runs in ONE transaction guarded by pg_advisory_xact_lock(0x52454c41) — concurrent migrators (rela/rela-server/rela mcp) serialize, and PostgreSQL transactional DDL makes a partial failure roll back entirely. (2) schema_version is now a singleton: CREATE TABLE ... (id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id), version INT NOT NULL) with INSERT ... ON CONFLICT (id) DO UPDATE — structurally forbids a second row, no WHERE-less UPDATE. Godoc updated to claim concurrency-safety accurately. Verified: pgstore conformance (which migrates ~100 fresh schemas) still green with -race.'
status: addressed
---

## Resolution plan (fix now)

1. **Advisory lock the migration** (#2): acquire a single pooled connection,
`SELECT pg_advisory_lock($const)` before reading the version, run the apply loop
on that connection, `pg_advisory_unlock` after. Serializes migrators across
processes; makes the godoc claim true.
2. **Singleton schema_version** (#3): `CREATE TABLE schema_version (id BOOLEAN
PRIMARY KEY DEFAULT true CHECK (id), version INT NOT NULL)` + `INSERT ... ON
CONFLICT (id) DO UPDATE SET version = excluded.version`. Structurally forbids a
second row and collapses the UPDATE-or-INSERT branch.

Both are small and remove the "rare but real" concurrent-startup failure.
