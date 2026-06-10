---
id: REV-8UFVYT
type: review-checklist
title: 'Review: Default package coverage floor'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] `go-test-coverage` PASS on regenerated profile; lint 0 issues on the new config tests
- [x] Full `just ci` green (pre-review commit; floors re-verified after review changes)

## Code Review

- [x] `/code-review` run (cranky-code-reviewer; independently re-measured every override number)
- [x] Findings: 0 critical, 1 significant (mcp proximity — addressed with explicit override), 1 minor (config 0-floor — addressed by writing the tests, 92.6%, floor 87), 1 nit (wont-fix with reason), plus confirmations of the graphquerynaive rationale and regex anchoring — RR-TFOEDA, RR-R375DS, RR-N18BKF
- [x] All critical/significant findings addressed

## Verification

- [x] Negative check: untested scratch package fails the floor; removed after verification

**PR:** https://github.com/sourcehaven-bv/rela/pull/966
