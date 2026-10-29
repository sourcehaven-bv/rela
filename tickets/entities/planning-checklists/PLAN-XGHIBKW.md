---
id: PLAN-XGHIBKW
type: planning-checklist
title: 'Planning: Form default: current-user token'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

The ticket assumed list filters accept `$USER`. They do not:
`resolveFilterVariable` (internal/dataentry/helpers.go:48) knows only `$today`,
`$tomorrow` and `$yesterday`; only a comment in `FixedFiltersEditor.vue:14`
claims `$USER`. So this ticket defines one current-user token for both surfaces.

In:
- Token `$user` (exact, lowercase like the date tokens): the signed-in principal's user id (`principal.User`, the `user` of `GET /_me`).
- Form field `default: $user` on a property field (data-entry.yaml): the SPA fills the principal's user id.
- List filter value `$user` (static `filters:` and `filter[p][op]=$user` on `/api/v1/{type}`): the server substitutes the request's principal, also inside `in`/`ne` lists.
- Fix the `$USER` comment in FixedFiltersEditor.vue.

Out: `$user` in a relation filter (`filter[<relation>]`) and in the commands
list input; the list-export script context, which reports filter values as given
(as it does for `$today`); `$user` as a relation default (that needs the user
entity, not the principal id); the commands list input (`applyFilters`, which
skips `$` values today); a way to write a literal `$user` value.

**Acceptance Criteria:**

1. A create form with `default: $user` on a string property opens with the signed-in user id in that field (frontend test with a stubbed `/_me`).
2. If `/_me` answers after the form initialised, the field is filled then, unless the user already typed in it; a `prop.<name>` query parameter still wins.
3. With no identified user (`/_me` fails, or answers `unknown`), the field stays empty.
4. `GET /api/v1/tickets?filter[owner][eq]=$user` as alice returns alice's rows only, through both the pushdown and the in-memory filter path; `in` lists resolve `$user` per member.
5. With no identified principal, a `$user` filter value stays the literal text, so it does not match rows with an empty owner.
6. A list's static `filters:` with `$user` drive the sidebar count (nav_status) per caller.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: effort s)
- [x] ~~Searched for existing libraries that solve this problem~~ (N/A: in-repo token)
- [x] Checked codebase for similar patterns or reusable code
- [x] ~~Looked for reference implementations in other projects~~ (N/A)
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:** date filter tokens (`resolveFilterVariable`,
helpers.go:39-70); `useMe()` (frontend/src/queries/me.ts) for the principal in
the SPA; predicate `current_user.id` (internal/predicatefns/currentuser.go) is
the user-entity notion, deliberately not reused for a property value.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Server:
- `gatedListQuery` gets `user string`, set in `gateListQuery` from `principal.From(ctx).User` when it is neither empty nor `principal.Unknown`.
- `resolveFilterVariable(value, user string)` and `resolveFilterVariablesInList(value, user)` substitute `$user` when user is set; callers in applyV1Filters, filterSetValues and planListPushdown pass `gated.user`.

SPA:
- New `frontend/src/utils/formDefaults.ts`: `resolveDefault(value, me)` returns me.user for exactly `$user` (undefined when me is missing or `unknown`), and the value otherwise.
- DynamicForm uses `useMe()`; initializeDefaults resolves each property default through it and skips an undefined result. A watch on the me data fills `$user` fields that are still empty once `/_me` answers.

Alternative rejected: resolving the default on the server when the form config
is served. The config payload is principal-independent (CLAUDE.md, "the
configuration is not a secret"), and a per-principal config would break that.

**Files to modify:**
internal/dataentry/{helpers.go,listfieldgate.go,api_v1.go,listpushdown.go} and
tests; frontend/src/utils/formDefaults.ts (+test);
frontend/src/components/forms/DynamicForm.vue (+test);
frontend/src/components/configure/FixedFiltersEditor.vue (comment);
docs-project/entities/guides/GUIDE-data-entry.md.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** The token comes from data-entry.yaml or a query
parameter; only the exact string `$user` is replaced. The value is the caller's
own principal id, never another user's.

**Security-Sensitive Operations:** None new. A filter only narrows the rows the
list ACL already allows; resolving `$user` to the caller adds no access. The SPA
prefill is a convenience: the server validates and authorizes the create as
before, so a forged value gains nothing. `/_me` stays display-only; filling a
field is not an access decision.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** Go: table test for resolveFilterVariable(List) with and
without user (AC 4, 5); API test listing with `filter[owner][eq]=$user` as two
users, once on a pushdown-able query and once with `q` to force the in-memory
path (AC 4). Frontend: formDefaults unit test (AC 3); DynamicForm test with
stubbed getMe, immediate and late answer, typed value kept, query param wins (AC
1, 2).

**Edge Cases:** `$User`/`$USER` are not tokens (exact match); `$user` inside a
longer string is not replaced; empty principal.

**Negative Tests:** unidentified principal (AC 3, 5).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:** A late `/_me` answer overwriting typed input; mitigated by filling
only empty fields. An operator who used `$USER` per the old comment gets no
match either way; the docs name the token. Effort: s.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:** GUIDE-data-entry.md: form field table (`default`), and
the filter "Variable substitution" section.

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-W3UY2R1 (minor, relation filters and commands
input out of scope, docs say property filters), RR-BERZK38 (minor, export
context reports the raw value, as for date tokens), RR-MP0DXYR (minor, AC 6 for
nav_status counts). No critical or significant findings.
