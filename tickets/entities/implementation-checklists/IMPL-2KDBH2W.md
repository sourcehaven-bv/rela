---
id: IMPL-2KDBH2W
type: implementation-checklist
title: 'Implementation: Reduce Go and TypeScript code size'
started: "2026-11-27"
completed: "2026-11-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: no new code; existing tests merged into tables)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: no new code)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: error paths unchanged)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: existing fixtures kept)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: assertions kept as they were)
- [x] ~~Only specifying values that matter for the test~~ (N/A: assertions kept as they were)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: assertions kept as they were)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: assertions kept as they were)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: refactor with no feature)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: covered by the suites)

**Verification Evidence:**

- Go: `go build` and `go vet` clean for none, memorybackend, postgres, sqlite
  and headless; `go test ./...` passes for default, memorybackend and sqlite;
  `just lint`, `just arch-lint`, `just comment-lint`, `just plimsoll`,
  `just comment-body` clean.
- Frontend: `npm run typecheck`, `npm run lint`, `npm run test:run` (7103
  tests), `npm run build`, `npm run lint:comments` pass; `rela-components`
  `npm run check` passes.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
      patterns extracted to a helper / constant / type where it
      sharpens the contract (don't extract for its own sake; CLAUDE.md
      "three similar lines is better than a premature abstraction"
      still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
