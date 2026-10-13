---
id: IMPL-TCW61K
type: implementation-checklist
title: 'Implementation: Realtime 2/2b: SPA presence avatars and claim badge'
started: "2026-10-12"
completed: "2026-10-12"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (vitest for usePresence, PresenceAvatars, LeaseBadge, EntityDetail presence target, EntityList and kanban badges, kanban live path)
- [x] Integration tests written (test full flow, not just units) (e2e/tests/presence.spec.ts with two browser contexts)
- [x] Happy path implemented
- [x] Edge cases from planning handled (refused refetch clears; faced types subscribe with a face; unknown type not ready; caps with more)
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] Using fixture builders or factories for test data (presence page object, LeaseClient)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end (Playwright drives two real browser contexts against rela-server)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (presence spec 10 of 10 after the live baseline fix)

**Verification Evidence:** AC1: e2e bob appears for alice; leave timed from
context close within 25 s. AC2: e2e own avatar count 0; vitest 20 viewers plus
more 3 render 5 and +18. AC3: usePresence vitest clears on 404. AC4: e2e claim
shows badge on detail, list and kanban; release removes it live on each. AC5:
vitest and e2e render markup in status literally; no v-html. AC6:
KanbanView.live.test with world and filter matches the page-1 list path. AC7:
EntityDetail vitest: faced type subscribes with its face; none while the type is
unknown.

## Quality

- [x] Code follows project patterns (check similar code) (useLiveSubscription, shouldDropHeldContent, WorldBadge placement)
- [x] Checked for DRY opportunities (viewerTitle shared by title and aria description)
- [x] No security issues introduced (security review: none)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
