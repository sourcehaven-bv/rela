---
id: IMPL-77SB4J
type: implementation-checklist
title: 'Implementation: Postgres derived-schema reconciler (seam + unique rule): atomic unique:true, db status drift, dry-run'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (TestUniqueIndexName_*, TestMapUniqueViolation, TestValidateSchemaName, TestMapUniquePropertyConflict)
- [x] Integration tests written (test full flow, not just units) — pgstore-gated TestDerivedUnique_* against a real Postgres
- [x] Happy path implemented (reconcile create/enforce; write rejection → property 422)
- [x] Edge cases from planning handled (empty/absent exempt, cross-schema, drop-on-removal, pre-existing dupes, hash truncation)
- [x] Error handling in place — reconcile degrades to unenforced+warn (never fatal); write errors mapped, not swallowed

## Test Quality

- [x] Using fixture builders (person() helper, newReconciledStore() factory, personSpec)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects (uniqueIndexName recomputed, not literal)
- [x] Property comparisons use the returned outcome/error, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Built the postgres CLI (`go build -tags postgres`) and ran against a live
Postgres.app database:
- `db reconcile --dry-run` on a schema with `unique:true` → `+ would create unique constraint on person.email`, **exit 1** (the CI gate).
- `db reconcile` → `+ created unique constraint on person.email`, exit 0.
- `db reconcile --dry-run` when converged → `Derived schema: up to date (1 unique constraint(s) enforced)`, exit 0.
- Seeded two duplicate `email` rows, dropped the index, re-ran `db reconcile` → `! NOT enforced ... (1 duplicate value group(s))`, **exit 0** (no crash — AC5, load-bearing).
- `db reconcile --dry-run --show-values` → revealed the blocking value `dup@x.com` (opt-in only).
- `db status` with pre-existing dupes → printed the derived drift AND exited 0 (migrations current — informational).

Automated: all pgstore-gated tests pass against Postgres
(RELA_TEST_DATABASE_URL); full pgstore conformance suite green (unchanged store
contract); all touched packages pass under `-race -shuffle=on`.

## Quality

- [x] Code follows project patterns (mirrors startVersionSweepIfSupported build-tagged optional-capability wiring; advisory-lock convention; type-asserted optional store capability)
- [x] Checked for DRY opportunities — DDL helpers are package functions (not methods, to respect the god-object line); the reconcile planner is shared by store-open/status/dry-run (one plan(), three surfaces)
- [x] No security issues introduced — DDL-injection guarded (charset validator + literal escaping); enumeration-oracle guarded (count by default, values opt-in, pgErr.Detail never propagated)
- [x] No silent failures — reconcile failures logged with the blocking values; write errors returned mapped
- [x] No debug code left behind (spike files removed)
