---
id: PLAN-CB7K2M
type: planning-checklist
title: 'Planning: CheckboxWidget is unstyled — the only widget with no design tokens'
status: done
---

<!-- @managed: claude-workflow v1 -->
## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope:**

IN: the `CheckboxWidget` **edit** arm's visual treatment, plus whatever the
change does to the **display** arm as a side effect.

OUT: markdown task-list checkboxes in entity content (`- [ ] item`). Those are
rendered by the markdown pipeline, not the widget, and carry their own toggle
behaviour (`utils/checkboxToggle.ts`). Confirmed present on the same page
during verification — 4 of the 5 checkboxes on the TKT-001 detail view — and
deliberately left alone. Restyling them is a separate surface with a separate
risk profile.

OUT: the `display: table` cell rendering of booleans, which routes to text so
values stay Cmd-F searchable (`frontend/CLAUDE.md`).

OUT: behaviour of any kind. No change to checked/unchecked semantics, the
`@change` handler, or the auto-save path.

**Acceptance Criteria:**

1. The edit-arm checkbox carries the same visual language as its grid
   neighbours (token radius, accent colour, focus ring).
   *Test:* computed style in a live browser — `appearance: none`, 18px box,
   `border-radius: 4px` (`--radius-sm`); verified against the running app.
2. Checked state fills with the theme accent and renders a checkmark.
   *Test:* computed `background-color` equals `--accent-color` in both themes;
   `::after` renders at 5×10px rotated 45°.
3. The display (read-only) arm stays visibly muted after `appearance: none`
   removes the browser's native disabled rendering.
   *Test:* unit test pins the `.display-checkbox` hook + `disabled`; verified
   under mutation.
4. No behaviour regression.
   *Test:* full frontend suite; plus a live toggle that must persist to disk.

## Research

- [x] ~~For larger features: run `/research`~~ (N/A: xs styling change)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — xs, single-file styling change.

**Existing Solutions:**

No library needed; this is ~40 lines of CSS.

The decisive find is **prior art already in the tree**:
`components/forms/RelationCards.vue:961-999` (`.inline-edit-checkbox`) is a
fully custom-drawn checkbox used for inline boolean edit — `appearance: none`,
18px box, accent fill, CSS-border checkmark. That is the same job on a
different surface, so the widget should look like it rather than invent a
second boolean style.

Also surveyed:
- `EntityList.vue:1378` — row-select checkboxes, styled only with
  `accent-color`. A lighter approach, but it can't produce the token radius or
  the focus ring, so it under-delivers on AC1.
- `TextWidget.vue` / `SelectWidget.vue` — the grid neighbours whose look this
  must match. Note both still use **raw px** (`6px`, `10px 12px`, `14px`)
  rather than scale tokens; they predate TKT-8VVBRI's token migration.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

**Technical Approach:**

Port `RelationCards`'s `.inline-edit-checkbox` rule set into
`CheckboxWidget.vue`'s scoped `<style>`, keyed on `input[type='checkbox']` so
it covers both arms, and express the values as `scales.css` tokens where a
token exists (`--radius-sm` for the 4px radius) rather than as literals.

The one genuinely new consideration: `appearance: none` is what makes the box
drawable, and it *also* discards the greyed rendering the browser gave the
disabled display arm for free. That treatment was load-bearing (RR-UD2I chose a
real disabled checkbox precisely for its native affordances), so the muted
state has to be drawn explicitly instead of inherited — otherwise a read-only
boolean becomes indistinguishable from an editable one. This is the only part
of the change that is not a copy of existing CSS.

Alternatives considered:
- **`accent-color` only** (the `EntityList` approach). Two lines, no
  `appearance: none`, so the display arm keeps its native greying and AC3
  becomes free. Rejected: `accent-color` cannot set the border radius or the
  focus ring, so the control still reads as an OS widget — it fails AC1, which
  is the entire ticket.
