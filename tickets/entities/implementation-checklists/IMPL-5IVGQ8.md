---
id: IMPL-5IVGQ8
type: implementation-checklist
title: 'Implementation: Lua rela.tagged_entities: find entities by version tag'
started: "2026-10-27"
completed: "2026-10-27"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code (visibility `taggedentities_test.go` gate table, budget test; lua `taggedentities_test.go` binding shape, options, errors)
- [x] Integration tests written (test full flow, not just units) (`storetest` TaggedEntities conformance on sqlite and postgres; `appbuild/luataggedentities_db_test.go` end to end with acl.yaml on both database builds)
- [x] Happy path implemented
- [x] Edge cases from planning handled (rename follow, delete, delete then recreate, rename onto a deleted id, id reuse renamed away, per face, soft delete, type filter, unknown tag, zero name)
- [x] Error handling in place (errors surfaced, not swallowed) (store errors returned; Lua raises ErrHistoryUnsupported verbatim and other errors as a generic error after logging)

## Test Quality

- [x] Using fixture builders or factories for test data (tagHarness helpers, tagRow, taggedFixture)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

sqlite build CLI (`rela-sqlite script`), project with two maatregelen:
freeze.lua tagged M-001 and M-002 (`tag M-001 v1`, `tag M-002 v1`); M-001
updated to status=done; M-002 deleted; compare.lua printed `M-001 maatregel v1
deleted=false draft -> done`, `M-002 maatregel v1 deleted=true`, `other type:
0`. Default fs build: compare.lua raised `rela.tagged_entities: version history
is not supported on this storage backend`. ACL gating (history:read, unreadable
type) verified by TestLuaTaggedEntities on sqlite and postgres.

## Quality

- [x] Code follows project patterns (check similar code) (gate mirrors resolveHistorySubject; batched header read like ResolveHeaders)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds) (one SQL text shared by both backends in
storeutil)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
