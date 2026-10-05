---
id: IMPL-H94W87
type: implementation-checklist
title: 'Implementation: Generated faces and worlds: implicit face, generated default world only without declared worlds, faced write grants must name a face'
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

- [x] ~~Feature manually tested end-to-end~~ (N/A: verified by HTTP-level and e2e tests instead of a manual pass)
- [x] ~~Each acceptance criterion verified with test scenario from planning~~ (N/A: verified per stage by the automated tests listed under Verification Evidence, not one by one against PLAN-68ZQAR)
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are covered by the table-driven tests)

**Verification Evidence:** Every stage landed on `faces-intrinsic` with its own
tests. Write targets: `TestWriteTarget` (cli, visibility, dataentry),
`TestUpdateEntity_FacedBareIDNamesReadableFaces` (mcp),
`TestUpdateEntity_FacedBareIDNamesItsFaces` (lua). Collection create per face:
`TestFaceGrant_CollectionCreateIsPerFace`. SPA: `WorldSwitcher.test.ts`,
`unknownWorldQuery.test.ts`, `useCreateFace.test.ts`. E2E:
`faces-worlds.spec.ts` (switcher, stale `?world=default`, create face picker)
and the existing faces specs; full suite 358 passed, with three load flakes that
pass on rerun and the kanban race fixed. `go test ./...` passes on the default,
postgres and sqlite builds; arch-lint, comment-lint and golangci-lint are clean.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
