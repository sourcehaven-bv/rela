---
id: IMPL-1IE0Q0
type: implementation-checklist
title: 'Implementation: Message options rendered as reply buttons'
started: "2026-10-29"
completed: "2026-10-29"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: `TestValidateProps_StringList`, `internal/texthazard` tests, `TestParse_ListProperty`, `TestChoices_Load`, `TestChoices_Problems`, `TestChoices_ButtonWarning`, `TestChoices_SessionWarning`, `TestChoicesReachable`, `TestWhenHolds_List`, `TestChannelPaths_ListProperty`; SPA `channelView.test.ts` choices table, `channelValues` list carry, `useSessionDecisions.test.ts` choice suite
- [x] Integration tests written (test full flow, not just units): `TestPost_ListProp`, `TestChoices_PoCFlow` (`session_moved` and `session_wrong_state`), `TestChoices_SecondAnswerWithoutSession`, `TestRead_Choices`, `TestPostMessage_ListProp`, `TestChannelCLI_ListProp`, `TestDocsExample_Choices`; ChannelsPanel choice wiring; e2e `channels.spec.ts` "a click on an option posts the answer"
- [x] Happy path implemented: `list: true` string message properties; `choices: {from, answer, into, other}` on a message type; options render as buttons on a top-level question; a click posts the answer type as a reply; Other opens the composer; MCP and CLI post lists
- [x] Edge cases from planning handled: empty, duplicate, long, unicode and hazard items refused; NFC duplicates; stale question (only the decisionCause question at the head generation, none while `hasNewer`); answered question hides the buttons; removed answer brings them back; no post right shows a list; string to list config change on edit
- [x] Error handling in place (errors surfaced, not swallowed): load problems keyed by config path; `session_moved`, `session_wrong_state` and `invalid_message` mapped in the SPA

## Test Quality

- [x] Using fixture builders or factories for test data: realtime harness and config fixtures, library story fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end: e2e `channels.spec.ts` 10 of 10 locally
- [x] Each acceptance criterion verified with test scenario from planning: see PLAN-3C3RLP
- [x] ~~Edge cases manually verified~~ (N/A: covered by tests)

**Verification Evidence:** build and vet on all four tags; golangci-lint 0
issues; arch-lint, plimsoll, comment-lint clean; `go test -race` on the touched
packages; channel conformance with sqlite and postgres tags; frontend typecheck,
eslint, vitest (5248 tests); library vitest (620 tests), `vue-tsc -b` and
Storybook build; markdownlint; `just docs`.

**Deviations:** no composer entry of list values (out of scope; the e2e test
posts questions over the API); `TestChoices_PoCFlow` covers both the PoC config
(`session_wrong_state`) and an answer type without `in:` (`session_moved`);
`openapi.Schema` limits moved into an embedded `Limits` and choice checks into
`choiceChecker` to stay under plimsoll; paging moved to `useChannelPaging.ts`;
U+FE0F allowed and folded before the duplicate check; U+200D allowed only
between symbols.

## Quality

- [x] Code follows project patterns (check similar code): `list: true` as in schema.yaml; choice buttons reuse `RlChannelDecisions` and the `useSessionDecisions` click lock; `expect_generation` as in TKT-T9G1AE
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): one hazard check
(`texthazard.IsDisplayHazard`); one list normalizer used by every write path
- [x] No security issues introduced: rela-security-reviewer found none
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
