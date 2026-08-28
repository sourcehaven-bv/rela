---
id: IMPL-826Q8J
type: implementation-checklist
title: 'Implementation: Extract dataentry theme/settings/palette cluster to appearanceHandler (App 104 → 92)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new behavior — receiver-only moves; existing HTTP-level handler tests drive every moved route)
- [x] ~~Integration tests written~~ (N/A: same)
- [x] Happy path implemented
- [x] Edge cases from planning handled (wiring parity: newAppearanceHandler(app) constructed in both NewApp and rebindApp; schema/services as closures so live reload propagates; palette reload is in-place Reresolve under the service mutex, so the by-value handle stays correct)
- [x] Error handling in place

## Test Quality

- [x] ~~Fixture builders~~ (N/A: 22 changed test lines are mechanical re-points app.handleX → app.appearance.handleX)
- [x] ~~No hardcoded values in assertions~~ (N/A)
- [x] ~~Only values that matter~~ (N/A)
- [x] ~~Interpolated values from objects~~ (N/A)
- [x] ~~Property comparisons from original object~~ (N/A)

## Manual Verification

- [x] Feature manually tested end-to-end (full suite + dataentry package green)
- [x] Each acceptance criterion verified
- [x] Edge cases manually verified

**Verification Evidence (PR #1464, commit 3c4d129b):**
- App 104 → 92 (92 production methods verified; directive ratcheted).
- appearanceHandler: 12 methods, no directive needed, no store.Store field.
- The one non-mechanical line (settings_handlers.go palette save) is a
literal inline of the Cfg() one-line accessor — same single atomic load.
- All 5 routes re-pointed in api_v1.go; router.go correctly untouched.
- Gates: build, tests, -race dataentry, plimsoll, arch-lint, comment-lint,
golangci-lint all green; CI green except the Rela Tickets gate (resolved by this
ticket's files landing on the branch).

## Quality

- [x] Code follows project patterns (viewsHandler shape; improved constructor: newAppearanceHandler(app) reads fields in one place so NewApp/rebindApp can't drift)
- [x] Checked for DRY opportunities
- [x] No security issues introduced (relation-default lookup keeps the identical Services/viewReader path — DEC-ZBI39P row-gate + field redaction stay on the path; ACL settings tests pass)
- [x] No silent failures
- [x] No debug code left behind
