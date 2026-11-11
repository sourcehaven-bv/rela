---
id: IMPL-7P426QG
type: implementation-checklist
title: 'Implementation: Hide message types from the web app composer'
started: "2026-11-10"
completed: "2026-11-11"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- End to end: Playwright spec "message types people do not post"
(e2e/tests/channels.spec.ts) against a built rela-server with the quiet room and
`offer: false` on progress and step. Step has no decision button, Plan keeps
one, Progress/Step are not in the composer; posting progress and step over the
HTTP API returns 201, the messages show for both readers, and step moves the
session. Passes; with the OpenChannel wiring removed it fails on the Step
decision button (checked). The whole channels spec passes (17 tests).
- AC1/AC6/AC7, button fallback, select() of an unoffered type: Storybook
play stories OfferFalse* in RlChannelComposer.stories.ts and
RlChannel.stories.ts. Mutating the filters makes 4 + 1 of them fail.
- AC4/AC5: OfferFalseCompose and OfferFalseEdit stories.
- Decision button: decisionViews tests (type button dropped, preset kept).
- AC8/AC9: dataentryconfig table tests (unknown key, `"no"`, `off`, null,
on a field, docs example) and TestValidateChannels_OfferFalseWithPreset;
appbuild TestRealtimeChannelSchema_PresetButtons.
- Suites: frontend 6011 unit tests pass; story suites pass; vue-tsc clean;
go test dataentryconfig, appbuild, dataentry pass; go vet, arch-lint,
comment-lint gate clean. golangci-lint could not load packages locally (go list
timed out while other sessions ran race suites); left to CI.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