- **Extract a shared checkbox class** into `styles/`, used by both
  `RelationCards` and the widget. Rejected for this ticket: it edits a working
  component to serve a styling change, and the two uses sit at different sizes
  in different layouts. Noted as follow-up instead.
- **Replace the display arm with a glyph/`<span>`.** Rejected outright — the
  ticket forbids it and RR-UD2I documents why (native screen-reader semantics).

**Files to modify:**
- `frontend/src/widgets/CheckboxWidget.vue` — the styles
- `frontend/src/widgets/widgets.test.ts` — guard for the display arm

## Security Considerations

- [x] ~~Input sources identified~~ (N/A: no input is read)
- [x] ~~Input validation approach defined~~ (N/A: no input is read)
- [x] ~~Security-sensitive operations identified~~ (N/A: none)
- [x] ~~Error handling doesn't leak sensitive information~~ (N/A: no errors)

**Input Sources & Validation:**

None. The change is scoped CSS in a single SFC. It adds no props, reads no
config, performs no I/O, and interpolates no value into a selector or a URL.

The one adjacent-to-security property worth stating: the change must not make
a **read-only** control look editable, since that misrepresents an ACL verdict
to the user. That is AC3, and it is why the disabled state is drawn explicitly
rather than left to the browser. It is a correctness/UX property rather than a
vulnerability — a user who clicks a disabled checkbox still cannot write, since
the input carries `disabled` and the server re-checks regardless.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test Scenarios:**

AC1/AC2 are *computed-style* properties. jsdom does not apply scoped SFC
styles, so a Vitest assertion on them would pass against no CSS at all — a
guard that cannot fail. They are therefore verified in a **real browser**
against the running server, and that is stated in the test file rather than
faked with a jsdom assertion.

AC3 is testable as structure: the `.display-checkbox` hook the CSS hangs off.
Pinned by unit test, and the guard was **verified under mutation** (removing
the class from the SFC makes it fail; restoring it makes it pass).

AC4 is covered by the existing 1824-test suite plus a live end-to-end toggle
that must land on disk.

**Edge Cases:**
- Checked *and* disabled — a read-only `true`. Must still read as checked, not
  merely as muted. Covered by the unit test looping both `true` and `false`.
- Both themes — the accent is a token, so light/dark correctness follows from
  using `var(--accent-color)`. Verified in both.
- Edit arm must NOT pick up the display-only muting; pinned by its own test.

**Negative Tests:**
- Removing `class="display-checkbox"` must fail the suite (mutation-verified).
- The edit arm must not carry that class (asserted directly).

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *`appearance: none` silently degrades the display arm.* The main risk, and
  the reason AC3 exists. Mitigated by drawing the disabled state explicitly and
  pinning the hook with a mutation-verified test.
- *Selector reaches further than intended.* The rule is keyed on
  `input[type='checkbox']`, and scoped styles apply only within the SFC — which
  renders exactly one input. Confirmed live: the 4 markdown checkboxes on the
  same page kept `appearance: auto`, so nothing leaked.
- *Divergence from `RelationCards`.* Two hand-maintained copies of one visual.
  Accepted for an xs ticket; recorded as a follow-up rather than pretended away.

**Effort:** xs (confirmed — 2 files, ~40 lines of CSS, ~20 of test).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] N/A — no user-facing docs needed.

Nothing an operator writes, configures, or invokes changes. There is no new
config key, no new widget name, no change to which widget the registry picks,
and no change to behaviour. `docs/data-entry.md` documents the `widget:`
override and the widget set; both are unaffected by how a checkbox is painted.

## Design Review

- [x] ~~Run `/design-review` before starting implementation~~ (N/A: xs
      single-file CSS change with no interface, data-flow, or API surface to
      review; the design question — match `RelationCards` vs `accent-color`
      only — is recorded under Approach with the rejection rationale)
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** None (review not run — see above).
