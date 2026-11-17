---
id: PLAN-EXBX3JM
type: planning-checklist
title: 'Planning: Read-time today and now in every query surface'
started: "2026-11-16"
completed: "2026-11-16"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `now()` in predicate; one read time per request on ctx
(`internal/readtime`); `$today`/`$now`/`$today±Nd|w` resolved in every
filter-DSL read path (search, dashboard `query:`, nav scope, views/traverse
`where:`, feeds, gantt, CalDAV, mail templates, Lua `list_entities`, scheduler
for_each, CLI `--where`); date vs datetime rule documented (UTC); docs fixes.
Out: date pushdown to the store (follow-up ticket), a project time zone,
read-time derived properties, Lua `rela.today` zone.

**Acceptance Criteria:**
1. `/_search?q=prop:due<$today` returns overdue rows (TestV1Search_DateVariables).
2. `today()`/`now()` read one value per request (TestAttachReadTime, TestReadTimeConditions).
3. A list condition on today()/now() gives the same rows on memstore, sqlite and postgres and stays on the Go path (TestListPushdown_ConditionMatchesGoPath*, TestListCondition_DateRows).
4. `--where due<$today` compiles to `today()`, so a cached program follows the read time (TestFromFilterDateVariables).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: design doc in .ignored covers the options; approach follows existing filter variables)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: internal plumbing of an existing feature)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `dataentry.resolveFilterVariable` already resolved `$today` on the REST filter path; generalized into `filter.ResolveDateVariable`.
- `predicatefns.Bind` already took a `now`; the evaluator now reads it from ctx.
- `queryplan.LowerCondition` already declines ordered comparisons.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Leaf package `readtime` carries the read time on ctx.
The data-entry router stamps it per request; `IdentityBinder` stamps when
absent; `Rebind` replaces. The evaluator binds `today()`/`now()` from it. Filter
callers bind date variables with `filter.BindDateVariables`. Rejected: lowering
date comparisons now (stored dates have no canonical shape; see RR-7LRJ1TO,
RR-OCS1CUV).

**Files to modify:** internal/readtime, internal/filter/datevars.go,
internal/predicatefns, internal/dataentry (router, helpers, queryservice, views,
feeds, gantt, caldav, list), internal/appbuild (identity, rebind, scheduler),
internal/lua/runtime.go, internal/mailtemplate, internal/cli/list.go.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] ~~Security-sensitive operations identified (file access, auth, crypto)~~ (N/A: no new security-sensitive operation; ACL path unchanged)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** Filter values from config and requests. A date
variable must match an anchored allowlist regex (`$today`, `$now`,
`$today±N{d,w}`, N ≤ 6 digits); anything else is a literal, as before.

**Security-Sensitive Operations:** None new. Row gating and redaction run before
conditions, as before.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** unset date (guard), garbage date `TBD`, YAML time.Time values,
non-UTC clock, midnight boundary, offsets in weeks, large offsets refused.

**Negative Tests:** `$today+1m`, `$now+1d`, `$todayx`, `$TODAY`, 7-digit offsets
stay literals.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A filter that relied on `$today` being a literal string (none known).
Date conditions stay on the Go path: no performance change. Effort: m.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: docs updated in the same change; docs-checklist created at review)

**Documentation Impact:** GUIDE-data-entry, GUIDE-cli-reference, GUIDE-mail,
GUIDE-flows.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-7LRJ1TO (deferred to follow-up), RR-OCS1CUV
(deferred to follow-up), RR-5X1BFBE (addressed).
