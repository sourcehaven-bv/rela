---
id: PLAN-DXJVBH
type: planning-checklist
title: 'Planning: Version sweep selects only rows without a stored hash'
started: "2026-10-09"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: the sweep candidate query on pgstore and sqlitestore, triggers
that keep the stored hash honest, a write-back guard, migrations (pgstore 0021,
sqlitedb v15). Out: a restored relation dedups against its own delete version
(pre-existing, unchanged; noted on the ticket).

**Acceptance Criteria:**
1. The sweep scan uses the partial index on unhashed rows on both backends (EXPLAIN).
2. A version written or purged outside the sweep makes the row a candidate again (storetest `VersionWrittenOutsideTheSweep`, trigger tests).
3. The existing `SweepBacklog` cases keep passing.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small follow-up; approach set in RR-N2MXHR)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: database triggers and an index)
- [x] Checked codebase for similar patterns or reusable code (migration 0020 trigger, sqlitedb contentHashDDL)
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal invariant)
- [x] Reviewed relevant rela concepts for prior art (store-backends, .claude/rules/versioning.md)

**Research Doc:** N/A

**Existing Solutions:** Extends the content_hash column and update trigger from
BUG-1DWMYO.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** A non-NULL content_hash means the current lifecycle's
latest version has that hash. Triggers clear it on a version insert with another
hash or op delete, on a version delete, and on a live insert carrying a hash.
The write-back also requires the latest version to be a non-delete with the same
hash. The sweep selects content_hash IS NULL; partial indexes serve it.
Alternative rejected: clearing the hash from each Go version writer, which a new
writer could forget.

**Files to modify:** pgstore sweep.go, purge.go, migrations/0021; sqlitestore
sweep.go, purge.go; sqlitedb sqlitedb.go, migrate.go; storetest sweepbacklog.go;
tests; .claude/rules/versioning.md.

## Security Considerations

- [x] ~~Input sources identified (user input, config, external APIs)~~ (N/A: no new input)
- [x] ~~Input validation approach defined (allowlist preferred over blocklist)~~ (N/A: no new input)
- [x] Security-sensitive operations identified (file access, auth, crypto) (trigger SQL uses format('%I') on TG_TABLE_SCHEMA, no user input)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no new error paths)

**Input Sources & Validation:** None.

**Security-Sensitive Operations:** Version purge: the purge's version delete
clears the hash, and the tombstone keeps purged content from being re-captured.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
1. EXPLAIN on both backends after sweeping 5,000 rows.
2. storetest `VersionWrittenOutsideTheSweep/{EntityUpdate,EntityDelete,RelationUpdate}`; trigger tests per backend.
3. Full store suites with -race.

**Edge Cases:** Version on another face; same-hash version; purge of the latest
version; a delete version written between the candidate read and the write-back.

**Negative Tests:** Mutations: triggers without the hash condition, write-back
without the latest-version guard.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] ~~Security risks assessed (see Security Considerations)~~ (N/A: no new attack surface)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A path that breaks the invariant hides a row from the sweep.
Mitigated by keeping the checks in the database, not in each writer, and by the
write-back guard.

## Documentation Planning

- [x] ~~User-facing docs identified (skip if internal refactor)~~ (N/A: internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: refactor)

**Documentation Impact:**
- [x] N/A - Internal change, no user-facing docs needed (.claude/rules/versioning.md updated)

## Design Review

- [x] Run `/design-review` before starting implementation (cranky-code-reviewer over the design and diff)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-N2MXHR (origin); review findings recorded on the
implementation review
