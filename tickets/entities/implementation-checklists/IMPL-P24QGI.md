---
id: IMPL-P24QGI
type: implementation-checklist
title: 'Implementation: Widget override for view section fields (`widget:` on ViewSectionField)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Go: `widget_test.go` (validation: unknown name, whitespace-only, capitalised,
type mismatch, file-outside-properties, RR-4ICH8M unresolved-source case, both
warning kinds, silence when unauthored), `widget_table_test.go` (drift guard),
`sections_widget_test.go` (both construction sites carry Widget).

Frontend: `SectionEditForm.test.ts` +8 (override applied, default preserved,
display arm, hint arm DROPPED per RR-2GBB0V, StatusControl two-axis per
RR-66MT0D, ACL conjunction intact, unknown-name fallback);
`sectionEditFields.test.ts` +4; `registry.widgetTable.test.ts` (drift guard +
AC2 in the right language per RR-693NL9).

E2E: `view-section-widget-override.spec.ts` — 4 specs, all passing.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Go table tests use the `widgetConfig(...)` builder (mirrors the existing
`renderConfig`); frontend reuses the established `mountForm` / `makeFields`
harness; e2e reuses `SEED` rather than literal ids.

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Verified in a real browser via Playwright against a freshly built
`bin/rela-server` (the embedded-SPA binary), not only in unit tests:

| AC | Result |
| -- | ------ |
| 1 `widget: checkbox` clickable + persists | PASS — click fires a PATCH; value survives a reload |
| 2 omitting `widget:` unchanged | PASS — frontend table test over every property type |
| 3 unregistered name errors | PASS — error names the valid set |
| 4 type mismatch errors | PASS — error names property, type, accepted types |
| 5 undeclared property warns, no error | PASS |
| 6 `widget: file` outside `properties` errors | PASS |

The **load-bearing** e2e case is `title` (a string) forced to `textarea`. A
boolean already resolves to a checkbox on its own, so a checkbox-only assertion
would still pass if the plumbing were silently dropped; only the textarea
proves config reached the registry.

Two genuine failures were found and fixed during this verification, both of
which unit tests could not have caught:

1. **A stale `bin/rela-server` (Aug 17)** was running the e2e suite, embedding a
   pre-change SPA. `just ci` does NOT rebuild it. The first e2e run failed for
   this reason alone — worth knowing for the next ticket.
2. **The autosave debounce (800ms)** meant the original spec reloaded before the
   PATCH landed and read a stale value, which is indistinguishable from a
   dropped write. `toggleSectionCheckbox` now waits for the PATCH response.

**Not done:** no human has looked at the page. The assertions are structural
(tag name, enabled state, checked state), so they prove the right widget renders
and works — not that it looks right. Recommend a glance via `just dev` before
merge, as with TKT-HOIX1.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

Patterns followed: `Render`'s thread for the plumbing, `validRelationWidgets`
for the allowlist shape, `inertSectionRenderWarnings` for warn-don't-error.
DRY: `buildSectionFieldData` already unifies both Go construction sites, so
`Widget` landed in both at once; `joinWidgetTableKeys` is a deliberate sibling
of `joinMapKeys` (different value type, same output contract) rather than a
generic rewrite of a widely-used helper.

Security: the only new input is an operator-authored string validated at config
load against a CLOSED allowlist; it selects among components already compiled
into the bundle and never reaches a path, command, or query. The ACL
conjunction is untouched and pinned by a test — widget selection decides WHICH
component renders, never WHETHER it is writable.

Two temporary scaffolds written during debugging were removed (a wire-conversion
probe and an e2e response probe); `git status` is clean of them.
