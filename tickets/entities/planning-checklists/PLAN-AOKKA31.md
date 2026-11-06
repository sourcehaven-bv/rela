---
id: PLAN-AOKKA31
type: planning-checklist
title: 'Planning: Lua: rela.now timestamp with time of day'
started: "2026-11-06"
completed: "2026-11-06"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:** In: `rela.now` in every Lua runtime, one clock reading for
`rela.today`, `rela.now` and `rela.now_unix`, a `WithClock` option, docs. Out:
automation templates, which already have `{{now}}`; a time zone setting.

**Acceptance Criteria:**
1. `rela.now` is UTC RFC 3339 with second precision (TestClockBindings).
2. The three values agree for one clock reading, including across local midnight (TestClockBindings "just after local midnight").
3. `rela.date_add(rela.now, "1d")` works (TestClockBindings).
4. Without `WithClock` the wall clock is used (TestNowDefaultsToWallClock).

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small change)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: standard library time formatting)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A: small change)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** `rela.now_unix` and `rela.today` in
`internal/lua/runtime.go`; `{{now}}` in `internal/automation/template.go` uses
RFC 3339.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:** Read the clock once in `registerReadBindings`, set the
three fields from it. Add an unexported `clock` field and `WithClock` option.
Rejected: a `rela.now()` function, because the other time values are stamped
fields and a function would disagree with `rela.today` across midnight.

**Files to modify:** `internal/lua/runtime.go`, `internal/lua/now_test.go`,
`docs-project/entities/guides/GUIDE-lua-scripting.md`.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None; the value comes from the server clock.

**Security-Sensitive Operations:** None.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** See acceptance criteria.

**Edge Cases:** Local midnight in a non-UTC zone; year end; sub-second clock
value is truncated.

**Negative Tests:** N/A: the binding takes no input.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A script could confuse the UTC date of `rela.now` with `rela.today`;
the docs state the difference.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** Lua guide "Context" table.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: one-field addition with no design choice beyond the format)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review)

**Design Review Findings:** N/A
