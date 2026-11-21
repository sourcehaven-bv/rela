---
id: IMPL-LBN0BF7
type: implementation-checklist
title: 'Implementation: Entity page refresh after actions, step scroll, sidebar flash, run actor, unknown routes and 422 log'
started: "2026-11-21"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (keepInView.test.ts, notFound.test.ts, DynamicForm.refusal.test.ts; new cases in EntityDetail.actions, ChangesPanel, MentionedByPanel, FlowRunsPanel, focusRun, Sidebar.builtins and flowView tests)
- [x] Integration tests written (test full flow, not just units) (EntityDetail mounts the real panels; the refusal test sends the create through the real API client; the router tests push through the real guards; flow-task-hint.spec.ts checks the step form in a browser)
- [x] Happy path implemented
- [x] Edge cases from planning handled (a failed action does not bump; a failed or cancelled sidebar load shows the built-in rows; a step form that renders later takes over the scroll; /s/<space>/login keeps its space; a 422 without field errors logs nothing; a 5xx still logs)
- [x] Error handling in place (errors surfaced, not swallowed) (unexpected failures are still logged; refusals are shown to the user)

## Test Quality

- [x] Using fixture builders or factories for test data (the existing mount helpers and view responses of each test file)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: not checked in a browser; the built-in browser had no free tab)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified (each fix reverted once to see its test fail)

**Verification Evidence:**
Reverting each source fix makes its new test fail. Full vitest, vue-tsc and eslint (no errors) pass. The browser's own "Failed to load resource" line for a 422 is a network log the app cannot suppress. The signup bot has no person entity, so it reads "System"; no configuration can name it.

## Quality

- [x] Code follows project patterns (check similar code) (keepInView now also serves the router's anchor scroll; actorLabel is the BUG-9WCZYOX helper reused)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced (frontend only; the not-found page renders static text)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
