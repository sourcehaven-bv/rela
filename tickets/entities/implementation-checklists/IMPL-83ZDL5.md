---
id: IMPL-83ZDL5
type: implementation-checklist
title: 'Implementation: Store API takes entity.Ref; zero-value EntityQuery is invalid'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] Integration tests written (test full flow, not just units)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

Built in #1725, #1726, #1728, #1729, #1730, #1732, #1734, #1735, #1733.
storetest gained `RunAddressTests`, `RunFamilyTests`, `RunFaceSelectionTests`,
attachment and relation-key cases, run on fs, mem, pg and sqlite. Integration:
BUG-J3PBFN and BUG-95W7MV regression tests through CLI, MCP, Lua and data-entry
on faced fixtures.

## Test Quality

- [x] Using fixture builders or factories for test data
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] ~~Feature manually tested end-to-end~~ (N/A: internal store API refactor; user-visible effects are covered by the CLI, MCP and data-entry regression tests and the E2E job)
- [x] Each acceptance criterion verified with test scenario from planning
- [x] ~~Edge cases manually verified~~ (N/A: edge cases are pinned by storetest on all four backends rather than checked by hand)

**Verification Evidence:**

- AC1: storetest green on fs and mem (Test job), pg (Postgres Backend job)
and sqlite (SQLite Backend job) on every PR.
- AC2: `RunFaceSelectionTests`; zero `FaceSelection` is `ErrInvalidQuery`.
- AC3: `RunAddressTests` (zero Ref, `X@draft`, malformed faces).
- AC4: `RunAttachmentTests` family-existence cases.
- AC5: storetest relation cases for two tails on one triple (#1733).
- AC6: `RunVersionTests` per-face history, `VersionMeta.Face`.
- AC7: `pgstore/face_read_explain_test.go` and the sqlite EXPLAIN QUERY PLAN
test (#1725).
- AC8, AC9: BUG-J3PBFN (#1730) and BUG-95W7MV (#1729) regression tests.
- AC10: `internal/archguard` bareref, tailless, faceselect and directread
guards; `zeroface_test.go` deleted.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind

`rela-security-reviewer` ran on PR 8 (#1734): no critical or significant
findings. Family reads were consolidated into `store.FamilyHeaders` (RR-L7NGRP).
