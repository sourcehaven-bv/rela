---
id: IMPL-F2OZVE
type: implementation-checklist
title: 'Implementation: Realtime 3/7: SPA channels'
started: "2026-10-20"
completed: "2026-10-20"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code: Go tests for `Describe`, per-message `_actions`, the channel schema endpoint, the `channels:` config checks and the wire golden (`internal/realtime`, `internal/realtimecfg`, `internal/dataentry`, `internal/dataentryconfig`, `internal/openapi`); vitest for the API client, `useChannel`, `useEntityChannels`, the channel schema store, `renderMessageMarkdown`, `channelValues`, `channelErrors` and `ChannelsPanel`; Storybook play tests for the rela-components channel components
- [x] Integration tests written (test full flow, not just units): `e2e/tests/channels.spec.ts` runs rela-server with acl.yaml and realtime.yaml and covers live posts between users, a user without the post role, a live close, revoked read access and a faced type
- [x] Happy path implemented: T7a config, endpoint and data layer (PR #1937, merged), T7b library components (PR #1938, merged), T7c panel on the entity page (T7c PR)
- [x] Edge cases from planning handled: restricted refs show a placeholder and never an id, edits keep hidden and unreadable props, removed and pending messages, ephemeral expiry, closed and archived channels, refused versus transient refetch, the 500-message cap, a non-default world on channel and presence routes, entity-to-entity navigation
- [x] Error handling in place (errors surfaced, not swallowed): each server error type maps to a field error, a refetch, a schema reload or a toast, and a test checks every type in `internal/realtime/errors.go` has one rule

## Test Quality

- [x] Using fixture builders or factories for test data: the channel wire golden samples, the realtime e2e project helper and the story fixtures
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: the e2e spec drives the real server and browser for every user-facing flow; the components were checked by screenshot in Storybook)
- [x] Each acceptance criterion verified with test scenario from planning: each of the 23 acceptance criteria in plan section 15 names its test
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by the unit, story and e2e tests listed above)

**Verification Evidence:** Each slice ran build and vet on the default,
`postgres` and `sqlite` tags, `go test -race` on the touched packages, `just
lint`, `just arch-lint`, `just comment-lint`, `just plimsoll` and `just docs`;
the frontend typecheck, lint, unit tests and build; the rela-components checks
(purity included), story tests and library build; and the channels and presence
e2e specs. PRs #1937 and #1938 passed all 29 CI checks.

## Quality

- [x] Code follows project patterns (check similar code): the channel schema comes from a realtime service method through a consumer-side interface; the `channels:` check uses a narrow interface wired in appbuild; library components stay pure (purity check) and the app adapts wire types to them
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds): the edit, delete, remove and release
affordances share their check functions with the operations; the reserved
operation names are defined once
- [x] No security issues introduced: rela-security-reviewer ran on all three slices; no critical or significant findings, and all minor ones are addressed
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
