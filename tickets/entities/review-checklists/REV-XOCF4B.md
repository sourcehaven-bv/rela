---
id: REV-XOCF4B
type: review-checklist
title: 'Review: Relation write bypasses ACL (incl. --read-only) when peer entity does not exist'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (CI green: Test, Postgres Backend, Fuzz, E2E)
- [x] Lint clean (CI green: Lint, Architecture, God-object lint; golangci-lint 0 issues)
- [x] Coverage maintained (entitymanager 87.2%, dataentry 80.7% — above floors)

## Code Review

- [x] Ran cranky-code-reviewer on PR #1115
- [x] ~~All critical review-responses addressed~~ (N/A: no critical findings)
- [x] ~~All significant review-responses addressed~~ (N/A: no significant findings)
- [x] Self-reviewed the diff for unrelated changes (fix + tests only)

**Review Responses:** RR-4F3ETV, RR-TQAH7F, RR-TCBQTQ — all minor, deferred to
follow-up polish (stale warning text, string-matching fragility, cross-route
status inconsistency). None blocking.

## Acceptance Verification

- [x] Each acceptance criterion tested: authz runs before peer-existence in Create/Update/DeleteRelation; dangling-peer allowed write → 422; dangling-peer denied write → 403; existing-peer denied → 403; no ungated store write; exactly one audit record
- [x] Test evidence: TestReadOnlyACL_DanglingPeerRelationWrite_Refused, TestDanglingPeerRelationWrite_AllowedACL_422, TestManager_RelationWrite_AuthorizesBeforePeerExistence, TestReadOnlyACL_EveryWriteRoute_DeniesAndDoesNotMutate (P4). demo_a flips to NOT-REPRODUCED

**Acceptance Status:** PASS — reviewer verdict "correct, hole closed, ship it",
no security false-negative. Verified the empty-FromType concern is strictly
equal-or-more-restrictive (cannot manufacture an ALLOW).

## Documentation (enhancements only)

- [x] ~~Docs section~~ (N/A: security bugfix, no user-facing API change)

## Final Checks

- [x] Commit message explains the why (authz-before-existence + the deliberate DEC-HWZHA reversal for the missing-peer case)
- [x] No TODOs left unaddressed
- [x] Ready for another developer to use

## Pull Request

- [x] PR created: https://github.com/sourcehaven-bv/rela/pull/1115
- [x] All code CI checks pass (Rela Tickets gate clears on this bug reaching done)
- [x] PR URL documented above

**PR:** https://github.com/sourcehaven-bv/rela/pull/1115
