---
id: PLAN-13H5L2C
type: planning-checklist
title: 'Planning: Audit service seam and SQL audit log: in-transaction, hash-chained, verifiable'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `sqlaudit` (shared SQL over `sqlexec`), `pgaudit`, `sqliteaudit`,
pg migration 0050, sqlite schema v41, appbuild wiring of a database sink on the
sqlite and postgres builds, `rela audit verify|prune|import`, docs. Out:
appending inside the write transaction and failing closed (TKT-TU0VZ4Y), read
API and Changes tab (TKT-X1ZFO04), SIEM forwarding (TKT-AB4XKWT).

**Acceptance Criteria:**
1. On sqlite and postgres, entity writes produce rows in `audit_log` (TestSQLiteAuditGoesToTheDatabase).
2. The chain verifies, and an edit, deletion or relink of any row or column is reported at the right seq (sqlaudittest EditedRowBreaks, ColumnEditsBreak, DeletedRowBreaks, RelinkedRowBreaks).
3. Concurrent appends from several processes form one gapless chain (ConcurrentAppends with a second pool on pg).
4. Prune keeps the rest verifiable via an anchor and records itself; below 1095 days needs a flag (PruneKeepsChainVerifiable, TestAuditPrune_RetentionFloor).
5. Import appends JSONL files in name order and records itself (Import, TestAuditCmds_SQLite).
6. Each tenant schema has its own chain (pgaudit TestSchemaIsolation).

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** RES-HVW3B7O

**Existing Solutions:**
- `sqlflowrun`/`pgflowrun`/`sqliteflowrun` and `sqlrecords`: one SQL implementation over `sqlexec` with thin adapters; reused as the model.
- Hash-chained logs as in Certificate Transparency and pgAudit-style append-only tables; no library needed for SHA-256 chaining.
- Concept `audit-log`.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Gapless `seq`, hash over the previous hash and every
column, chain lock per schema (pg advisory xact lock; sqlite single writer).
appbuild replaces `Config.Audit` with an async `sqlaudit.Sink` on the database
builds, because `Audit.Record` has no context and may run inside the store
transaction. Alternatives (in research): keep JSONL and ship files, or a
separate audit database; rejected because they cannot share the write
transaction in TKT-TU0VZ4Y.

**Files to modify:** internal/audit/{audit,prepare,jsonl}.go,
internal/audit/{sqlaudit,pgaudit,sqliteaudit}/,
internal/store/pgstore/migrations/0050_audit_log.sql,
internal/sqlitedb/{sqlitedb,migrate}.go,
internal/appbuild/{appbuild,auditlog*,configloader_sqlite}.go,
internal/cli/{audit,kong}.go, .go-arch-lint.yml, .testcoverage.yml, docs-project
guides.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Audit records come from the write path and pass
`audit.Prepare` (sanitize) as in the JSONL sink. All SQL is parameterized. `rela
audit import` reads operator-named files; lines over 1 MiB or invalid JSON stop
the import with the file and line number.

**Security-Sensitive Operations:** Prune deletes audit rows: 1095-day (three-year) floor
unless `--allow-short-retention`, recorded as `audit-prune`. The chain detects
edits but not a full rewrite by a database writer; the head from `rela audit
verify` must be kept elsewhere (documented).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Shared conformance suite `sqlaudittest.RunAll` run by
sqliteaudit and (DB-gated) pgaudit; sqlite v40 to v41 migration rung; appbuild
sqlite end-to-end; CLI tests on Nop and sqlite.

**Edge Cases:** Empty log, prune of nothing, prune of everything then append,
verify across pages, control characters in a record, concurrent appends, sink
Close twice and Record after Close.

**Negative Tests:** Tampered rows and columns, import with a bad line, keep-days
0 and below 1095, commands on the fs build.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Chain lock serializes appends per schema: one short transaction per
record, acceptable. A schema migration on every existing sqlite and pg database:
additive tables only. Effort: xl across three PRs; this one l.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**
- [x] docs/cli-reference.md - `rela audit`
- [x] docs/audit-log.md - database log, chain, threat model, retention, import

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-Z3BLF6H (addressed), RR-YIE211P (deferred to
TKT-TU0VZ4Y), RR-WOJL98T (wont-fix), RR-YYFYRZ5 (wont-fix)
