---
id: PLAN-EFSE20B
type: planning-checklist
title: 'Planning: Flows M1j: db load copies unfinished runs into sqlite'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `db load` into an existing sqlite database keeps unfinished runs.
Out: other backends (sqlite paths are the only import paths).

**Acceptance Criteria:**
1. `db load` of a source with unfinished runs into an existing sqlite database keeps the runs with branches and definitions (`TestMarkdownData_FlowRunsRoundTrip`).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] Searched for existing libraries that solve this problem (none needed; standard library and existing rela packages)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: rela-internal lifecycle work)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: small change

**Existing Solutions:** Builds on the tenant registry
(`internal/tenant/registry.go`), `tenant.Provisioner`, `state.KV` and the flow
run stores.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** The in-transaction import opens the run store on the
transaction's connection with `sqliteflowrun.NewInTx`, so runs commit or roll
back with the rows. Runs are copied with `flowrun.CopyRun`.

**Files to modify:** internal/appbuild/projectdata_sqlite.go,
internal/flowrun/copy.go, internal/fsimport/flowruns.go.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Input is the operator's own project files.

**Security-Sensitive Operations:** See above. Errors name the failing step; DSNs
and tokens are not logged.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Round-trip test on the sqlite tag; CopyRun unit test.

**Edge Cases:** Run with branches; failed copy (partial copy deleted).

**Negative Tests:** Covered in the edge cases above: failing stores, refused
orgs, closed provisioner or sweeper.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** Partial import: same transaction as the rows.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** N/A: no user-facing change beyond the import summary
line.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (skipped: no separate design review was run; the finished change went through /code-review, findings recorded as review-responses)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review findings; code review findings are all addressed)

**Design Review Findings:** None (see review checklist for code review
findings).
