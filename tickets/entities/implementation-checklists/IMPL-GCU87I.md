---
id: IMPL-GCU87I
type: implementation-checklist
title: 'Implementation: Dashboard cards: list source, server-side values, matrix and ratio'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (dashboard_cards_validate_test.go: 17 load-time cases plus the columns-by-shape decode; dashboard_card_test.go: fold, axis cap and color-resolution units; DashboardMatrix.test.ts; new DashboardView list-card cases)
- [x] Integration tests written (dashboard_card_test.go drives GET /api/v1/_dashboard/card through the real handler for every display, ACL row gating, a hidden value, the permission filter, ratio with a real query scope, and the size-independent read budget)
- [x] Happy path implemented
- [x] Edge cases from planning handled (unset axis goes to unplaced; integer axis capped at 50 with truncated; hidden group or tone value sets partial; bad or stale card index answers 404; query card answers 400; ratio without a scope resolver errors instead of matching everything)
- [x] Error handling in place (list pipeline errors go through writeListPipelineError; a stale value is dropped on the client)

## Test Quality

- [x] Using fixture builders or factories for test data (seedStatusTickets, seedGatedTickets, installDashboardCards, card()/value() builders in the SPA tests)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Demo project at /tmp/afae-demo/project (5 tickets, 3 with estimated_hours, query
scope `started`), served by a fresh build on port 8794.

- `/api/v1/_dashboard/card?card=0..3` returned: count 5; a priority-by-status matrix with tone colors (badge-red, kit-orange-500, badge-orange, badge-green); an hours-by-status integer matrix with columns 3 and 8 and `unplaced: 2`; a ratio of 3/5 with per-priority groups.
- Screenshots in light and dark mode show the count, matrix, ratio and server breakdown tiles next to the existing query cards. Shaded cells use the accent; toned cells use the palette colors with readable text in both themes.
- Keyboard: a matrix cell takes focus, its aria-label reads "high, open: 1", and activating it opens `/list/all_tickets?filter[priority]=high&filter[status]=open`, which shows the one matching ticket with both filters applied.
- The card link of a list card opens `/list/all_tickets`.
- `rela validate` accepts the new cards; the load-time checks are covered by the unit tests.

## Quality

- [x] Code follows project patterns (check similar code) (reuses navStatusRows, listPage, viewQueryScope, applyScope, visibility.Redact; handler sits next to handleV1Dashboard so the permission filter stays in its allowed file)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (groupCounts serves breakdown and ratio;
styleFill reuses styleSwatch)
- [x] No security issues introduced (rows gated and redacted before folding; no cross-principal cache; Cache-Control no-store; permission filter unchanged)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
