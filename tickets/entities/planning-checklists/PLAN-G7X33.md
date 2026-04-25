---
id: PLAN-G7X33
type: planning-checklist
title: 'Planning: Relation pickers should display name + id, not id alone'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope:
- `frontend/src/components/forms/RelationPicker.vue` only.
- Two display sites in this component: (a) selected-entity chips at the top, (b) dropdown candidate items.

Out of scope:
- `RelationCards.vue` and `LinkExistingModal.vue` — confirmed by user.
- Configurable display-property; entity list views; non-form pickers.

**Acceptance Criteria:**

1. **AC1 — Selected chip with title:** When an entity has a non-empty `properties.title`, the selected chip shows `Title (ID)` (e.g. `Fix login bug (TKT-YR7OW)`). Verified by mounting the component with a value and an entity that has a title.
2. **AC2 — Selected chip without title:** When an entity has no title, the selected chip shows just the id (e.g. `TKT-YR7OW`) — no `()` and no duplication. Verified by mounting with an entity whose `properties.title` is missing/empty.
3. **AC3 — Dropdown items consistent:** Each candidate row in the dropdown uses the same `Title (ID)` / `ID` format on a single line, replacing the existing two-span layout. Verified by typing a search query and inspecting the dropdown.
4. **AC4 — Search still matches by id:** Typing the id in the search box continues to surface the matching candidate (e.g. `TKT-` matches a ticket). Verified by the existing e2e test in `e2e/pages/form.page.ts:96-98` which searches by `targetId`.
5. **AC5 — Type label preserved:** The type pill (`<span class="entity-type">`) stays where it currently is in both chip and dropdown — only the textual id/title rendering changes.

## Research

- [x] Searched for existing helpers
- [x] Checked codebase for similar patterns
- [x] Reviewed relevant concepts

**Existing Solutions / Patterns:**

- `RelationPicker.vue:100-102` already has `getEntityLabel(entity)` returning `title || id`. The fix is a one-line tweak there plus minor template changes.
- `RelationCards.vue:130` and `LinkExistingModal.vue:119` use the same `title || id` fallback (out of scope for this ticket).
- E2E pattern at `e2e/pages/form.page.ts:96-98` searches the dropdown by `targetId` substring — the new `Title (ID)` rendering still contains the id, so the existing selector continues to work.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns
- [x] Alternatives considered
- [x] Dependencies identified — none

**Technical Approach:**

Replace the single `getEntityLabel` helper with a `formatEntityLabel(entity):
string` that returns:

```ts
const title = String(entity.properties.title ?? '').trim()
return title ? `${title} (${entity.id})` : entity.id
```

Template changes in `RelationPicker.vue`:

1. **Selected chip (~line 161):** Replace `{{ getEntityLabel(entity) }}` with `{{ formatEntityLabel(entity) }}`. The chip already has the type pill — no further change.
2. **Dropdown item (~lines 195-197):** Collapse the existing two-span layout (`entity-id` + `entity-label`) into a single span using `formatEntityLabel(entity)`. Drop the now-unused `.entity-id` style (or leave it; doesn't matter — the template won't reference it). Keep the type pill.

Function rename also clarifies that the value now includes the id.

**Files to modify:**

- `frontend/src/components/forms/RelationPicker.vue` — script + template + small style cleanup.

**Alternatives considered:**

- Keep two spans in selected chip (one for id, one for title): rejected — chips are narrow, single-line `Title (ID)` reads better and matches the user's request.
- Add a configurable display-property at the metamodel level: rejected — out of scope, no current need.

## Security Considerations

- [x] Input sources identified — `entity.properties.title` is project-controlled (markdown frontmatter), already rendered as text via `{{ }}` interpolation (Vue auto-escapes). Same trust model as today.
- [x] No new sinks. No new validation needed.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified
- [x] Integration test approach defined

**Test Scenarios:**

Add a new Vitest unit test file
`frontend/src/components/forms/RelationPicker.test.ts` (no existing tests for
this file — adds floor coverage). Cover:

- AC1: render with a selected entity whose `properties.title` is set → chip text contains `Title (ID)`.
- AC2: render with a selected entity whose `properties.title` is missing → chip text equals the id, with no parentheses.
- AC3: render with a search query that produces a candidate → dropdown item text matches `Title (ID)` or `ID`.
- AC4: existing e2e in `e2e/tests/forms.spec.ts` (uses `form.page.ts:96-98`) — already searches by id substring. We rely on this pre-existing coverage as the integration test for AC4 (no new e2e needed; the change preserves the contract).

**Edge Cases:**

- `properties.title` set to empty string `""` → fall back to id (treated as "no title" via `.trim()`).
- `properties.title` set to whitespace `"   "` → fall back to id.
- `properties.title` is non-string (defensive): `String(...)` coerces; `.trim()` runs on the result.
- Long title that overflows chip width: existing chip styling clips/wraps the same as today (no regression — same span).

**Negative Tests:**

- Empty title with `String(undefined)` → currently returns `"undefined"` because of `String(...)`. The new helper uses `??` (nullish coalescing) before `String`, so `undefined` → `''` → `id`. Add a test for this.

## Risk Assessment

- [x] Technical risks: minimal
- [x] Security risks: none
- [x] Effort: **xs–s** (one component, ~10 lines + test file)

**Risks:**

- E2E selectors that target `.entity-id` or `.entity-label` separately could break. Mitigation: grep showed no e2e selectors using `.entity-id` / `.entity-label` inside `.dropdown-item` — only `:has-text()` against the rendered string. ✅ safe.
- Visual regression in narrow form columns. Mitigation: manual smoke test in browser with the dev server.

## Documentation Planning

- [x] N/A — internal UI tweak, no user docs.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: single-file template change with format pre-agreed with user; risk too low to warrant separate design review)
- [x] All critical/significant findings addressed in plan (none — no design review)
