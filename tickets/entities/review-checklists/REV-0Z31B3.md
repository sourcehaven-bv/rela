---
id: REV-0Z31B3
type: review-checklist
title: 'Review: Weekly fuzz sweep'
status: done
---

<!-- @managed: claude-workflow v1 -->

## Automated Checks

- [x] Store packages + consumers green under `-race` after the ValidateRelationType extraction
- [x] `golangci-lint` 0 issues; gofmt clean
- [x] Full `just ci` green after review fixes

## Code Review

- [x] `/code-review` run (cranky-code-reviewer)
- [x] Findings: 0 critical, 4 significant (ALL addressed: failure classification + build gate; oracle vacuity anchored; relType oracle extracted to storeutil.ValidateRelationType across all 3 backends; known-red documented), 1 minor batch (addressed), 1 nit (wont-fix with reason) — RR-WW7L59, RR-P59UTD, RR-T4VKMV, RR-FGL95I, RR-YA3PRX, RR-MCWQ6S
- [x] Reviewer verified: discovery correctness (39 targets, helpers excluded), workflow injection surfaces clean, issue dedupe + label idempotence correct, cron timing, timeout adequacy, committed seeds pass both backends

## Verification

- [x] The sweep found 5 real failures in its first 2-second run (4 harness-oracle bugs fixed here; 1 production bug filed as BUG-RHFHTH with the weekly-fuzz-sweep measure entity linked)

**PR:** https://github.com/sourcehaven-bv/rela/pull/964
