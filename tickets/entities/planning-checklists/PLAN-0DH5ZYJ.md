---
id: PLAN-0DH5ZYJ
type: planning-checklist
title: 'Planning: done_when per step'
started: "2026-11-15"
completed: "2026-11-15"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `done_when: <predicate>` on a list tab of a `mode: steps` page,
compiled at load through `internal/predicate`; a per-principal endpoint `GET
/api/v1/_page_status?page=<id>[&entity=<anchor>]`; a check on the step number in
the step bar. Works on entity pages (rows narrowed to the anchor).

Not in scope: blocking Next, a warning tone or count, done_when on kanban,
calendar, gantt, document or dashboard tabs, a timebox.

**Semantics (decision):** a step is done when its list shows the principal at
least one row and every row it shows satisfies `done_when`. An empty list is not
done: a workshop step whose list is still empty has not started.

**Acceptance Criteria:**

1. `done_when` on a list tab of a steps page loads; on a tabs page, on a
non-list tab, or blank, it fails to load naming page and tab; an expression that
does not compile against the list's type fails to load. Test:
`TestValidatePages` cases; conditionlint test.
2. The sidebar payload carries `done_check: true` on such a tab, and never
the done state. Test: sidebar test.
3. `/_page_status` returns `{steps: {<tab>: {done}}}` for visible tabs with
done_when; done is true only when every visible row matches and there is at
least one. Test: Go table test.
4. Rows hidden from the principal never change the answer: a hidden row that
fails done_when does not make the step undone, and a hidden matching row does
not make an empty list done. Test: Go test with a row-gated policy.
5. One gated read per step, no per-row lookups. Test: budget test with
`fieldPolicyReads`, flat calls at 10 and 50 rows.
6. The step bar shows a check on a done step's number and Next stays enabled
when a step is not done. Test: PageSteps/PageView unit test; e2e.

## Research

- [x] For larger features: run `/research` to create a structured research doc
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A: same shape as the navigation `status:` rules.

**Existing Solutions:**

- Navigation `status:` rules (`internal/dataentry/nav_status.go`): a
predicate per entry compiled by `conditionlint.CompileViewConditions`, counted
per principal over the gated rows of a list by `navStatusRows`, served by
`/_nav_status` apart from the principal-independent sidebar. The done check
reuses all of it.
- `resolvePageScope` narrows a list read to an entity page tab from config.
- `useNavStatus` refetch pattern (route change, debounced `entity:changed`).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Backend:
- `PageTab.DoneWhen`; validated with `walk` in `validateStepListOptions`.
- `dataentryconfig.PageDoneEntries` / `PageDoneConditionID`;
`conditionlint.ViewConditionPageDone` compiles each against its list's type.
- `page_status.go`: for each visible tab with done_when, `configuredListRows`
(the `navStatusRows` body over a query that can carry the page scope), prime the
verdicts, `applyViewCondition`, done = all kept and at least one.
- Errors per step degrade to no entry (logged); a hidden anchor is the
uniform 404.

Frontend:
- `getPageStatus` API, `usePageStatus` composable.
- `PageSteps.vue` takes `done` and draws a check on the number, with
"done" for screen readers.

Alternatives rejected: putting the done state in the sidebar payload (breaks the
principal-independent sidebar); counting failures in SQL (`count` over a
predicate the store cannot always lower would need a Go fallback anyway, and
navigation status already takes this path); judging on the client (would need
every row on the client).

**Files to modify:**

- internal/dataentryconfig/page.go, page_test.go
- internal/conditionlint/viewcondition.go (+ test)
- internal/apiwire/v1/responses.go
- internal/dataentry/page_status.go (new) + tests, nav_status.go,
api_v1.go, requeststats.go, views_handler.go
- frontend/src/api/pageStatus.ts (new), composables/usePageStatus.ts (new)
- frontend/src/components/pages/PageSteps.vue, views/PageView.vue + tests
- frontend/src/i18n/locales/en.json, nl.json
- e2e/tests/pages.spec.ts, fixture
- docs-project/entities/guides/GUIDE-data-entry.md

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `done_when` is operator config, compiled at load; a bad one is a load
error.
- `page` and `entity` query parameters are ids. The page selects its own
declared conditions; the anchor goes through the gated read inside
`resolvePageScope` (hidden and missing are the same 404).

**Security-Sensitive Operations:**

An aggregate over rows: gate, then redact, then judge, as the root CLAUDE.md
requires. The answer is one bit per step over the gated set; a hidden row cannot
change it, so it is no existence oracle. Field `visible:` applies: the condition
judges the redacted row. The response is no-store.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:** see the test named under each acceptance criterion.

**Edge Cases:**

- Empty list: not done.
- A tab hidden by `permission:`: absent from the response.
- A condition reading a field the principal cannot see: judged on the
redacted row (nil), so not done; never a leak.
- Unknown page: 404; entity page without `entity`: 400.

**Negative Tests:** done_when on a tabs page, on a kanban tab, blank, not
compiling; `entity` on a plain page.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- Each judged step reads its whole list in Go (the same full header scan the
navigation status takes, TKT-D3RAB0E). Mitigation: pinned by a call budget test;
a workshop page has a handful of steps.

Effort: m.

## Documentation Planning

For enhancements: identify what documentation needs updating.

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] docs/data-entry.md (via docs-project GUIDE-data-entry): steps section

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** self-review recorded as review-responses on the
ticket: empty-list semantics (decided: not done), and the done state must stay
off the sidebar payload (addressed with `done_check` plus `/_page_status`).
