---
id: IMPL-RCZIY9
type: implementation-checklist
title: 'Implementation: Schema property labels render on generic entity details'
started: "2026-10-02"
completed: "2026-10-02"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Go API test `TestV1SchemaWithCustomTypes` asserts the property label is returned
by `/_schema`. Frontend `EntityDetail.world.test.ts` asserts a generic detail
property uses its schema label and an unlabeled property keeps its field label.
No new error path was introduced.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

Existing app fixture and `entry` / `section` builders are used. Assertion
strings intentionally pin the external label and fallback contract.

## Manual Verification

- [x] Feature manually tested end-to-end — N/A: verified through the frontend component and v1 schema API tests; no separate browser session used
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified through tests

**Verification Evidence:**
- `go test ./internal/dataentry -run '^TestV1SchemaWithCustomTypes$' -count=1` — passed.
- `npm test -- --run src/components/entity/EntityDetail.world.test.ts` — 75 tests passed.
- `npm run typecheck` — passed.
- `npx eslint src/components/entity/EntityDetail.vue src/components/entity/EntityDetail.world.test.ts src/types/schema.ts` — 0 errors; 8 pre-existing warnings (large file, existing v-html/template warnings, existing test type assertions).
- `just docs` — passed; regenerated docs. It reported 11 existing orphan guides during its advisory validation.
- `git diff --check` — passed.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or patterns extracted to a helper / constant / type where it sharpens the contract (don't extract for its own sake; CLAUDE.md "three similar lines is better than a premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

The API uses the existing shared property serializer, and the display mapping
uses its already-resolved property definition. The change is display-only and
introduces no I/O or new error handling.
