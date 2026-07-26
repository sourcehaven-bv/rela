---
id: REV-1TZYIH
type: review-checklist
title: 'Review: Fix stale identity assertion in policy-less script-read seam test'
status: done
---

## Automated Checks

- [x] `Test` job green on PR #1228 (the job this bug reddened)
- [x] `Lint`, `Frontend`, `Fuzz`, `Postgres Backend`, `Architecture`, `God-object lint`, CodeQL all pass
- [x] All cross-compile targets pass

## Code Review

- [x] Change is test-only; no production wiring touched (verified via `git diff --stat`)
- [x] Security contract preserved — the test still fails if the policy-less path returns a gated or deny reader
- [x] Assertion now pins observable behaviour (type + pass-through read) rather than pointer identity
- [x] ~~cranky-code-reviewer agent run~~ (N/A: 8-line single-assertion test fix with a documented root cause)

## Verification

- [x] Root cause confirmed by bisecting to #1208's `return a.store` → `visibility.Unrestricted(a.store)` change
- [x] Prevention recorded as AM-ungated-read-contract-not-identity

**PR:** https://github.com/sourcehaven-bv/rela/pull/1228

**Summary:** `TestScriptReadSeam_PolicylessProjectStaysUnrestricted` asserted
pointer identity with `app.store`, which #1208 invalidated by naming the ungated
path `visibility.Unrestricted`. Production behaviour was correct throughout —
only the assertion was stale, and it reddened the shared `Test` job on every
open PR including all 9 approved dependabot bumps. Fixed by asserting the
ungated contract instead of the identity.
