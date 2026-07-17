---
id: PLAN-PYE29T
type: planning-checklist
title: 'Planning: Reposition Properties auto-save indicator inline in the section heading, hidden when idle'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Problem:** In the entity-detail Properties section, `AutoSaveIndicator` is
absolutely positioned by `SectionEditForm.vue` (`.section-edit-form-indicator`,
`top: -28px; right: 0`). This lifts a persistent gray "saved" checkmark into the
empty top-right corner of the section, detached from content, overlapping the
heading row. The indicator is always visible because `AutoSaveIndicator` renders
`idle` and `saved` identically.

**Scope:**

- IN: the properties-section `SectionEditForm` usage in `EntityDetail.vue`;
placement of its indicator into the section heading row; idle-hidden +
fade-out-after-save visibility behaviour of `AutoSaveIndicator`.
- OUT: the per-row `cards`/`list` `SectionEditForm` instances (own `#indicator`
slot); changing `useAutoSave` save/PATCH logic or its status state machine.

**Acceptance Criteria:** AC1 idle hidden; AC2 inline right-aligned on heading
row while saving; AC3 hold ~1s then fade out ~0.3s; AC4 error persists; AC5 no
cards/list regression. (Full scenarios verified in IMPL-C0LALM / REV-J77CHY.)

## Research

- [x] ~~run `/research`~~ (N/A: small UI change, approach is clear)
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Searched for existing libraries that solve this problem
- [x] Reviewed relevant rela concepts for prior art

**Existing Solutions / prior art:** `useAutoSave.ts` already has the `saving →
saved → (1.2s) → idle` timing (`SAVED_INDICATOR_MS`), so the change is
presentational. Cards/list sections already use the `#indicator` slot (adopted
over `<Teleport>` after the #997 unmount crash; TKT-IHC7C, RR-FC1D/RR-FC2A).

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified

**Technical Approach:** (1) `AutoSaveIndicator` hides when idle (opacity 0 +
0.3s transition), error wins; a11y via a visually-hidden live region. (2)
`SectionEditForm` gains an optional `heading` prop → renders its own
heading+indicator flex row; removed the absolute float. (3) `EntityDetail`
passes `heading` and suppresses the generic `<h2>` for that section (only when
heading is truthy).

**Alternatives considered:** CSS negative-margin (brittle, rejected); template
ref via defineExpose (v-for ref-array pitfall, rejected); `<Teleport>` (#997
crash, rejected).

**Files to modify:** AutoSaveIndicator.vue, SectionEditForm.vue,
EntityDetail.vue + their tests.

## Security Considerations

- [x] ~~Input sources~~ (N/A: presentational-only change, no new inputs)
- [x] ~~Input validation~~ (N/A)
- [x] ~~Security-sensitive operations~~ (N/A)
- [x] Error handling doesn't leak sensitive information — error tooltip shows the
existing message; SR announcement stays generic ("Save failed")

**Input Sources & Validation:** N/A — CSS/template-only; `status`/`error` are
internal reactive state already surfaced by `useAutoSave`.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined
- [x] Integration test approach defined

**Test Scenarios:** AC1-5 (each with a concrete assertion). **Edge Cases:**
rapid re-edit (status-driven visibility, not timer); error mid-fade (error
wins); empty heading (headless path — added post-review, RR-32ARO9); unmount
during saved hold (slot pattern, no crash). **Negative:** save error → no
fade-out.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (N/A)
- [x] Effort estimated: **s**

**Risks:** idle-hidden is global to `AutoSaveIndicator` (also affects cards/list
— desired hidden-until-needed intent); heading+indicator on one row without a
hack (mitigated by the form owning its own heading row).

## Documentation Planning

- [x] User-facing docs identified

**Documentation Impact:**

- [x] N/A — cosmetic UI behaviour, no documented feature/API surface changes.
(docs-checklist DOCS-4KPR9D confirms N/A across code/project/external docs.)

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (Skipped: effort=s, cosmetic UI change with an obvious approach and no cross-subsystem or data-model impact. The design was instead validated interactively with the user before implementation — placement, timing, and wiring mechanism were each confirmed via explicit questions.)
- [x] ~~All critical/significant findings addressed in plan~~ (N/A: no design review run; code-review findings were addressed instead — see REV-J77CHY / RR-32ARO9, RR-4SN00Y, RR-ZE29PY.)

**Design Review Findings:** N/A (design validated with user; code review covered
the implementation — RR-32ARO9, RR-4SN00Y, RR-ZE29PY, RR-95OACT, RR-DNWKY0).
