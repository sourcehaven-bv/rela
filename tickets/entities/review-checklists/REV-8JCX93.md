---
id: REV-8JCX93
type: review-checklist
title: 'Review: Frontend unit suite makes real HTTP calls'
status: done
---

<!-- @managed: claude-workflow v1 -->

Reconstructed after the fact. BUG-762I34 shipped as `status=done` without a review
checklist. A malformed-frontmatter parse error made `rela validate` skip the
affected entities and report success, so `done-bug-needs-review-done` never
failed CI (see TKT-W76LRP). Items below record what PR #1340 actually did — no
new verification is claimed.

## Automated Checks
- [x] Frontend unit suite green and no longer emits an unhandled socket-hangup rejection
- [x] Full CI green on PR #1340 before merge

## Code Review
- [x] Reviewed as part of PR #1340 (merged 2026-08-16)
- [x] HTTP adapter stubbed in the shared `src/test/setup.ts`, alongside the existing globals

## Acceptance Verification
- [x] An unmocked call can no longer reach a socket
- [x] Property asserted directly, so the guarantee is pinned by a test rather than by absence of a symptom
- [x] Recorded as an automated measure (AM-frontend-tests-no-network)

## Documentation
- [x] ~~User-facing docs~~ (N/A: test-harness change)

## Final Checks
- [x] 5-whys analysis recorded (why1-why5) with `prevention`
- [x] Ready to merge

## Pull Request
- [x] PR #1340 merged to develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1340
