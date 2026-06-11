---
id: PLAN-0S83
type: planning-checklist
title: 'Planning: Analyze page warning count out of sync with visible tables (gaps + duplicates hidden)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The summary badge on the data-entry `/analyze` page sums warnings
from all six analyses produced by `runAnalysis()` (Properties, Cardinality,
Validations, Orphans, Duplicates, ID Gaps), but the SPA hardcodes only four
check-type cards (Properties, Cardinality, Validations, Orphans). Duplicates and
ID Gaps inflate the badge but never appear on the page. Reporter's example:
badge said 67 warnings, the page showed 8 rows.

**Scope:**

- IN: render Duplicates and ID Gaps cards/rows so the badge total matches the
sum of per-card counts. Frontend-only change. Add Vue unit-test coverage for the
new sections.
- OUT: filtering/toggling warning categories (GH#785 "ideally" option 3).
- OUT: reconciling CLI vs UI counting semantics for ID gaps. The page can
display per-ID rows; the goal is visibility, not changing the count.
- OUT: backend changes to `runAnalysis()`. The data is already on the wire as
`byCheck["Duplicates"]` and `byCheck["ID Gaps"]`, and individual issues are in
`result.issues`.

**Acceptance Criteria:**

1. Given an analysis result containing Duplicates issues, the page renders a
Duplicates card with the count badge AND a table row per duplicate issue. Test:
vitest mounts AnalyzeView with `makeResult([dup1, dup2])` and asserts both the
count and the rendered rows.
2. Given an analysis result containing ID Gaps issues, the page renders an
ID Gaps card with the count badge AND a row per missing ID. Since gap issues
have no `entityId`/`entityType`, the row uses em-dash placeholders (the existing
template path) and is non-clickable (existing `isClickable` returns false).
Test: vitest mount + assert count + assert rows + assert row is not
`.clickable`.
3. Summary badge total equals sum of all six card counts when all sections
have issues. Test: vitest mount + assert badge text matches
`Object.values(byCheck).reduce(...)`.
4. Existing four sections (Properties, Cardinality, Validations, Orphans)
are unchanged. Test: existing AnalyzeView.test.ts cases continue to pass.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- Backend already returns exactly what we need:
  - `internal/dataentry/analyze.go:70` — `runAnalysis()` returns six sections,
including `Duplicates` and `ID Gaps`.
  - `internal/dataentry/api_v1.go:1267-1284` — handler flattens all section
issues into `result.Issues` and increments `result.ByCheck[section.Name]`.
- The CLI's analyze command (`internal/cli/analyze.go`) has subcommands for
both `duplicates` and `gaps`, so this isn't a missing analysis — it's a
rendering gap.
- The frontend already supports an "entity-less" issue row (em-dash fallback
in template, asserted by the existing `'renders an em-dash when entity cell or
type cell is empty'` test). `analyzeGaps` issues have no entityId/Type, so
they'll use this code path out of the box.
- `isClickable` already handles the "no entity, no scriptError" case: returns
false, so gap rows will be inert. No new logic needed.

**Reference implementations:** the four existing entries in `CHECK_TYPES`
already match the pattern we'll extend.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Frontend-only. Add two entries to the `CHECK_TYPES` array in
`frontend/src/views/AnalyzeView.vue`:

```ts
{ key: 'Duplicates', label: 'Duplicates', description: 'Entities with identical titles' },
{ key: 'ID Gaps',    label: 'ID Gaps',    description: 'Missing numbers in auto-generated ID sequences' },
```

Keys MUST match the backend section names exactly (`section.Name` →
`result.ByCheck[name]`). Verified against `internal/dataentry/analyze.go:103`
("Orphans"), `:137` ("Duplicates"), `:187` ("ID Gaps") — the second has a space,
so `key: 'ID Gaps'` (not `'IDGaps'` or `'id-gaps'`).

Existing template handles the rest: count badge, row rendering, em-dash fallback
for entity-less rows, non-clickable row state. No CSS changes, no new
components, no API changes.

Also update `e2e/tests/fixtures.ts` constant `ANALYSIS_CHECKS` (the e2e suite
asserts the card count and titles via this list — without the bump, the existing
`'shows all check type cards'` test would fail).

**Alternatives considered:**

1. Auto-derive `CHECK_TYPES` from `result.byCheck` keys: rejected because the
curated `description` strings are user-facing copy that needs to be intentional,
and the iteration order would depend on response order.
2. Collapse ID Gap warnings server-side to one row per prefix (matching CLI's
`len(allGaps)` semantics): rejected as out-of-scope per ticket. Also changes API
shape — a breaking change for any other consumer.
3. Exclude Duplicates and Gaps from the summary total: rejected because it
hides information that users may want; the issue's "expected behavior" #1
prefers visibility.

**Files to modify:**

- `frontend/src/views/AnalyzeView.vue` — two entries added to `CHECK_TYPES`.
- `frontend/src/views/AnalyzeView.test.ts` — new test cases for Duplicates,
ID Gaps, and badge-total equality.
- `e2e/tests/fixtures.ts` — extend `ANALYSIS_CHECKS` to six entries.

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined
- [x] Security-sensitive operations identified
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** the new entries only read from the existing
analyze API response, which is already trusted server output. No new user input,
no new validation surface.

**Security-Sensitive Operations:** none. The render path for gap rows reuses the
existing em-dash fallback and non-clickable state — both already exercised by
`'renders an em-dash...'` and `'does nothing when a load-error row...'` tests.
Lua script-error envelope handling is irrelevant here.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:**

- AC1 (Duplicates rendered): mount with `[makeIssue({checkType: 'Duplicates',
entityId: 'T-1', entityType: 'note', severity: 'warning'}), ...]`; assert card
exists, count badge shows 2, two `.issue-row` elements visible.
- AC2 (ID Gaps rendered + inert): mount with `[makeIssue({checkType: 'ID Gaps',
message: 'Missing ID: TKT-005', severity: 'warning'})]`; assert card, count=1,
row exists, row does NOT have `.clickable` class.
- AC3 (badge total matches sum): mount with one issue per check type across
all six categories; assert summary badge text equals 6, equals
`Object.values(result.byCheck).reduce((a,b)=>a+b, 0)`.
- AC4 (existing behavior preserved): existing AnalyzeView.test.ts cases
continue passing — verified by running the suite.

**Edge Cases:**

- `result.byCheck` does not contain a key for one of the six checks (zero
issues): existing template path renders "No issues" — already covered.
- ID Gap issue has empty `entityId` but a non-empty `message`: existing
em-dash + `.entity-empty` path renders correctly — already covered by the
em-dash test, will be reused.

**Negative Tests:** N/A — this is a pure additive rendering change. There is no
new input validation or error path.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- **Visual regression on the analyze page (low):** adding two cards extends
the page vertically. No layout change — same `.check-cards` flex column.
Mitigation: manual verification with seeded data (gaps + duplicates).
- **Key drift if backend section names change (low):** the constant strings
`'Duplicates'` and `'ID Gaps'` must stay in sync with
`internal/dataentry/analyze.go`. Mitigation: the test asserting cards render for
these check types is itself the canary — any rename on the Go side breaks the
test.

**Effort:** s (xs would be appropriate if it were truly a 2-line change, but
test cases + manual verification make it small not extra-small).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — UI consistency fix; no user-facing docs reference the analyze
page's hidden categories. The page itself is self-documenting via the
`description` strings being added.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: trivial 2-entry constant addition with no architectural decisions; alternatives enumerated in Approach section above)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None — this is a 2-entry constant addition with
matching tests. There is no design decision worth a `/design-review` round trip;
the alternatives were enumerated above. If review of the implementation is
preferred, `/code-review` after the change is the right gate.
