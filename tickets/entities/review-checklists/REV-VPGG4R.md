---
id: REV-VPGG4R
type: review-checklist
title: 'Review: ACL read-side: SSE /api/v1/_events per-type gating — type-scoped staleness signal, ReadQuery-gated'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] All tests pass (`just test`) — full dataentry suite green; `-race -count=2` clean on SSE tests
- [x] Lint clean (`just lint`) + arch-lint clean
- [x] Coverage maintained — sole failure is the gitignored `e2e/node_modules/flatted` artifact (absent in CI); all real packages above floors

## Code Review

- [x] Run `/code-review` (cranky-code-reviewer, full diff vs origin/feat/acl-search-tkt-ba8bsx)
- [x] All critical review-responses addressed — 0 critical found
- [x] All significant review-responses addressed
- [x] Self-reviewed the diff for unrelated changes

**Review Responses:** 0 critical, 1 significant (RR-01SJ8N — RelationChange
didn't actually refresh membership because acl.Request.Globals is memoized; the
mocked test gave false confidence → FIXED by re-deriving a fresh gate on
RelationChange + new real-resolver test TestSSEACL_MembershipChangeReGates that
drives a live acl.Declarative and proves the membership flip without reconnect),
1 minor (RR-SB0IPG — undebounced RelationChange cost cliff → coalesced into the
flush window, one re-walk per connection per burst), 1 nit (RR-ZS04NY — M1-M4
cleanups: stale godoc, dead entityIds, zero-frame guard, stale comment).
Reviewer verified clean: timer/lifecycle/goroutine (no leak, defer unsubscribe +
flush.Stop, exits on cancel/close, -race clean), no-id-on-wire boundary holds,
fail-closed correct, no zero-value frame on the wire, frontend
clobber-protection intact. The deep design-review exploration (11 earlier RRs)
is captured in the ticket; the cacheId/mergebox/snapshot-ACL alternatives are
recorded as rejected (snapshot-ACL → IDEA-CQMKMD).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:**

| AC | Status | Evidence |
|----|--------|----------|
| 1 per-type gating | PASS | TestSSEACL_PerTypeGating (ticket frame, no feature) |
| 2 role-relation Query delivers | PASS | TestSSEACL_RoleRelationInheritance |
| 3 no id on wire | PASS | TestSSEACL_NoIDOnWire (TKT-/id tokens absent) |
| 4 DenyAll withholds | PASS | TestSSEACL_DenyAllWithholds |
| 5 debounce | PASS | TestSSEACL_Debounce (20→1) + MultiType |
| 6 cheap cached gate + membership refresh | PASS | TestSSEACL_VerdictCached (cache) + TestSSEACL_MembershipChangeReGates (real-resolver membership flip without reconnect) |
| 7 fail-closed on error | PASS | TestSSEACL_FailClosedOnZeroVerdict |
| 8 NopACL | PASS | TestSSEACL_NopACLAllTypesFlow |
| 9 audit-isolation | PASS | TestSSE_DoesNotFlowAuditEvents green |
| 10 client | PASS | useEvents.test.ts (11 tests, id-less entity:changed); 1032 frontend tests green |
| 11 docs | PASS | GUIDE section + regenerated docs/acl-security.md |
| — two-principals-per-connection (RR-GVHEIK) | PASS | TestSSEACL_TwoPrincipalsDifferentFrames |

## Documentation

- [x] Docs-checklist created and linked (DOCS-O6X5S3)
- [x] User-facing documentation updated
- [x] Docs-checklist marked done

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs left
- [x] Ready for another developer to use

## Pull Request

- [ ] Run `/pr`
- [ ] All CI checks pass
- [ ] PR URL documented below

**PR:** <!-- pending -->
