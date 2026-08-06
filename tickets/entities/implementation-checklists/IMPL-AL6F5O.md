---
id: IMPL-AL6F5O
type: implementation-checklist
title: 'Implementation: Case-variant entity IDs collide in fsstore but coexist in memstore/pgstore'
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

- [x] ~~Using fixture builders or factories for test data~~ (N/A: entity.New + literal IDs; no object graph to build)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] Interpolated values constructed from objects, not hardcoded
- [x] Property comparisons use original object, not hardcoded strings

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**

Reproduced BEFORE the fix, against a real case-insensitive filesystem
(`OsFS` in a `t.TempDir()` on macOS — `MemFS` is case-sensitive and does NOT
show it):

    fsstore   CreateEntity("ABC") after "abc" -> err=nil ; GetEntity("abc") = "OVERWRITER UPPER"
    memstore  CreateEntity("ABC") after "abc" -> err=nil ; GetEntity("abc") = "LOWER"

Two backends, same calls, different outcomes — fsstore loses an entity, the
byte-exact backends keep both. That divergence is the bug.

Also verified the same divergence pre-existed on origin/develop (validators
reverted), confirming it is not a regression from PR #1272.

AFTER the fix, all three new conformance cases pass in every backend:

- fsstore  — `go test ./internal/store/fsstore/` green
- memstore — `go test ./internal/store/memstore/` green
- pgstore  — run against a REAL PostgreSQL 15 instance
  (`RELA_TEST_DATABASE_URL` set, migration 0007 applied on a fresh DB);
  `CreateRejectsCaseVariantID`, `RenameRejectsCaseVariantID`, and
  `RenameToOwnCaseVariantIsAllowed` all confirmed RUN (not skipped) and PASS.

Full verification:

- `go test ./...` (default build) — pass
- `go test -tags postgres ./internal/store/...` against real PG — pass
- `FuzzRenameKeyCollapse` 20s (fsstore), `FuzzRelationKeyCollision` 20s
  (memstore) — pass
- `golangci-lint run ./internal/store/...` — 0 issues
- `just arch-lint` — OK; `just coverage-check` — pass (76.8%)

Two pre-existing pgstore tests needed updating, both legitimately:

1. `TestListOrderIsByteWise` used the pair "a-2"/"A-2" to prove byte-wise
   ordering — a case-variant pair that is now unrepresentable. Changed to
   "a-9", which preserves the mixed-case ordering property the test exists to
   check (lowercase still sorts last under byte order, not under en_US) while
   using a legal fixture.
2. `TestStatusFreshSchema` pins the embedded migration count; bumped 6 -> 7.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
