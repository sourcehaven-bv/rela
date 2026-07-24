---
id: REV-GJI2YP
type: review-checklist
title: 'Review: Patch brace-expansion DoS advisory in frontend devDeps (GHSA-3jxr-9vmj-r5cp)'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks
- [x] `npm audit` → 0 vulnerabilities
- [x] Frontend tests: 1359 passed; lint + build unaffected
- [x] `govulncheck` (Go side) clean — unrelated to this npm bump

## Code Review
- [x] ~~cranky-code-reviewer~~ (N/A: no code change — a transitive lockfile-only dependency bump; nothing to review beyond the version diff)
- [x] Self-reviewed the diff: only `frontend/package-lock.json`, patched versions only

## Acceptance Verification
- [x] npm audit 0 vulns — PASS
- [x] tests/lint/build unaffected — PASS
- [x] Dependabot alerts will close on merge — expected

## Documentation
- [x] N/A (no user-facing change)

## Final Checks
- [x] Commit message explains the why (advisory, devDep scope)
- [x] Ready to merge

## Pull Request
- [x] PR #1192 opened against develop.

**PR:** https://github.com/sourcehaven-bv/rela/pull/1192
