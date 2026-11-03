---
id: IMPL-P9FU0Q
type: implementation-checklist
title: 'Implementation: Preset buttons that open the composer prefilled'
started: "2026-10-30"
completed: "2026-10-31"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestButtons_Compose`, `TestButtons_ComposeProblems`, `TestButtons_ComposeGuards`, `TestButtons_ComposeOffers`, `TestButtonOptions_Compose`, `TestSessionView_ButtonCompose`, `TestPost_ComposePreset`, composer stories (`ComposeMode`, `ComposeModeFocus`, `ComposeModeTypeSwitch`, `ComposeModeDuringEdit`, `ComposeModeEditEnds`), `decisionViews presets`, `useSessionDecisions compose presets`
- [x] Integration tests written (test full flow, not just units): `TestButtons_DeciderAgreesCompose`, `TestWireChannelSession_Button` compose case, wire and MCP goldens, OpenAPI assertion, `ChannelsPanel preset buttons` compose cases, E2E "a preset that needs text opens the composer"
- [x] Happy path implemented: a `compose: true` preset opens the composer with its values locked in a strip; Send adds the preset values and the session generation read at the click
- [x] Edge cases from planning handled: left-out required list refused; guards reading a left-out prop or the whole message refused, also for earlier guards admitting a clicking role; `confirm` warned and dropped; discard of an open edit keeps compose mode; preset merged only when the composer reports the same compose key; stale generation refused with `session_moved`
- [x] Error handling in place (errors surfaced, not swallowed): load errors name the type, button and property

## Test Quality

- [x] Using fixture builders or factories for test data: realtime config fixtures (`composeRealtime`), session fixtures, E2E realtime project
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: covered by the E2E compose test against a real server)
- [x] Each acceptance criterion verified with test scenario from planning: AC table in the notes maps 1 to 14 to tests
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build on four tags; vet; `just lint` (0 issues on all
tag sets), arch-lint, plimsoll, comment-lint; `go test -race` on touched
packages; 5469 app tests and 647 rela-components tests, type checks; `just
build-frontend`; `channels.spec.ts` 11 of 11; `just docs`; markdownlint;
archplan ci; coverage check.

**Deviations:** the wire compose case is in `channels_handler_test.go`; decider
and offers cases use their own fixture; a guard reading the whole `message` is
handled but has no test because no compiling predicate reads it; `effect` was
split for gocognit.

## Quality

- [x] Code follows project patterns (check similar code): strict boolean via `internal/yamlscalar` like `confirm` and `unread`; preset values go through `NormalizeProps` like one-click presets; composer mode mirrors the existing edit mode
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): `composeOf` and `unreadOf` share one flag
reader
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
