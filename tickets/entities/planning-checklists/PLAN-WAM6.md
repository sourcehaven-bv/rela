---
id: PLAN-WAM6
type: planning-checklist
title: 'Planning: Analyze view shows ID-derived placeholder instead of entity title'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** The `/analyze` page is supposed to show the entity name/title in
addition to the ID for each issue row. Instead it shows a cosmetic transform of
the ID (first letter capitalized, hyphens → spaces). The placeholder lives in
`getEntityTitle()` in `frontend/src/views/AnalyzeView.vue:107`.

Crucially: the **backend already supplies the title** on every entity-linked
issue. `internal/dataentry/analyze.go` calls `s.Meta.DisplayTitle(...)` in
`analyzeOrphans` (line 124), `analyzeDuplicates` (line 173), `analyzeProperties`
(line 409), `analyzeCardinality` (lines 310/328/350/372), and
`analyzeValidations` for the rule-name title (line 451 area). `handleV1Analyze`
in `api_v1.go:1272` copies `issue.Title` to `APIIssue.Title`, which serializes
as the JSON `title` field. `frontend/src/types/config.ts:231` already declares
`AnalyzeIssue.title?: string`.

The bug is one function: `getEntityTitle()` reformats `entityId` instead of
reading `issue.title`.

**Scope:**

In scope:
- Replace `getEntityTitle()` body in `frontend/src/views/AnalyzeView.vue`
with `issue.title || issue.entityId` fallback.
- Add frontend vitest cases covering the rendering paths.

Out of scope:
- Backend changes — backend already supplies `title`.
- ID Gaps rows — no `entityId`, no entity to title; existing
`entity-empty` `—` rendering applies and is correct.
- Lock affordances / inaccessibility handling (RR-MGJN, deferred).
- Navigation / click semantics.

**Acceptance Criteria:**

1. Entity-linked row renders `issue.title` on the title line and `issue.entityId`
on the ID line.
   - Test: vitest case with `{entityId: 'note-2', title: 'My Note'}` expects
`.entity-title` text = "My Note", `.entity-id` text = "note-2".
2. Entity-linked row with empty title falls back to `entityId` on the title line.
   - Test: vitest case with `{entityId: 'note-2', title: ''}` expects
`.entity-title` text = "note-2".
3. Script-error / load-error row (rule-name in `title`, no `entityId`) renders
the rule name unchanged.
   - Existing test at AnalyzeView.test.ts:81 (`title: 'broken-rule'`) must
continue to pass — it already renders the title cell with the rule name because
the entity-empty branch fires when `entityId` is empty. We'll extend coverage to
assert the title text.

## Research

- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions:**

- TKT-YR7OW (Relation pickers showing name + id): same UX pattern but the
frontend lookup goes through `entitiesStore` for relation candidates. That
ticket lives in widget code where the entity-summary lookup is the only source
of truth. Our case is different — backend ships the title in the wire payload,
so no store lookup needed.
- `s.Meta.DisplayTitle` is the backend's single-source-of-truth helper; used
in `entityToV1` (`api_v1.go:1296`) and across all analyze sections. The frontend
just needs to honour the field it produces.
- No external library considered — the change is one line.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Replace the body of `getEntityTitle(issue: AnalyzeIssue): string` with:

```ts
function getEntityTitle(issue: AnalyzeIssue): string {
  return issue.title || issue.entityId
}
```

Drop the now-stale comment about "v1 from entity properties" — it's done (the
backend supplies it). Keep the function rather than inlining `issue.title ||
issue.entityId` at the template call site so that a future change (e.g. trim,
locale formatting) has one place to land.

**Alternatives considered:**

- Inline `{{ issue.title || issue.entityId }}` in the template — rejected.
Keeping the function preserves the indirection point and matches existing
pattern `getEntityTypeLabel`.
- Fetch the entity via `entitiesStore` from the frontend — rejected. The
backend already ships the title; an extra fetch is wasteful and adds a failure
mode.
- Add a separate computed for the title slot — rejected. The function is
pure and trivial; a computed would over-engineer this.

