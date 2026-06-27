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

**Review Responses:** 0 critical, 1 significant (RR-01SJ8N — membership refresh
was false because acl.Request.Globals is memoized; mocked test gave false
confidence → fixed by fresh-gate re-derive + real-resolver test
TestSSEACL_MembershipChangeReGates), 1 minor (RR-SB0IPG — coalesced re-walk), 1
nit (RR-ZS04NY — M1-M4 cleanups). Reviewer verified clean:
timer/lifecycle/goroutine (no leak, -race clean), no-id-on-wire boundary,
fail-closed, no zero-value frame, frontend clobber-protection. Earlier 11-RR
design exploration captured in the ticket; cacheId/mergebox/snapshot-ACL
rejected (snapshot-ACL → IDEA-CQMKMD).

## Acceptance Verification

- [x] Each acceptance criterion tested
- [x] Test evidence documented in implementation checklist

**Acceptance Status:** AC1–11 + RR-GVHEIK two-principals all PASS. AC6
membership refresh proven against a real acl.Declarative
(TestSSEACL_MembershipChangeReGates — flip without reconnect). Full table in
IMPL-2JSV1T.

## Documentation

- [x] Docs-checklist created and linked (DOCS-O6X5S3)
- [x] User-facing documentation updated
- [x] Docs-checklist marked done

## Final Checks

- [x] Commit messages explain the why
- [x] No TODOs/FIXMEs left
- [x] Ready for another developer to use

## Pull Request

- [x] Run `/pr`
- [x] CI: local `just ci` green; stacked-PR CI subset (full suite re-fires on retarget to develop after the stack lands, same flow as 972/949)
- [x] PR URL documented below

**PR:** https://github.com/sourcehaven-bv/rela/pull/981 (base:
feat/acl-search-tkt-ba8bsx — 4th PR of the read-side stack)
