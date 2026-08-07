---
id: PLAN-J86M7L
type: planning-checklist
title: 'Planning: Scannable detail-page field layout: single-column default + authored span (views and forms)'
status: done
---

<!-- @managed: claude-workflow v1 -->

> **Split note.** This checklist originally planned all three themes together.
> After design review the work was split into three PRs under FEAT-OJ8L0H:
> **TKT-8VVBRI** (PR 1, design tokens), **this ticket** (PR 2, layout — the
> primary ask), **TKT-8GUI60** (PR 3, icons). The research and findings below
> are retained in full because they informed all three; sections marked
> *(PR 1)* / *(PR 3)* moved with those tickets.

## Understanding

- [x] Problem/requirements clearly understood
- [x] Scope defined (what's in/out documented below)
- [x] Acceptance criteria documented with specific test scenarios

**Scope (this PR):** the detail-page field layout — single-column default plus
authored `span` on a 12-column grid, across **views and forms**, and unifying
the three duplicated `.properties-list` definitions.

Post-review directive from the user: **"correct version always"** — where a
finding offered a smaller-now/correct-later split, the correct version is built.
Hence forms are in scope alongside views.

**Depends on TKT-8VVBRI (PR 1).** The three `.properties-list` components carry
19 radius/font literals between them; landing tokens first avoids churning the
same declarations twice.

OUT of scope: tokens (PR 1), icons (PR 3), label humanization (separate PR), ACL
verdict logic, mobile work beyond the collapse rule.

**Acceptance Criteria:** 10 criteria on the ticket; test scenarios below.

## Research

- [x] ~~For larger features: run `/research` to create a structured research doc~~
(N/A: options are narrow and documented inline; the layout model was settled by
direct user direction with a reference implementation supplied.)
- [x] Searched for existing libraries that solve this problem
- [x] Checked codebase for similar patterns or reusable code
- [x] Looked for reference implementations in other projects
- [x] Reviewed relevant rela concepts for prior art

**Research Doc:** N/A — see inline findings.

**Reference implementation — Filament (user-provided).** A 12-column grid where
each field declares a `columnSpan`, defaulting to full width, labels above
inputs. City / State / Zip share a row because each declares a 1/3 span, not
because the viewport fit three boxes. This is the model adopted. Note it is a
**form** builder — which is why forms are in scope (RR-OYENHV).

**Prior art in the codebase:**

- `SettingsView.vue:1944` — existing `minmax(160px, max-content) 1fr` grid.
- `TKT-W3OPRX` — precedent for consolidating triplicated CSS into a shared
stylesheet; `styles/back-button.css` — precedent for the shared-file pattern.
- `ValidateConfig` (`validate.go:120`) — strict two-phase load-time validation
with did-you-mean suggestions; the convention `span` must follow.
- Widget registry models `WidgetMode = 'display' | 'edit'`
(`widgets/types.ts:13`), covered by `widgets.test.ts`.
- *(PR 1)* `tokens.css` theme-only boundary; the Go `appTypographyCSS` contract.
- *(PR 3)* Lucide vs Heroicons vs Phosphor; `getIconEmoji` allowlist shape.

## Approach

- [x] Technical approach chosen and documented
- [x] Approach builds on existing patterns (not reinventing)
- [x] Alternatives considered (document why rejected)
- [x] Dependencies identified (packages, APIs, types)

### Root cause

`SectionEditForm.vue:304` — the property block is not a grid:

```css
.properties-list { display: flex; flex-wrap: wrap; gap: 16px 32px; }
.property-item   { display: flex; flex-direction: column; min-width: 200px; }
```

A wrapping flex row of grow-able `min-width: 200px` items: column edges differ
line to line, width is independent of content, and the wrap point is viewport
arithmetic — hence the orphaned `ESTIMATED HOURS`. **Adjacency is implicit; that
is the defect.** `.properties-list` is also redefined in three components
(`SectionEditForm` 200px, `PropertyDisplay` 120px + `.property-long`,
`SidePanel` a third), scoped so they never share.

Correction to an early read: fields render as controls **not** because display
mode is missing, but because `SectionEditForm` is per-field — `row.writable`
picks `mode="edit"`, else `mode="display"` (lines 242-272). In the prototype
every field is writable for `alice`. Inline autosave editing is a real
ACL-driven feature; **must not be removed.**

### Chosen approach

- CSS: `.properties-list { display: grid; grid-template-columns: repeat(12, 1fr) }`;
`.property-item` defaults to `grid-column: span 12`, overridden per field.
Labels above fields.
- Config: `Span int` on **both** `ViewSectionField` (`config.go:574`) and
`FormField` (`config.go:176`) — disjoint structs.
- Wire: `SectionFieldData` (`sections.go:56`) carries `Span`, populated at
**both** construction sites — `sections.go:186` and `:228`. Factor the
duplicated literal blocks while there.
- Validation: **load-time error** for span outside 1..12 via
`validateViews`/`validateForms`, indexed message. Frontend independently falls
back to full width for out-of-range values from hand-crafted responses.
- Row behaviour (specified, not deferred): unfilled remainder stays empty; a
non-fitting field wraps; below a defined container width all items collapse.
- `default_view.go:46` authors no spans, so single-column is the common case
and must look good unaided.
- Unify the three `.properties-list` definitions into one shared stylesheet.

**Alternatives considered:**

- *Two-column `minmax(label, max-content) 1fr` label/value grid* — the original
proposal. Fixes alignment but bakes in one rigid shape, cramps long values, and
one long label starves the value column. **Rejected** for the Filament model at
the user's direction.
- *Content-proportional widths* (heuristic on value length) — **rejected**:
width follows declared grouping, not a guess about content.
- *`repeat(auto-fit, minmax(300px, 1fr))`* — same viewport-dependent wrap.
- *Frontend-only span map per entity type* — hardcodes layout in the SPA.
- *Views-only spans* — offered post-review; **rejected** ("correct version
always"): the Filament reference is a form builder.

**Files to modify:**

- `internal/dataentryconfig/config.go` — `Span` on `ViewSectionField` +
`FormField`
- `internal/dataentryconfig/validate.go` — span range validation
- `internal/dataentry/sections.go` — `SectionFieldData.Span`, both sites
- `internal/dataentry/default_view.go`, wire converter
- `frontend/src/api/views.ts` — `span?: number`
- `frontend/src/styles/` — shared properties-list stylesheet
- `frontend/src/components/forms/SectionEditForm.vue`, `DynamicForm.vue`
- `frontend/src/components/common/PropertyDisplay.vue`
- `frontend/src/components/forms/SidePanel.vue`
- `prototypes/data-entry/project/data-entry.yaml` — demo the span model
- `docs/data-entry.md`, `frontend/CLAUDE.md`

## Security Considerations

- [x] Input sources identified (user input, config, external APIs)
- [x] Input validation approach defined (allowlist preferred over blocklist)
- [x] Security-sensitive operations identified (file access, auth, crypto)
- [x] Error handling doesn't leak sensitive information

**Input sources & validation:** `span` comes from project-authored YAML. Integer
1..12 enforced at load. The frontend emits a clamped value or a class — **never
interpolates raw config into CSS**. A non-integer fails yaml unmarshal before
validation; pin that message in a test.

**Security-sensitive operations:** None new — presentation-layer work.
Invariants preserved: (a) ACL-driven `row.writable`/`inaccessible` gating
untouched — a layout change must never make a read-only field editable; (b)
`InaccessibleField` stays visually distinct.

**Layout-as-oracle — resolved, not merely mitigated (RR-P2QU85):** git-crypt
inaccessible fields are *rendered* as a lock placeholder, not removed
(`sectionEditFields.ts:96`), so they hold their grid cell and nothing reflows.
`visible:`-redacted properties are dropped from the wire, but CLAUDE.md is
explicit that field-level redaction hides **values only** and makes no claim to
conceal which properties exist (the metamodel is served over the API). A grid
gap therefore leaks nothing already public. No `v-html` introduced.

## Test Plan

- [x] Test scenarios documented for each acceptance criterion
- [x] Edge cases identified and documented
- [x] Negative test cases defined (invalid input, error conditions)
- [x] Integration test approach defined (not just unit tests)

**Test scenarios (AC -> test):**

1. *Single-column default* — a section with no spans renders every item at
`span 12`; no two items share a row.
2. *Authored spans, views AND forms* — three `span: 4` fields occupy one row on
a view section and on a form.
3. *Span validation* — `span: 0`, `13`, `-1` -> load-time
`ConfigValidationError` with an indexed message; `span: "abc"` -> pinned yaml
unmarshal error; frontend falls back to full width for an out-of-range API
value.
4. *Wire round-trip on BOTH sites* — Go test that `Span` survives
YAML -> config -> `SectionFieldData` -> view JSON for a detail-page section
**and** a card/list section.
5. *Default view* — `default_view.go` output legible with zero spans.
6. *Row behaviour* — two `span: 5` leave the remainder empty; `span: 8` then
`span: 6` wraps; narrow container collapses all to full width.
7. *Shared stylesheet* — all three former `.properties-list` surfaces render
from the unified rules.
8. *ACL unchanged* — `sectionEditFields` tests stay green; a non-writable field
still renders `mode="display"`; `InaccessibleField` holds its cell.
9. *Themes* — screenshots of detail/list/form in light and dark.
10. *No regression* — `npm run test:run`, `typecheck`, `lint`,
`go test ./...`, `/e2e`.

**Edge cases:**

- Entity with 1 property; with ~20 properties.
- Long single-line value; multi-line/long-text value.
- Empty/null values — the grid cell holds its column, no collapsed row.
- Mixed writable/non-writable in one section — edit and display widgets in the
same row align on one baseline.
- Very long property labels (labels now sit above fields — verify benign).

**Negative tests:** invalid span -> load error; a read-only field must not gain
an editable control.

## Risk Assessment

- [x] Technical risks assessed with mitigations
- [x] Security risks assessed (see Security Considerations)
- [x] Effort estimated (xs/s/m/l/xl)

**Risks:**

- *New permanent config surface* (`span` on two structs). Mitigation: one
optional field following a well-understood convention (12-column, Filament),
validated at load, documented in `docs/data-entry.md`.
- *Span dropped on one of two DTO sites* — would work on the detail page and
vanish on cards/lists. Mitigation: AC 4 tests both explicitly.
- *Regressing inline autosave editing.* Mitigation: no changes to
`row.writable`/routing; existing tests plus a manual edit round-trip.
- *Unifying three `.properties-list` definitions changes SidePanel/
PropertyDisplay visually.* Mitigation: verify all three; document intended
deltas (`RR-E6HYNB` was raised previously for exactly this).
- *Merge conflict with PR 1* on the same three components. Mitigation: the
`depends-on` ordering — tokens land first.

**Effort:** l (down from xl after the split; icons and tokens moved out).

## Documentation Planning

- [x] User-facing docs identified (skip if internal refactor)
- [x] Docs-checklist will be created when entering implementation

**Documentation Impact:**

- [x] `docs/data-entry.md` — **required**: document `span` on view section
fields and form fields (12-column grid, default full width, 1..12 validation,
row/wrap behaviour).
- [x] `frontend/CLAUDE.md` — record the shared `.properties-list` stylesheet so
future code doesn't re-fork it.
- [x] ~~docs/metamodel.md, docs/cli-reference.md, README.md~~ (N/A: no
metamodel, CLI, or project-level surface changes.)

## Design Review

- [x] Run `/design-review` before starting implementation
- [x] All critical/significant findings addressed in plan

**Design Review Findings:** RR-GWVGDX (critical, addressed — drove the kanban
`icon:` design, now carried by TKT-8GUI60), RR-IUMZV8 (significant, addressed —
load-time span validation), RR-OYENHV (significant, addressed — second DTO, both
sites, forms in scope), RR-P2QU85 (significant, addressed — row behaviour
specified, ACL oracle disproved), RR-09N4MN (minor, addressed — carried by
TKT-8VVBRI and TKT-8GUI60). All five resolved; none deferred.