**Files to modify:**

- `frontend/src/views/AnalyzeView.vue` (line 107 — body of `getEntityTitle`,
plus removal of the stale comment).
- `frontend/src/views/AnalyzeView.test.ts` (add cases for title rendering).

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:**

- `issue.title`: comes from `DisplayTitle` on the backend, which already
produces a human-readable string from entity properties. Rendered via Vue's text
interpolation (`{{ }}`), which HTML-escapes by default — no XSS surface.
- `issue.entityId`: backend-controlled, alphanumeric + hyphen by entity-ID
convention. Also rendered via text interpolation.

**Security-Sensitive Operations:** None. No new I/O, no new API, no new trust
boundary.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

| AC # | Vitest scenario |
|------|-----------------|
| 1 | Issue with `entityId: 'note-2', title: 'My Note'` — assert `.entity-title` = "My Note", `.entity-id` = "note-2" |
| 2 | Issue with `entityId: 'note-2', title: ''` — assert `.entity-title` = "note-2" |
| 2 (alt) | Issue with `entityId: 'note-2'` (title omitted) — assert `.entity-title` = "note-2" |
| 3 | Issue with `entityId: '', title: 'broken-rule', scriptError: {...}` — assert `.entity-empty` rendered (no entity-title/-id), because `<template v-if="issue.entityId">` is false. (Verifies we don't regress the script-error rendering path.) |

**Edge Cases:**

- Title equal to ID (e.g. entity has no custom title and `DisplayTitle`
fell back to ID server-side): renders ID twice — once in the `.entity-title`
line, once in the `.entity-id` line. Acceptable per AC 2 interpretation; better
than reformatting and losing user intent. Documented in the test as accepted
behaviour.
- Title with special characters (`<`, `&`, quotes): Vue text interpolation
HTML-escapes, so no XSS. Not adding a specific test for this since it's a Vue
invariant — text interpolation is the test we already trust.
- Empty `title` and empty `entityId`: the outer template guard
`<template v-if="issue.entityId">` prevents the title/id pair from rendering at
all; the `entity-empty` `—` is shown. No new code path.

**Negative Tests:**

- The existing test at `AnalyzeView.test.ts:97` (regular-violation
navigation) implicitly covers the entity-row rendering path. We will extend the
test file rather than replace it.

**Integration approach:** vitest renders `AnalyzeView` with mounted
`mount(AnalyzeView, { attachTo: document.body })` and a stubbed `analyze` API —
that's the same pattern as the existing tests, which is integration- shaped
(full component tree, all CSS classes, real Pinia store). No backend changes
mean no e2e test is required for this ticket.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *Risk:* The placeholder produced a stable readable string even when
`title` was empty (e.g. "Tkt jmis"); replacing it with `entityId` shows the raw
ID, which is slightly less pretty. *Mitigation:* Acceptable — the prettified ID
was misleading (no relation to actual title) and the fallback path triggers only
when `DisplayTitle` itself returns empty, which is rare and indicates
missing-title data the user should see plainly.
- *Risk:* Some entity types might be configured so `DisplayTitle` returns
the ID (no `primary` field). User then sees ID in both lines. *Mitigation:*
Already the backend's expected behaviour — `DisplayTitle` is the canonical
answer. Out of scope to override.

**Effort:** xs — one-line code change plus 3 vitest cases. Estimate stands at
`s` on the ticket only because it includes the full review pipeline.

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — visible behaviour change in `/analyze`; no docs page documents
the placeholder behaviour. No CLI / API / CLAUDE.md changes.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (Skipped: one-line frontend fix; replaces a placeholder body with `issue.title?.trim() || issue.entityId`. Design review's purpose is to catch architectural mistakes before implementation; no architectural surface here)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: design review skipped; code review via cranky-code-reviewer ran in the review phase instead — 0 critical/significant findings)

**Design Review Findings:** N/A — design review skipped for this scope (see strikethrough rationale above).
