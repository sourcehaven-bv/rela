---
id: REV-P5DOCM
type: review-checklist
title: 'Review: Widget override for view section fields (`widget:` on ViewSectionField)'
status: in-progress
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — Go suite green; frontend 1819 tests across 113 files; 8 e2e specs (4 new + TKT-HOIX1's 4, run together against a rebuilt binary)
- [x] Lint clean (`just lint`) — 0 issues. Five findings fixed during implementation (misspell ×2, gocritic unnamedResult, revive unused-parameter, whitespace) plus two more after the review rewrite (misspell, modernize/slices.Contains). `e2e npm run lint` also clean — the page-object rule is NOT covered by `just ci`, and it caught a raw selector in the new spec.
- [x] Coverage maintained (`just coverage-check`) — package floor 50% PASS, total 65% PASS, total 78.0%. Touched packages: `dataentryconfig` 89.1%, `dataentry` 80.9%.

## Code Review

- [x] Run `/code-review` command (invokes cranky-code-reviewer agent) — run twice: a design review before implementation, a code review after
- [x] All critical review-responses addressed
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 15 total, all `addressed`.

Design review (pre-implementation) — 7:

| ID | Severity | Finding |
| -- | -------- | ------- |
| RR-YTC4W5 | critical | My claim that `ViewCell.Widget` was dead was FALSE — it is live and shipped on every table cell; my grep pattern could not match a field assignment |
| RR-2GBB0V | critical | My hint-arm rationale was false — unschema'd fields ARE editable (a missing verdict reads as writable); the real constraint is no PropertyDef to validate against |
| RR-Z0GGTO | significant | Don't accept table drift — the two existing resolvers already disagree on `file` |
| RR-NGY84F | significant | cards/list rows pass no `attachments`; `widget: file` would break there |
| RR-66MT0D | significant | The StatusControl warning was unbuildable (machine-ness is runtime); the interaction is two-axis |
| RR-693NL9 | minor | AC2's test was specced in Go, but the behaviour lives in `registry.ts` |
| RR-9G51IS | minor | Inheritance rationale weak; effort under-estimated (s → m) |

Code review (post-implementation) — 8:

| ID | Severity | Finding |
| -- | -------- | ------- |
| RR-DRIFT1 | critical | The Vitest drift-guard asserted its own copy of the registrations, not the registry — VERIFIED green through two mutations that should have failed it |
| RR-LISTW | significant | `widget:` on a `list: true` property was unvalidated → array flattened, scalar PATCHed over a list. Silent data corruption |
| RR-INLEN | significant | `widget:` on an inline enum ignored the value set → free-text over a constrained set |
| RR-CUSTY | significant | Custom-type equivalence was a name blacklist, not a `meta.Types` lookup → any undeclared type accepted as enum-like |
| RR-AMBIG | minor | No warning for a widget on a multi-type-traversal source — the one case with no other signal |
| RR-COLDUP | minor | `viewCollectionTypes` was a divergent hand-copy of ValidateConfig's map |
| RR-FILEORD | minor | `widget: file` check ran before the type check, hiding the better error |
| RR-JOINDUP | nit | `joinWidgetTableKeys` duplicated the existing generic `sortedMapKeys` |
| RR-E2EPATCH | nit | e2e autosave wait matched any PATCH under `/api/v1/` |

**Self-review of the diff.** Verified `git diff develop...HEAD` contains no
unrelated changes. Two prototype-project changes are deliberately committed
SEPARATELY so this ticket's commits stay single-purpose:

- `d2842498` — `ticket_summary.lua` used `rela.entity_id`, which the Lua runtime
  never binds, so every render of that document failed. Pre-existing on develop
  (byte-identical, last touched by an unrelated CalDAV commit) and surfaced only
  because the demo opened that page. Fixed rather than left broken.
- `3eecc88e` — the worked `widget:` example on the prototype ticket view.

Demo data mutated by the manual verification (`TKT-001.md`, `is_blocked` flipped
by a real click) was reverted — it is data, not a code change.

`registry.ts`'s diff is a pure restructure: the same ten registrations, moved
into exported data so the drift guard can assert them. No behaviour change.

## Acceptance Verification

- [x] Each acceptance criterion tested (reference planning checklist)
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
| -- | ------ | -------- |
| 1 — `widget: checkbox` renders a clickable checkbox that saves | PASS | e2e clicks it, asserts the PATCH lands and survives a reload. Also confirmed BY HAND in a browser: `is_blocked` flipped `false` → `true` on disk. |
| 2 — omitting `widget:` reproduces today's selection | PASS | Frontend table test over every property type asserting `widget: undefined` → `defaultWidgetFor(propertyDef)`, incl. the list-over-values-over-type precedence (RR-0Z1P6) |
| 3 — unregistered name is a config-load error naming the valid set | PASS | `TestValidateConfig_SectionFieldWidget` + `_ErrorListsValidNames` |
| 4 — a type mismatch is a config-load error | PASS | `_MismatchNamesAcceptedTypes`; extended post-review to `list` and value-set rules |
| 5 — a widget on an undeclared property warns, does not error | PASS | `TestCollectConfigWarnings_WidgetOnUndeclaredProperty` |
| 6 — `widget: file` outside a `properties` section errors | PASS | Table case using a genuine `file` property so it isolates the display-mode rule |

Beyond the ACs, pinned by test: the ACL conjunction is untouched (a widget
override cannot make a read-only field editable); the hint arm provably DROPS
the override; the StatusControl interaction is inert on `render: input` and live
on `render: display`; both Go construction sites carry `Widget`.

**Drift guard proven, not assumed.** Injected drift into the fixture → both Go
and Vitest fail. Injected drift into `registry.ts` (changed a widget's supported
types; added a new widget) → Vitest fails on both. The first version of the
guard passed both of those mutations, which is how RR-DRIFT1 was found.

**Manual verification.** Feature confirmed in a real browser by the user:
`description` renders as a textarea where its type default is a single-line
input — the case that can only come from config — while its neighbours keep
their type defaults. Checkbox click persisted to disk.

**Known cosmetic issue, filed not fixed:** `CheckboxWidget`'s edit arm carries
ZERO design tokens (every sibling widget has 6–13), so it renders as a bare
native control in a styled grid. Pre-existing and not a regression — this ticket
merely made a checkbox easy to place next to styled controls. Filed as
TKT-CBSTYLE (backlog, xs); out of scope here, which decides *which* widget
renders, not how it looks.

## Documentation (enhancements only)

- [x] Docs-checklist created and linked via `has-docs`
- [x] User-facing documentation updated
- [x] Docs-checklist marked as done

**Docs Checklist:** DOCS-WGTOVR

Note: this ticket also documents TKT-HOIX1's `render:` axis, which shipped
UNDOCUMENTED — its PR touched no docs file at all. `docs/` is generated from
`docs-project/entities/`, so an edit to the generated file is silently
overwritten by `just docs`. I hit the same trap once before catching it.

## Final Checks

- [x] Commit message explains the why, not just what
- [x] No TODOs or FIXMEs left unaddressed
- [x] Ready for another developer to use

Four commits, each single-purpose: the feature; the review fixes (whose message
records the data-corruption bug and the false drift-guard); the unrelated Lua
fix; the worked example.

Two debugging scaffolds written during implementation (a wire-conversion probe,
an e2e response probe) were removed — verified absent from the tree.

## Pull Request

- [ ] Run `/pr` command to create PR and monitor CI
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- filled by /pr -->
