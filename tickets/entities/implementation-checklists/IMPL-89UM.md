---
id: IMPL-89UM
type: implementation-checklist
title: 'Implementation: Markdown checkboxes in entity content are no longer clickable'
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: fix touches existing call sites; no new fixtures needed)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation in new test assertions)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: tests compare HTML output, not domain objects)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Reproduced the bug on `develop`: `node -e` showed marked v17 emits `<input disabled="" type="checkbox">` (attribute order changed), confirming the regex in `markdown.ts:62` never matches.
- Standalone curl test against `rela-server` confirmed the multipart-body server-side failure: `curl -F` returned HTTP 400 "Invalid checkbox index"; `-d` (urlencoded) returned 200 and updated the file.
- Full e2e suite: 198/198 passed including the un-skipped + strengthened `checkboxes.spec.ts` "clicking a checkbox persists the toggle on the server" test which now asserts BOTH the server-side file content AND the SPA's rendered checkbox state.
- Frontend unit tests: 787/787 passed, including new assertions that `data-cb-idx` is present (interactive renders), `disabled` is present (non-interactive renders), and the counter resets per render with an exact-count assertion.
- Edge case: rapid double-click on the same checkbox now suppressed by `togglingIndices` set (no test in e2e — covered by the in-flight-guard design; would need timing-sensitive integration test for direct coverage).

## Quality

- [x] Code follows project patterns (check similar code)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
