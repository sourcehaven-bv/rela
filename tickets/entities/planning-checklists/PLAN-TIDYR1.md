---
id: PLAN-TIDYR1
type: planning-checklist
title: 'Planning: Body inline edit: start editing from a sticky pencil button, not from a click on the text'
started: "2026-10-05"
completed: "2026-10-05"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

In scope: `RlInlineEdit` `trigger="explicit"` (only consumer: `EntityBody.vue`,
the entity body on the detail page). Content clicks stop starting the edit; the
pencil button becomes sticky. Story and docs comments updated.

Out of scope: `trigger="value"` inline edits (property values, badge), which are
small single-value buttons and do not have the selection problem. No keyboard
shortcut for "edit body". No change to the editor itself.

**Acceptance Criteria:**
1. Click / double-click / triple-click on body prose does not open the editor; selection works.
2. Hover shows the pencil; clicking it opens the editor.
3. On a body taller than the viewport, the pencil stays visible at the top-right of the body while scrolling (below the mobile sticky topbar).
4. Tab reaches the pencil; Enter/Space opens; Escape returns focus to it.
5. Links, task checkboxes, comment highlights and diagrams keep working.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~ (N/A: small UI change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A

**Existing Solutions:**
- `RlInlineEdit.vue` already renders an edit button for `explicit`, absolutely positioned top-right, revealed on hover/focus, always shown on `pointer: coarse`.
- Sticky precedent: the Milkdown toolbar (`milkdownEditor.css`) sticks at `top: 0`, and at `64px` below 768px to clear the mobile topbar (`PageLayout.vue` `.page-layout__sticky`).
- Notion / GitHub / Confluence: read view is plain selectable text; editing starts from an explicit control. That is the model adopted here.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**
1. `RlInlineEdit.vue`: remove `onReadClick` and the `INTERACTIVE` selector; the explicit read div no longer starts an edit on click. `cursor: text` becomes `auto`. Update the `trigger` prop doc and the "When the read view is not plain text" header comment.
2. Sticky pencil: wrap the button in a zero-height rail placed first inside the read div: `position: sticky; top: var(--rl-inline-edit-sticky-top, var(--rl-space-1)); height: 0; display: flex; justify-content: flex-end; z-index: 1`. The button drops `position: absolute`. A zero-height rail takes no layout space, so the prose does not shift, and sticky keeps it within the read div's bounds.
3. App side: set `--rl-inline-edit-sticky-top` below 768px to clear the 64px mobile topbar, alongside the toolbar offset.
4. Story `ExplicitTrigger`: update text ("a click on the content does nothing; use the pencil") and make the sample body long enough to show stickiness.

Alternatives rejected:
- Double-click to edit: conflicts with double-click word selection.
- Keep click-to-edit but ignore clicks with a recent mousedown-drag / detail>1: the first click of a double-click has already opened the editor; would need a delay timer, which makes single-click editing feel laggy and is still surprising.
- Modifier-click to edit: undiscoverable.

**Files to modify:**
- `frontend/packages/rela-components/src/components/common/RlInlineEdit.vue`
- `frontend/packages/rela-components/src/components/common/RlInlineEdit.stories.ts`
- `frontend/packages/rela-components/src/components/common/RlInlineEdit.test.ts` (new) or app-side test
- `frontend/src/components/entity/EntityBody.vue` (doc comment)
- `frontend/src/App.vue` or `milkdownEditor.css` neighbour (mobile sticky offset)
- `e2e/tests/` new spec for body edit entry

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input Sources & Validation:** None new; pointer/keyboard events only.

**Security-Sensitive Operations:** None. Write permission still decides whether
the editable wrapper renders (`canEditBody`).

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**
- AC1: unit test (vitest, mount RlInlineEdit explicit): click on read content emits no `edit`. e2e: click and dblclick prose on an entity body; assert editor (`.milkdown`) absent and selection non-empty after dblclick.
- AC2: unit: click pencil emits `edit`. e2e: hover body, click button `Body, edit`, editor appears.
- AC3: e2e: entity with long body, scroll the pane halfway, assert the pencil's bounding box is inside the viewport and inside the body box. Manual check in browser on desktop and mobile width.
- AC4: unit: button is focusable; Escape in editor returns focus to button (existing behaviour, assert stays).
- AC5: existing e2e `checkboxes.spec.ts` and comment specs keep passing.

**Edge Cases:**
- Empty body (placeholder shown): pencil still reachable.
- Short body (one line): pencil does not overlap more than today.
- Read-only user: no pencil, plain content (unchanged branch).

**Negative Tests:** Click on prose must NOT emit `edit` (asserted).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**
- Discoverability drops for users used to clicking the text. Mitigation: hover background on the body remains, plus the pencil on hover.
- Sticky fails if an ancestor sets `overflow: hidden`. Mitigation: verify in browser; the e2e AC3 test catches it.
- Pencil overlaps the first line's right edge, as today. Accepted.

Effort: s

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] ~~Docs-checklist will be created when entering implementation~~ (N/A: no user docs describe body click-to-edit)

**Documentation Impact:** N/A. `docs/data-entry.md` does not describe how body
editing starts; component doc comments are updated in code.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: small, single-component UI change; plan reviewed with user)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** N/A
