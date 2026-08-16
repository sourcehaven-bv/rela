---
id: REV-F55NPH
type: review-checklist
title: 'Review: Vulnerability Check red on develop — CI go-version pin'
status: done
---

<!-- @managed: claude-workflow v1 -->

Reconstructed after the fact. BUG-219PVU shipped as `status=done` without a review
checklist. A malformed-frontmatter parse error made `rela validate` skip the
affected entities and report success, so `done-bug-needs-review-done` never
failed CI (see TKT-W76LRP). Items below record what PR #1338 actually did — no
new verification is claimed.

## Automated Checks
- [x] `govulncheck` clean after the Go 1.26.6 + x/image 0.45.0 bumps
- [x] Full CI green on PR #1338 before merge

## Code Review
- [x] Reviewed as part of PR #1338 (merged 2026-08-16)
- [x] Exact-patch pin recorded as an automated measure (AM-exact-go-patch-pin)

## Acceptance Verification
- [x] Vulnerability Check job green on develop after merge
- [x] Root cause recorded in the bug's 5-whys: the `go-version` pin resolved to an unpatched 1.26.5

## Documentation
- [x] ~~User-facing docs~~ (N/A: CI configuration change)

## Final Checks
- [x] 5-whys analysis recorded (why1-why5) with `prevention`
- [x] Ready to merge

## Pull Request
- [x] PR #1338 merged to develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1338
