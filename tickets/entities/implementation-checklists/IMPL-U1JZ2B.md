---
id: IMPL-U1JZ2B
type: implementation-checklist
title: 'Implementation: Scannable detail-page field layout: single-column default + authored span (views and forms)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Commit `573374d8`. Config -> Go -> wire -> CSS, plus the shared stylesheet.

**Go**: `Span` on `ViewSectionField` and `FormField` (disjoint structs);
`SpanColumns` + `validateSpan` wired into `validateViews`/`validateFormField`;
`SectionFieldData.Span`; a new `buildSectionFieldData` helper replacing the two
duplicated construction sites.

**A fourth copy the design review missed**: `v1.SectionField`
(`internal/apiwire`) is reached by a direct struct conversion, so the compiler
flagged it the moment `SectionFieldData` changed. Worth noting — that conversion
is what keeps the internal DTO and the wire surface in lockstep.

**Frontend**: `styles/properties-list.css` (the 12-column grid),
`utils/fieldSpan.ts` (the only place a span becomes CSS), span threaded through
`SectionEditForm` / `PropertyDisplay` / `FieldShell` / `DynamicForm`.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

`assertSpans` compares against a property-name map, and errors on an
*unexpected* field too — a silently added field would otherwise pass.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Ran the real server (prototype project, editor role) and drove it in Chrome. The
prototype `data-entry.yaml` now authors spans on both the ticket view and the
create form, so the demo exercises the feature rather than just the default.

**AC 1 — single-column default.** Fields with no span render `span 12`, one per
row. Confirmed by computed style, not eyeball.

**AC 2 — spans on views AND forms.** Measured the actual rendered rows on
`/form/create_ticket` by bucketing children by `getBoundingClientRect().top`:

```text
[ title * (span 12) ]
[ description (span 12) ]
[ priority * (span 4) | Reported by * (span 4) | Assign to (span 4) ]
[ due_date (span 6) | estimated_hours (span 6) ]
[ belongs to (span 12) ]
[ tagged (span 12) ]
```

Detail page verified separately: six fields that were one ragged row with an
orphan are now three aligned rows of two, sharing one vertical axis.

**AC 3 — load-time validation.** `validateSpan` covered for 0/1/4/6/12 (valid)
and 13/99/-1/-12 (invalid), asserting the message locates the field and states
the range. **The documented error string was verified byte-exact against the
real output**, not transcribed by hand: `form "create_ticket": field[3]: span 13
is out of range (must be 1-12, or omitted for full width)`

**AC 4 — both DTO sites.** `TestSectionFieldSpan_SurvivesBothConstructionSites`
**caught a real bug on first run**: I had refactored `buildSectionEntityData`
but not the entry-source branch in `buildSections`, so spans worked on cards and
silently vanished on the detail page. Exactly the failure the design review
predicted (RR-OYENHV). Fixed; both sites now share the helper.

**AC 5 — default view.** Auto-generated views author no spans, so every field is
`span 12` — the single-column default is the unconfigured case.

**AC 6 — row behaviour.** Unfilled remainder stays empty (two `span: 5` leave 2
columns blank); a non-fitting field wraps; at 560px every field is full width,
verified by screenshot — a `span: 4` date input on a phone would be unusable.

**AC 7 — one stylesheet.** The three forked `.properties-list` definitions are
gone. `SidePanel` keeps a `--compact` modifier because it is a narrow rail where
the grid genuinely doesn't apply, but it now shares the label/value typography
instead of forking it. Two `text-transform: uppercase` rules disappeared with
the forks, so labels are no longer SHOUTED on the detail page while being
sentence-case in forms.

**AC 8 — ACL unchanged.** `sectionEditFields` tests green; the writable/display
split is untouched; `InaccessibleField` still renders in its own grid cell, so a
hidden field does not reflow the row.

**A bug found and fixed during verification:** converting `.form-fields` to a
grid initially broke the form badly — `FormFieldList` also emits
`RelationCards`/`RelationPicker`, which carry their own root class and so became
auto-width grid items. The first fix (`.form-fields > * { grid-column: span 12
}`) then silently swallowed every authored span, because it has equal
specificity to `.form-field` and won on source order. Diagnosed from computed
style — `--field-span: 4` was present while `grid-column` computed to `span 12`.
Both rules now read the same custom property.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The DRY work is the point of the ticket, not incidental: two duplicated Go
construction sites collapsed into `buildSectionFieldData`, and three forked CSS
blocks into one stylesheet.

Security: `span` is project-authored config that reaches CSS. It is validated to
an integer 1-12 at load and clamped again in `fieldSpanStyle`, which returns
`undefined` for anything non-numeric — a value can never be interpolated into a
stylesheet. A test asserts a hostile string (`'4; background: url(evil)'`) is
rejected.

Verification: `go test -race` ok, `golangci-lint` 0 issues, frontend 1479/1479,
typecheck clean, prettier + markdownlint clean.
