---
id: IMPL-9AVT4WM
type: implementation-checklist
title: 'Implementation: Seed script on the fs backend hits the 30 s rela script timeout'
started: "2026-10-30"
status: done
---

<!-- @managed: claude-workflow v1 -->

## Development

- [x] Unit tests written for new code
- [x] ~~Integration tests written (test full flow, not just units)~~ (N/A: covered by the store-level and manager-level tests plus the seed run)
- [x] Happy path implemented
- [x] Edge cases from planning handled
- [x] Error handling in place (errors surfaced, not swallowed)

## Test Quality

- [x] ~~Using fixture builders or factories for test data~~ (N/A: small literal entities)
- [x] No hardcoded values in assertions when object is in scope
- [x] Only specifying values that matter for the test
- [x] ~~Interpolated values constructed from objects, not hardcoded~~ (N/A: no interpolation)
- [x] ~~Property comparisons use original object, not hardcoded strings~~ (N/A: no property assertions)

## Manual Verification

- [x] Feature manually tested end-to-end
- [x] Each acceptance criterion verified with test scenario from planning
- [x] Edge cases manually verified

**Verification Evidence:**
- `TestCreate_GeneratesIDFromIDListing` failed on develop (0 id listings, 2
  full scans) and passes with the fix.
- `TestEntityIDs_MatchesAllFacesScan`: the listing equals an all-faces scan
  with faces, after reopening the store and after a family delete.
- `TestFSStoreTxViewIsEntityIDLister`: the real fsstore and its Tx view
  satisfy the interface.
- entitymanager, fsstore, search/..., appbuild, storetest tests pass.
  golangci-lint 0 issues, commentlint gates clean, `just arch-lint` OK.
- Seed of the GRC demo copy (fs backend), 30 min timeout build: develop
  user+sys 9-10 s, fix 5.8-6.3 s, wall 49-82 s vs 41-52 s. With the
  normal 30 s timeout the fix gets to about 295 of 341 entities on this
  loaded machine (develop about 220). The rest is tracked in BUG-UR0520H.

## Quality

- [x] Code follows project patterns (check similar code)
- [x] Checked for DRY opportunities — repeated literals, expressions, or
patterns extracted to a helper / constant / type where it sharpens the contract
(don't extract for its own sake; CLAUDE.md "three similar lines is better than a
premature abstraction" still holds)
- [x] No security issues introduced
- [x] No silent failures (errors logged AND returned)
- [x] No debug code left behind
