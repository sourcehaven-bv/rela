---
id: IMPL-YMN3L0D
type: implementation-checklist
title: 'Implementation: ACL guide: adopted row on a push remote is deleted when gone'
started: "2026-11-14"
completed: "2026-11-14"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] ~~Unit tests written for new code~~ (N/A: docs only; behaviour already pinned by TestPush_PushedRowIsDeletedWhenGone)
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: docs only)
- [x] Happy path implemented
- [x] ~~Edge cases from planning handled~~ (N/A: docs only)
- [x] ~~Error handling in place (errors surfaced, not swallowed)~~ (N/A: docs only)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: no tests changed)
- [x] ~~No hardcoded values in assertions when object is in scope~~ (N/A: no tests changed)
- [x] ~~Only specifying values that matter for the test~~ (N/A: no tests changed)
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no tests changed)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no tests changed)

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: docs only; text checked against code instead)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: docs only)

**Verification Evidence:** Read `goneEntity` and `goneRelation` in
internal/remotesync/gone.go: the `row.Adopted` hold applies only when
`!r.m.Remote.Push`, and on a push remote only an edited row reaches
`keepEdited`. The new sentence matches. `just docs` regenerated
docs/acl-security.md with only this change.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] ~~Checked for DRY opportunities~~ (N/A: docs only)
- [x] No security issues introduced
- [x] ~~No silent failures (errors logged AND returned)~~ (N/A: docs only)
- [x] No debug code left behind
