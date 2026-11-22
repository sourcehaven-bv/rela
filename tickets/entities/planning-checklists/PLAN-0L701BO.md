---
id: PLAN-0L701BO
type: planning-checklist
title: 'Planning: Audit record appends'
started: "2026-11-19"
completed: "2026-11-19"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: one `append-records` audit line per `records.Service.Append` and
`Reverse` call; a `denied-write` line when an owner is readable but not
writable; Audit becomes a required `records.Deps` field; GUIDE-records and
GUIDE-audit-log. Out: writing record audit lines inside the record transaction
(TKT-RS31NGP); retention expiry and owner delete/rename (system operations, not
appends).

**Acceptance Criteria:**
1. One append of N rows writes exactly one line (TestAppend_AuditsOneLinePerBatch).
2. A refused append on a readable owner is a denied-write line naming the owner; a hidden owner or invalid row writes nothing (TestAppend_AuditsDenial).
3. The line carries principal, triggered_by and asserted_by from ctx (TestAppend_AuditsOneLinePerBatch).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change, design agreed in discussion)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: uses the existing audit package)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal audit convention)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** Reuses `Service.write` and the denied-write convention
of `internal/records/mutate.go` (TKT-ERO2X5G).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `Append` and `Reverse` call `s.appended` after the
locked write commits and `s.deniedAppend` on error. `writableOwners` wraps
`ErrForbidden` in `forbiddenOwnerError`, which keeps the owner and unwraps to
the same error. Alternatives rejected: per-row lines (volume); a per-type
`audit:` option (batching removes the cost it would manage); in-transaction
lines now (needs a record-store hook, split to TKT-RS31NGP).

**Files to modify:** internal/records/{appendaudit.go,service.go,records.go},
internal/audit/audit.go, tests, GUIDE-records, GUIDE-audit-log.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Rows already validated by the service; the line
holds ids only, no field values.

**Security-Sensitive Operations:** A denied line is written only for
ErrForbidden, which the owner gate returns only for a readable owner; a hidden
owner stays silent, so the log is no existence oracle.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria; plus ledger transactions and
reversal lines.

**Edge Cases:** More than 20 owners: the summary lists 20 and a count.

**Negative Tests:** Hidden owner, invalid field, forbidden reversal.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Audit volume for unbatched writers: mitigated by one line per request
and guide advice to batch. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-records, GUIDE-audit-log (op table).

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: effort s; design discussed and agreed with the user before building)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run)

**Design Review Findings:** N/A
