---
id: REV-N7YOJR
type: review-checklist
title: 'Review: Cancel on a directly-opened create form walks out of the SPA'
status: done
---

<!-- @managed: claude-workflow v1 -->

Reconstructed after the fact. BUG-ZE4354 shipped as `status=done` without a review
checklist. A malformed-frontmatter parse error made `rela validate` skip the
affected entities and report success, so `done-bug-needs-review-done` never
failed CI (see TKT-W76LRP). Items below record what PR #1334 actually did — no
new verification is claimed.

## Automated Checks
- [x] Frontend suite green; Cancel behaviour pinned by test
- [x] Full CI green on PR #1334 before merge

## Code Review
- [x] Fix shipped in PR #1326; bug closed out in PR #1334
- [x] `handleCancel` no longer relies on `router.back()` when there is no SPA history to go back to

## Acceptance Verification
- [x] Cancel on a directly-opened create form (pasted URL, bookmark, new tab) stays inside the SPA
- [x] Recorded as an automated measure (AM-form-cancel-stays-in-spa)

## Documentation
- [x] ~~User-facing docs~~ (N/A: UI behaviour fix, no documented contract change)

## Final Checks
- [x] 5-whys analysis recorded with `prevention`
- [x] Ready to merge

## Pull Request
- [x] PR #1334 merged to develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1334
