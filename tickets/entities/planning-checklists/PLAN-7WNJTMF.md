---
id: PLAN-7WNJTMF
type: planning-checklist
title: 'Planning: Computed group keys for data-entry lists'
started: "2026-11-24"
completed: "2026-11-24"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: condition-based `group_by.groups` on data-entry lists,
server-side `_group`, SPA sections, docs, todo-app example. Out: computed groups
on kanbans, dashboard cards and flyouts beyond the shared grouping code; reader
time zone for `today()`.

**Acceptance Criteria:**
1. Rows carry `_group` = id of the first matching group (handler test).
2. Fallback group takes the rest; without one, rows go to Other (handler + vitest).
3. Config errors at load (validation table test).
4. Cost flat in rows (budget test).
5. A condition over a hidden field cannot place a row by its value (ACL test).

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: reuses the list `condition:` machinery)
- [x] ~~Searched for existing libraries~~ (N/A: internal config surface)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal config surface)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** list `condition:` (conditionlint.CompileViewConditions,
dataentry.applyViewCondition, MatchPage); nav `status:` rules (ordered, first
match); `group_by` groups/buckets.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** `ListGroup` gains `id` and `condition`. Conditions
compile at load under kind `list_groups`. The list GET with `list_id` evaluates
groups over the served page through `MatchPage` on the redacted rows (the same
redactor the list condition uses), first match wins, and stamps `_group`. The
SPA splits by `_group` in declared order. Rejected: a free-form `key:`
expression (a scalar expression has no declared order or labels); evaluating in
the SPA (would need a second predicate engine and would judge unredacted values
only on the client).

**Files to modify:** internal/dataentryconfig/{groupby,validate_groupby}.go,
internal/conditionlint/viewcondition.go,
internal/dataentry/{viewcondition,comment_counts}.go plus a new listgroups.go,
internal/apiwire/v1/responses.go, frontend useListGrouping.ts, listParams.ts,
types/config.ts, types/entity.ts, docs-project data-entry guide.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** data-entry.yaml (operator config): ids match
`^[a-z0-9_-]+$`, unique, at most 20 groups, fallback last; the request supplies
only `list_id`, never an expression.

**Security-Sensitive Operations:** Derived per-row value: computed after the row
gate, the list condition and paging, on the redacted row, per request, never
cached. A hidden field binds Nil, so placement carries no bit of its value.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see acceptance criteria; handler tests through the HTTP API.

**Edge Cases:** empty page; no list_id (no `_group`); a row matching several
groups; no fallback.

**Negative Tests:** duplicate id, bad id, fallback not last, two fallbacks,
value and condition mixed, condition with property/relation/buckets,
non-compiling condition.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** UTC `today()` differs from the reader's day near midnight:
documented. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** docs-project data-entry guide (generates
docs/data-entry.md).

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** Second redaction point (significant): groups reuse
the list condition's redactor helper on the served page only; the response
redaction stays at serialization, so no redacted copy is served (addressed). UTC
`today()` (significant): kept and documented, because the request asks for one
consistent zone and every condition surface uses UTC (wont-fix with reason).
ETag staleness (significant): list responses carry no ETag (checked, N/A). Group
cap 20 and budget test with `related()` (minor, addressed). `_group` carries the
id only (nit, addressed).
