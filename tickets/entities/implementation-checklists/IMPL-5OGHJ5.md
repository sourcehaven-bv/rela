---
id: IMPL-5OGHJ5
type: implementation-checklist
title: 'Implementation: Piles: personal working sets of entities (service, API, scope source, UI)'
started: "2026-10-08"
completed: "2026-10-09"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (piles service tests; pilestest conformance on kvpiles and pgpiles; lua, mcp, automation, autocascade, metamodel and dataentryconfig tests; mcpwire adapter test; Vitest for usePiles, PilePanel, NewPileDialog, EntityList, Sidebar, scope navigation)
- [x] Integration tests written (appbuild: TestPiles_CascadeWiring, TestPiles_AssigneeLifecycle, TestPiles_PushOwnerCheckUsesCallerGate, durability and re-assembly tests; dataentry handler and storetest.Counting budget tests; e2e/tests/piles.spec.ts)
- [x] Happy path implemented
- [x] Edge cases from planning handled (eviction at 500, 50-pile limit, case-insensitive names, faces, ambiguous bare ids, renames and deletes, hidden items, foreign pushes)
- [x] Error handling in place (store faults give 500; automation pushes are logged and never fail the write, by design)

## Test Quality

- [x] Using fixture builders or factories for test data (newService, writeMetamodelBody, appbuildOnDisk, e2e api fixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

- Live rela-server on a scratch project as user alice: `/_sidebar` piles_available true;
create with 3 items 201; duplicate add `{added:0}`; unknown item 404
item_not_found; duplicate name (other case) 409; unknown icon 400; pile
`_position` gives current 2 of 3 with prev/next addresses; `_remove` of an
unknown id 204; remove then GET count 2; unregistered export transform 404;
unknown and malformed pile ids give the same 404; piles persist to
`.rela/piles.json`.
- e2e `piles.spec.ts`: list selection to new pile, sidebar count, flyout panel, step through
`[1/3]` to `[2/3]`, remove with Undo. 1/1 passed, then 5/5 with
`--repeat-each=5`.
- `go test -race ./...`: all pass (dataentry and sqlitestore re-run alone after load timeouts).
- `just coverage-check`, `just arch-lint`, `just plimsoll`, `just comment-lint`, `just lint-md`,
golangci-lint on touched packages: clean. Frontend typecheck clean, lint 0
errors, `npm run test:run` 3818/3818. `just docs` regenerates the edited docs
byte-identically.

## Quality

- [x] Code follows project patterns (check similar code) (comments-style service outside the graph; consumer-side seams; wiring adapters in appbuild and mcpwire)
- [x] Checked for DRY opportunities (one MCP adapter shared by stdio and remote servers; Pile.Refs replaces three copies)
- [x] No security issues introduced (owner-only reads, read-gated owner check, write-only foreign pushes, person-mapping guard on owner moves)
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
