---
id: PLAN-IF2JJU7
type: planning-checklist
title: 'Planning: Mutable record types'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `mutable: true` per record type; update and delete of one row;
`updated_at`, `updated_by`, `revision` on the row; 409 on a stale revision;
audit lines for changes and deletes; HTTP PATCH and DELETE; migrations for
postgres and sqlite. Out: a general `closed_before` lock, `readonly` fields,
per-row ownership.

**Acceptance Criteria:**
1. A ledger with `mutable: true` fails to load (TestMetamodel_LedgerCannotBeMutable).
2. Update changes fields and ts, bumps the revision and writes an audit line (TestUpdate_ChangesRowAndAudits).
3. Delete removes the row and writes an audit line (TestDelete_RemovesRowAndAudits).
4. A stale revision, a hidden owner, a read-only owner and an append-only type are refused (TestChange_Refuses, TestRecordsHandler_ChangeRow).
5. Every backend keeps the Mutator contract (recordstest Replace, Delete, ConcurrentReplace).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: options were discussed with the project owner in the session)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: extends the in-tree record store)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: optimistic concurrency by revision is a standard pattern)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** The optional `records.Aggregator` capability is the
pattern for `records.Mutator`. `entitymanager.PatchEntity` is the pattern for a
partial change. The audit log in `internal/audit` records the change.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** An optional `records.Mutator` (Get, Replace, Delete) on
the store. The Service checks owner read, owner write and then the revision,
under the same lock as append. The revision is the WHERE condition of the SQL
UPDATE/DELETE. Alternatives rejected: per-field `readonly` and a per-row lock
(more complex than needed now).

**Files to modify:** internal/metamodel/records.go,
internal/records/{records,service,mutate,codec}.go, filerecords, sqlrecords,
pgstore migration 0050, sqlitedb v41, internal/audit, internal/appbuild,
internal/dataentry/records_handler.go, GUIDE-records.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The PATCH body and DELETE query. Fields go
through the same normalizeFields as append; unknown fields are refused.

**Security-Sensitive Operations:** A change needs update on every face of the
owner, like append. A row whose owner is hidden answers 404 like an unknown row,
and the revision is compared only after the access checks, so a 409 never
confirms a hidden row.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See the acceptance criteria. The store contract runs against
file, sqlite and postgres through recordstest.

**Edge Cases:** Concurrent replaces of one revision (one wins); a ts change that
moves the row to another month file; clearing an optional field with null;
deleting the last row of an owner.

**Negative Tests:** Stale revision 409, missing revision 400, invalid value 400,
unknown row 404, hidden owner 404, read-only owner 403, append-only type
refused.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Lost updates (mitigated by the revision). A row losing its history
(mitigated by the audit line). Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-records (docs/records.md): the `mutable` key,
PATCH and DELETE, revision and 409, audit.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: the design was agreed step by step with the project owner in the session; /code-review covers the result)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
